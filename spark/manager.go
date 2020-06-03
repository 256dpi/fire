package spark

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/256dpi/xo"
	"github.com/gorilla/websocket"
	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire"
)

const (
	// max message size
	maxMessageSize = 4096 // 4KB

	// the time after which a write times out
	writeTimeout = 10 * time.Second

	// the interval at which a ping is sent to keep the connection alive
	pingTimeout = 45 * time.Second

	// the time after a connection is closed when there is no ping response
	receiveTimeout = 90 * time.Second
)

type request struct {
	Subscribe   map[string]Map `json:"subscribe"`
	Unsubscribe []string       `json:"unsubscribe"`
}

type response map[string]map[string]string

type manager struct {
	watcher *Watcher

	upgrader     *websocket.Upgrader
	events       chan *Event
	subscribes   chan chan *Event
	unsubscribes chan chan *Event

	tomb tomb.Tomb
}

func newManager(w *Watcher) *manager {
	// create manager
	m := &manager{
		watcher:      w,
		upgrader:     &websocket.Upgrader{},
		events:       make(chan *Event, 10),
		subscribes:   make(chan chan *Event, 10),
		unsubscribes: make(chan chan *Event, 10),
	}

	// do not check request origin
	m.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// run background process
	m.tomb.Go(m.run)

	return m
}

func (m *manager) run() error {
	// prepare queues
	queues := map[chan *Event]bool{}

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
		case <-m.tomb.Dying():
			// close all queues
			for queue := range queues {
				close(queue)
			}

			// closed all subscribes
			close(m.subscribes)
			for sub := range m.subscribes {
				close(sub)
			}

			return tomb.ErrDying
		}
	}
}

func (m *manager) broadcast(evt *Event) {
	// queue event
	select {
	case m.events <- evt:
	case <-m.tomb.Dying():
	}
}

func (m *manager) handle(ctx *fire.Context) error {
	// check if alive
	if !m.tomb.Alive() {
		return tomb.ErrDying
	}

	// check if websocket upgrade
	if websocket.IsWebSocketUpgrade(ctx.HTTPRequest) {
		return m.handleWebsocket(ctx)
	}

	return m.handleSSE(ctx)
}

func (m *manager) close() {
	m.tomb.Kill(nil)
	_ = m.tomb.Wait()
}

func (m *manager) handleWebsocket(ctx *fire.Context) error {
	// try to upgrade connection
	conn, err := m.upgrader.Upgrade(ctx.ResponseWriter, ctx.HTTPRequest, nil)
	if err != nil {
		// error has already been written to client
		return nil
	}

	// ensure the connections gets closed
	defer conn.Close()

	// prepare queue
	queue := make(chan *Event, 10)

	// register queue
	select {
	case m.subscribes <- queue:
	case <-m.tomb.Dying():
		return tomb.ErrDying
	}

	// ensure unsubscribe
	defer func() {
		select {
		case m.unsubscribes <- queue:
		case <-m.tomb.Dying():
		}
	}()

	// set read limit (we only expect pong messages)
	conn.SetReadLimit(maxMessageSize)

	// prepare pinger ticker
	pinger := time.NewTimer(pingTimeout)

	// reset read deadline if a pong has been received
	conn.SetPongHandler(func(string) error {
		pinger.Reset(pingTimeout)
		return conn.SetReadDeadline(time.Now().Add(receiveTimeout))
	})

	// prepare channels
	errs := make(chan error, 1)
	reqs := make(chan request, 10)

	// run reader
	go func() {
		for {
			// reset read timeout
			err := conn.SetReadDeadline(time.Now().Add(receiveTimeout))
			if err != nil {
				errs <- xo.W(err)
				return
			}

			// read next message from connection
			typ, bytes, err := conn.ReadMessage()
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				close(errs)
				return
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				close(errs)
				return
			} else if err != nil {
				errs <- xo.W(err)
				return
			}

			// check message type
			if typ != websocket.TextMessage {
				m.websocketWriteError(conn, "not a text message")
				close(errs)
				return
			}

			// decode request
			var req request
			err = json.Unmarshal(bytes, &req)
			if err != nil {
				errs <- xo.W(err)
				return
			}

			// reset pinger
			pinger.Reset(pingTimeout)

			// forward request
			select {
			case reqs <- req:
			case <-m.tomb.Dying():
				close(errs)
				return
			}
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
					m.websocketWriteError(conn, "invalid subscription")
					return nil
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
						m.websocketWriteError(conn, "invalid subscription")
						return nil
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
				return nil
			}

			// get subscription
			sub, ok := reg[evt.Stream.Name()]
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
				evt.Stream.Name(): {
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
		case <-pinger.C:
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
		case err := <-errs:
			return err
		// handle close
		case <-m.tomb.Dying():
			return nil
		}
	}
}

func (m *manager) websocketWriteError(conn *websocket.Conn, msg string) {
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseUnsupportedData, msg), time.Time{})
}

func (m *manager) handleSSE(ctx *fire.Context) error {
	// check flusher support
	flusher, ok := ctx.ResponseWriter.(http.Flusher)
	if !ok {
		http.Error(ctx.ResponseWriter, "SSE not supported", http.StatusNotImplemented)
		return nil
	}

	// get subscription
	name := ctx.HTTPRequest.URL.Query().Get("s")
	if name == "" {
		http.Error(ctx.ResponseWriter, "missing stream name", http.StatusBadRequest)
		return nil
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
			return nil
		}

		// unmarshal data
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			http.Error(ctx.ResponseWriter, "invalid data encoding", http.StatusBadRequest)
			return nil
		}
	}

	// get stream
	stream, ok := m.watcher.streams[name]
	if !ok {
		http.Error(ctx.ResponseWriter, "stream not found", http.StatusBadRequest)
		return nil
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
			return nil
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
	queue := make(chan *Event, 10)

	// register queue
	select {
	case m.subscribes <- queue:
	case <-m.tomb.Dying():
		return tomb.ErrDying
	}

	// ensure unsubscribe
	defer func() {
		select {
		case m.unsubscribes <- queue:
		case <-m.tomb.Dying():
		}
	}()

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
				return nil
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
				evt.Stream.Name(): {
					evt.ID.Hex(): string(evt.Type),
				},
			}

			// write prefix
			_, err := w.Write([]byte("data: "))
			if err != nil {
				return xo.W(err)
			}

			// write json
			err = enc.Encode(res)
			if err != nil {
				return xo.W(err)
			}

			// write suffix
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return xo.W(err)
			}

			// flush writer
			flusher.Flush()
		// handle close
		case <-ctx.Done():
			return nil
		}
	}
}
