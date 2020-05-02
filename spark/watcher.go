package spark

import (
	"fmt"
	"sync"

	"github.com/256dpi/xo"
	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// StateFactory constructs per-connection state.
type StateFactory func(ctx *fire.Context) (State, error)

// CommandType identifies the type of a Command.
type CommandType string

const (
	// Subscribe registers a stream subscription on the connection.
	Subscribe CommandType = "subscribe"

	// Unsubscribe removes a stream subscription from the connection.
	Unsubscribe CommandType = "unsubscribe"

	// Created indicates a model was created.
	Created CommandType = "created"

	// Updated indicates a model was updated.
	Updated CommandType = "updated"

	// Deleted indicates a model was deleted.
	Deleted CommandType = "deleted"
)

// Command is the wire message exchanged between client and server.
type Command struct {
	Base `json:"-" spark:"command"`

	// Type identifies the command.
	Type CommandType `json:"type"`

	// Name is the stream name (subscribe, unsubscribe, and event commands).
	Name string `json:"name,omitempty"`

	// Params holds optional subscription parameters.
	Params stick.Map `json:"params,omitempty"`

	// ID is the resource id for event commands.
	ID string `json:"id,omitempty"`
}

// Validate implements the Message interface.
func (c *Command) Validate() error {
	return nil
}

// connState holds the per-connection registry of subscriptions and the
// user-provided state value.
type connState struct {
	mu       sync.RWMutex
	registry map[string]*Subscription
	user     State
}

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	factory  StateFactory
	reporter func(error)
	manager  *manager
	streams  map[string]*Stream
	events   chan *Event
	conns    sync.Map
	tomb     tomb.Tomb
}

// NewWatcher creates and returns a new watcher.
func NewWatcher(factory StateFactory, reporter func(error)) *Watcher {
	// prepare watcher
	w := &Watcher{
		factory:  factory,
		reporter: reporter,
		streams:  make(map[string]*Stream),
		events:   make(chan *Event, 100),
	}

	// create manager
	w.manager = newManager(&Protocol{
		Message:    &Command{},
		Connect:    w.connect,
		Handle:     w.handle,
		Disconnect: w.disconnect,
	})

	// run dispatcher
	w.tomb.Go(w.dispatch)

	return w
}

// Add will add a stream to the watcher.
func (w *Watcher) Add(stream *Stream) {
	// check existence
	if w.streams[stream.Name()] != nil {
		panic(fmt.Sprintf(`spark: stream with name "%s" already exists`, stream.Name()))
	}

	// save stream
	w.streams[stream.Name()] = stream

	// open stream
	stream.open(w, w.reporter)
}

// Action returns an action that should be registered in the group under
// the "watch" name.
func (w *Watcher) Action() *fire.Action {
	return fire.A("spark/Watcher.Action", []string{"GET"}, 0, 0, func(ctx *fire.Context) error {
		// handle connection
		err := w.manager.handle(ctx)
		if err != nil {
			if w.reporter != nil {
				w.reporter(err)
			}
		}

		return nil
	})
}

// Close will close the watcher and all opened streams.
func (w *Watcher) Close() {
	// close all streams to stop event production
	for _, stream := range w.streams {
		stream.close()
	}

	// stop dispatcher
	w.tomb.Kill(nil)
	_ = w.tomb.Wait()

	// close manager
	w.manager.close()
}

func (w *Watcher) connect(conn *Client) error {
	// construct user state
	var user State
	if w.factory != nil {
		var err error
		user, err = w.factory(conn.Context())
		if err != nil {
			return xo.WF(err, "state factory failed")
		}
	}

	// store state
	w.conns.Store(conn, &connState{
		registry: map[string]*Subscription{},
		user:     user,
	})

	return nil
}

func (w *Watcher) handle(conn *Client, msg Message) error {
	// get command
	cmd := msg.(*Command)

	// get state
	state := mustLoad(&w.conns, conn).(*connState)

	// get stream
	stream, ok := w.streams[cmd.Name]
	if !ok {
		return fmt.Errorf("invalid subscription")
	}

	// handle unsubscribe
	if cmd.Type == Unsubscribe {
		state.mu.Lock()
		delete(state.registry, cmd.Name)
		state.mu.Unlock()
		return nil
	}

	// check type
	if cmd.Type != Subscribe {
		return fmt.Errorf("invalid command type: %s", cmd.Type)
	}

	// prepare subscription
	sub := &Subscription{
		Context: conn.Context(),
		Data:    cmd.Params,
		Stream:  stream,
		State:   state.user,
	}

	// validate subscription if available
	if stream.Validator != nil {
		err := stream.Validator(sub)
		if err != nil {
			return fmt.Errorf("invalid subscription")
		}
	}

	// add subscription
	state.mu.Lock()
	state.registry[cmd.Name] = sub
	state.mu.Unlock()

	return nil
}

func (w *Watcher) disconnect(conn *Client) error {
	// remove state
	w.conns.Delete(conn)

	return nil
}

func (w *Watcher) broadcast(evt *Event) {
	select {
	case w.events <- evt:
	case <-w.tomb.Dying():
	}
}

func (w *Watcher) dispatch() error {
	for {
		select {
		case evt := <-w.events:
			w.deliver(evt)
		case <-w.tomb.Dying():
			return tomb.ErrDying
		}
	}
}

func (w *Watcher) deliver(evt *Event) {
	w.conns.Range(func(key, value interface{}) bool {
		conn := key.(*Client)
		state := value.(*connState)

		// update user state before dispatch so selectors observe current state
		if state.user != nil {
			err := state.user.Update(evt)
			if err != nil {
				if w.reporter != nil {
					w.reporter(xo.WF(err, "state update failed"))
				}
				_ = conn.Close()
				return true
			}
		}

		// get subscription
		state.mu.RLock()
		sub, ok := state.registry[evt.Stream.Name()]
		state.mu.RUnlock()
		if !ok {
			return true
		}

		// run selector if present
		if evt.Stream.Selector != nil {
			if !evt.Stream.Selector(evt, sub) {
				return true
			}
		}

		// map event type
		var typ CommandType
		switch evt.Type {
		case coal.Created:
			typ = Created
		case coal.Updated:
			typ = Updated
		case coal.Deleted:
			typ = Deleted
		default:
			return true
		}

		// enqueue command
		err := conn.Enqueue(&Command{
			Type: typ,
			Name: evt.Stream.Name(),
			ID:   evt.ID.Hex(),
		})
		if err != nil {
			if w.reporter != nil {
				w.reporter(xo.WF(err, "enqueue failed"))
			}
			_ = conn.Close()
		}

		return true
	})
}

func mustLoad(m *sync.Map, key interface{}) interface{} {
	v, _ := m.Load(key)
	return v
}
