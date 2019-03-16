package spark

import (
	"encoding/base64"
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
}

func (m *manager) handle(ctx *fire.Context) {
	// check if websocket upgrade
	if websocket.IsWebSocketUpgrade(ctx.HTTPRequest) {
		m.handleWebsocket(ctx)
	} else {
		m.handleSSE(ctx)
	}
}

func (m *manager) handleWebsocket(ctx *fire.Context) {
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
	err = m.websocketLoop(ctx, conn, q)
	if err != nil {
		// call reporter if available
		if m.watcher.Reporter != nil {
			m.watcher.Reporter(err)
		}
	}

	// unsubscribe queue
	m.unsubscribes <- q
}

func (m *manager) websocketLoop(ctx *fire.Context, conn *websocket.Conn, queue queue) error {
	// set read limit (we only expect pong messages)
	conn.SetReadLimit(maxMessageSize)

	// reset read readline if a pong has been received
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(receiveTimeout))
	})

	// prepare read error channel
	readErr := make(chan error, 1)

	// prepare requests channel
	reqs := make(chan request, 10)

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

			// decode request
			var req request
			err = json.Unmarshal(bytes, &req)
			if err != nil {
				readErr <- err
				return
			}

			// forward request
			reqs <- req
		}
	}()

	// prepare registry
	reg := map[string]*Subscription{}

	// run writer
	for {
		select {
		// handle request
		case req := <-reqs:
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
		// handle events
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
		// handle pings
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
		// handle errors
		case err := <-readErr:
			return err
		}
	}
}

func (m *manager) handleSSE(ctx *fire.Context) {
	// check flusher support
	flusher, ok := ctx.ResponseWriter.(http.Flusher)
	if !ok {
		http.Error(ctx.ResponseWriter, "flushing not supported", http.StatusNotImplemented)
		return
	}

	// check close notifier support
	closeNotifier, ok := ctx.ResponseWriter.(http.CloseNotifier)
	if !ok {
		http.Error(ctx.ResponseWriter, "closing not supported", http.StatusNotImplemented)
		return
	}

	// get subscription
	name := ctx.HTTPRequest.URL.Query().Get("s")
	if name == "" {
		http.Error(ctx.ResponseWriter, "missing stream name", http.StatusBadRequest)
		return
	}

	// prepare data
	data := Map{}

	// get data
	encodedData := ctx.HTTPRequest.URL.Query().Get("d")
	if encodedData != "" {
		// decode data
		bytes, err := base64.StdEncoding.DecodeString(encodedData)
		if err != nil {
			http.Error(ctx.ResponseWriter, "invalid data encoding", http.StatusBadRequest)
			return
		}

		// unmarshal data
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			http.Error(ctx.ResponseWriter, "invalid data encoding", http.StatusBadRequest)
			return
		}
	}

	// get stream
	stream, ok := m.watcher.streams[name]
	if !ok {
		http.Error(ctx.ResponseWriter, "stream not found", http.StatusBadRequest)
		return
	}

	// create subscription
	sub := &Subscription{
		Context: ctx,
		Data:    data,
		Stream:  stream,
	}

	// validate subscription if present
	if stream.Validator != nil {
		err := stream.Validator(sub)
		if err != nil {
			http.Error(ctx.ResponseWriter, "invalid subscription", http.StatusBadRequest)
			return
		}
	}

	// set headers for SSE
	h := ctx.ResponseWriter.Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("Content-Type", "text/event-stream")

	// write ok
	ctx.ResponseWriter.WriteHeader(http.StatusOK)

	// flush header
	flusher.Flush()

	// prepare queue
	q := make(queue, 10)

	// register queue
	m.subscribes <- q

	// process (reuse current goroutine)
	err := m.sseLoop(ctx, flusher, closeNotifier.CloseNotify(), sub, q)
	if err != nil {
		// call reporter if available
		if m.watcher.Reporter != nil {
			m.watcher.Reporter(err)
		}
	}

	// unsubscribe queue
	m.unsubscribes <- q
}

func (m *manager) sseLoop(ctx *fire.Context, flusher http.Flusher, close <-chan bool, sub *Subscription, queue queue) error {
	// get response writer
	w := ctx.ResponseWriter

	// create encoder
	enc := json.NewEncoder(w)

	// run writer
	for {
		select {
		// handle events
		case evt, ok := <-queue:
			// check if closed
			if !ok {
				return errors.New("closed")
			}

			// check stream
			if evt.Stream != sub.Stream {
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

			// write prefix
			_, err := w.Write([]byte("data: "))
			if err != nil {
				return err
			}

			// write json
			err = enc.Encode(res)
			if err != nil {
				return err
			}

			// write suffix
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return err
			}

			// flush writer
			flusher.Flush()
		// handle close
		case <-close:
			return nil
		}
	}
}
