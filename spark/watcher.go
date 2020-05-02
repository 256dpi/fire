package spark

import (
	"fmt"
	"sync"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type state struct {
	registry map[string]*Subscription
}

type CommandType string

const (
	// client
	Subscribe   CommandType = "subscribe"
	Unsubscribe CommandType = "unsubscribe"

	// server
	Created CommandType = "created"
	Updated CommandType = "updated"
	Deleted CommandType = "deleted"
)

type Command struct {
	Base `json:"-" spark:"command"`

	// the command type
	Type CommandType `json:"type"`

	// the subscription name and params
	Name   string    `json:"name,omitempty"`
	Params stick.Map `json:"params,omitempty"`

	// the created, updated or deleted id
	ID string `json:"id,omitempty"`
}

func (c *Command) Validate() error {
	return nil
}

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	reporter func(error)
	manager  *manager
	streams  map[string]*Stream
	events   chan *Event
	conns    sync.Map
}

// NewWatcher creates and returns a new watcher.
func NewWatcher(reporter func(error)) *Watcher {
	// prepare watcher
	w := &Watcher{
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

func (w *Watcher) connect(conn *Client) error {
	// store state
	w.conns.Store(conn, &state{
		registry: map[string]*Subscription{},
	})

	return nil
}

func (w *Watcher) handle(conn *Client, msg Message) error {
	// get command
	cmd := msg.(*Command)

	// get state
	state := mustLoad(&w.conns, conn).(*state)

	// get stream
	stream, ok := w.streams[cmd.Name]
	if !ok {
		return fmt.Errorf("invalid subscription")
	}

	// handle unsubscribe
	if cmd.Type == Unsubscribe {
		delete(state.registry, cmd.Name)
		return nil
	}

	// check type
	if cmd.Type != Subscribe {
		return fmt.Errorf("invalid command type: %s", cmd.Type)
	}

	// prepare subscription
	sub := &Subscription{
		Context: nil, // TODO: Set.
		Data:    cmd.Params,
		Stream:  stream,
	}

	// validate subscription if available
	if stream.Validator != nil {
		err := stream.Validator(sub)
		if err != nil {
			return fmt.Errorf("invalid subscription")
		}
	}

	// add subscription
	state.registry[cmd.Name] = sub

	return nil
}

func (w *Watcher) broadcast(evt *Event) {
	// deliver event
	w.conns.Range(func(key, value interface{}) bool {
		// get conn and state
		conn := key.(*Client)
		state := value.(*state)

		// get subscription
		sub, ok := state.registry[evt.Stream.Name()]
		if !ok {
			return true
		}

		// run selector if present
		if evt.Stream.Selector != nil {
			if !evt.Stream.Selector(evt, sub) {
				return true
			}
		}

		// get type
		var typ CommandType
		switch evt.Type {
		case coal.Created:
			typ = Created
		case coal.Updated:
			typ = Updated
		case coal.Deleted:
			typ = Deleted
		}

		// send command
		err := conn.Send(nil, &Command{
			Type: typ,
			Name: evt.Stream.Name(),
			ID:   evt.ID.Hex(),
		})
		if err != nil {
			// TODO: What to do?
			panic(err)
		}

		return true
	})
}

func (w *Watcher) disconnect(conn *Client) error {
	// remove state
	w.conns.Delete(conn)

	return nil
}

// Close will close the watcher and all opened streams.
func (w *Watcher) Close() {
	// close all stream
	for _, stream := range w.streams {
		stream.close()
	}

	// close manager
	w.manager.close()
}

func mustLoad(m *sync.Map, key interface{}) interface{} {
	v, _ := m.Load(key)
	return v
}
