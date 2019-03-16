package spark

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/256dpi/fire"

	"github.com/gorilla/websocket"
)

const (
	// max message size
	maxMessageSize = 4048

	// the time after which a write times out
	writeTimeout = 10 * time.Second

	// the timeout after which a ping is sent to keep the connection alive
	pingTimeout = 45 * time.Second

	// the timeout after a connection is closed when there is no traffic
	receiveTimeout = 90 * time.Second
)

type queue chan *Event

type request struct {
	Subscribe   map[string]Map `json:"subscribe"`
	Unsubscribe []string       `json:"unsubscribe"`
}

type response map[string]map[string]string

type manager struct {
	watcher *Watcher

	upgrader     *websocket.Upgrader
	subscribes   chan queue
	events       queue
	unsubscribes chan queue
}

func newManager(w *Watcher) *manager {
	// create manager
	h := &manager{
		watcher:      w,
		upgrader:     &websocket.Upgrader{},
		subscribes:   make(chan queue, 10),
		events:       make(queue, 10),
		unsubscribes: make(chan queue, 10),
	}

	// do not check request origin
	h.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// run background process
	go h.run()

	return h
}

func (m *manager) run() {
	// prepare queues
	queues := map[queue]bool{}

	for {
		select {
		// handle subscribes
		case q := <-m.subscribes:
			// store queue
			queues[q] = true
		// handle events
		case e := <-m.events:
			// add message to all queues
			for q := range queues {
				select {
				case q <- e:
				default:
					// close and delete queue
					close(q)
					delete(queues, q)
				}
			}
		// handle unsubscribes
		case q := <-m.unsubscribes:
			// delete queue
			delete(queues, q)
		}
	}
}

func (m *manager) broadcast(evt *Event) {
	// queue event
	m.events <- evt

	// TODO: Log error if enqueuing takes too long?
}

func (m *manager) handle(ctx *fire.Context) {
	// try to upgrade connection
	conn, err := m.upgrader.Upgrade(ctx.ResponseWriter, ctx.HTTPRequest, nil)
	if err != nil {
		// upgrader already responded with an error

		// call reporter if available
		if m.watcher.Reporter != nil {
			m.watcher.Reporter(err)
		}

		return
	}

	// ensure the connections gets closed
	defer conn.Close()

	// prepare queue
	q := make(queue, 10)

	// register queue
	m.subscribes <- q

	// process (reuse current goroutine)
	err = m.process(ctx, conn, q)
	if err != nil {
		// call reporter if available
		if m.watcher.Reporter != nil {
			m.watcher.Reporter(err)
		}
	}

	// unsubscribe queue
	m.unsubscribes <- q
}

func (m *manager) process(ctx *fire.Context, conn *websocket.Conn, queue queue) error {
	// set read limit (we only expect pong messages)
	conn.SetReadLimit(maxMessageSize)

	// reset read readline if a pong has been received
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(receiveTimeout))
	})

	// prepare read error channel
	readErr := make(chan error, 1)

	// prepare incoming channel
	inc := make(chan request, 10)

	// run reader
	go func() {
		for {
			// reset read timeout
			err := conn.SetReadDeadline(time.Now().Add(receiveTimeout))
			if err != nil {
				readErr <- err
				return
			}

			// read on the connection for ever
			typ, bytes, err := conn.ReadMessage()
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				readErr <- nil
				return
			} else if err != nil {
				readErr <- err
				return
			}

			// check message type
			if typ != websocket.TextMessage {
				readErr <- errors.New("not a text message")
				return
			}

			// decode message
			var req request
			err = json.Unmarshal(bytes, &req)
			if err != nil {
				readErr <- err
				return
			}

			// TODO: Add timeout.

			// forward request
			inc <- req
		}
	}()

	// prepare registry
	reg := map[string]*Subscription{}

	// run writer
	for {
		select {
		// wait for a request
		case req := <-inc:
			// handle subscriptions
			for name, data := range req.Subscribe {
				// get stream
				stream, ok := m.watcher.streams[name]
				if !ok {
					return errors.New("invalid subscription")
				}

				// prepare subscription
				sub := &Subscription{
					Context: ctx,
					Data:    data,
					Stream:  stream,
				}

				// validate subscription if available
				if stream.Validator != nil {
					err := stream.Validator(sub)
					if err != nil {
						return err
					}
				}

				// add subscription
				reg[name] = sub
			}

			// handle unsubscriptions
			for _, name := range req.Unsubscribe {
				delete(reg, name)
			}
		// wait for message to send
		case evt, ok := <-queue:
			// check if closed
			if !ok {
				return errors.New("closed")
			}

			// get subscription
			sub, ok := reg[evt.Stream.name]
			if !ok {
				continue
			}

			// run selector if present
			if evt.Stream.Selector != nil {
				if !evt.Stream.Selector(evt, sub) {
					continue
				}
			}

			// create response
			res := response{
				evt.Stream.name: {
					evt.ID.Hex(): string(evt.Type),
				},
			}

			// set write deadline
			err := conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				return err
			}

			// write message
			err = conn.WriteJSON(res)
			if err != nil {
				return err
			}
		// wait for ping timeout
		case <-time.After(pingTimeout):
			// set write deadline
			err := conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				return err
			}

			// write ping message
			err = conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return err
			}
		// exit if on read err
		case err := <-readErr:
			return err
		}
	}
}
