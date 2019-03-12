package spark

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/globalsign/mgo/bson"
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

type queue chan event

type request struct {
	Subscribe   map[string]string `json:"subscribe"`
	Unsubscribe []string          `json:"unsubscribe"`
}

type subscription struct {
	name    string
	id      string
	filters map[string]interface{}
}

type event struct {
	name string
	op   string
	id   string
	doc  bson.M
}

type response map[string]map[string]string

type hub struct {
	policy   *Policy
	reporter func(error)

	upgrader    *websocket.Upgrader
	subscribe   chan queue
	messages    queue
	unsubscribe chan queue
}

func newHub(policy *Policy, reporter func(error)) *hub {
	// create hub
	h := &hub{
		policy:      policy,
		reporter:    reporter,
		upgrader:    &websocket.Upgrader{},
		subscribe:   make(chan queue),
		messages:    make(queue, 10),
		unsubscribe: make(chan queue),
	}

	// do not check request origin
	h.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// run background process
	go h.run()

	return h
}

func (h *hub) run() {
	// prepare queues
	queues := map[queue]bool{}

	for {
		select {
		// handle queue subscription
		case q := <-h.subscribe:
			// store queue
			queues[q] = true
		// handle message
		case message := <-h.messages:
			// add message to all queues
			for q := range queues {
				select {
				case q <- message:
				default:
					// skip if channel is full or closed
				}
			}
		// handle queue unsubscription
		case q := <-h.unsubscribe:
			// delete queue
			delete(queues, q)
		}
	}
}

func (h *hub) broadcast(evt event) {
	// send message
	select {
	case h.messages <- evt:
	default:
		// skip if full
	}
}

func (h *hub) handle(w http.ResponseWriter, r *http.Request) {
	// try to upgrade connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader already responded with an error

		// call reporter
		h.reporter(err)

		return
	}

	// ensure the connections gets closed
	defer conn.Close()

	// prepare queue
	q := make(queue, 10)

	// register queue
	h.subscribe <- q

	// process (reuse current goroutine)
	err = h.process(conn, q)
	if err != nil {
		// call reporter
		h.reporter(err)
	}

	// unsubscribe queue
	h.unsubscribe <- q
}

func (h *hub) process(conn *websocket.Conn, queue queue) error {
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
	reg := map[string]subscription{}

	// run writer
	for {
		select {
		// wait for a request
		case req := <-inc:
			// handle subscriptions
			for key, token := range req.Subscribe {
				// parse token
				claims, expired, err := h.policy.ParseToken(token)
				if err != nil {
					return err
				}

				// continue if expired
				if expired {
					continue
				}

				// add subscription
				reg[key] = subscription{
					name:    claims.Subject,
					id:      claims.Id,
					filters: claims.Data,
				}
			}

			// handle unsubscriptions
			for _, key := range req.Unsubscribe {
				delete(reg, key)
			}
		// wait for message to send
		case evt := <-queue:
			// continue if event is not matched
			if !match(evt, reg) {
				continue
			}

			// create response
			res := response{
				evt.name: {
					evt.id: evt.op,
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

func match(evt event, reg map[string]subscription) bool {
	// check all subscriptions
	for _, sub := range reg {
		// continue if name does not match sub
		if evt.name != sub.name {
			continue
		}

		// check if resource sub
		if sub.id != "" {
			// return immediately if sub matches id
			if evt.id == sub.id {
				return true
			}

			// otherwise continue
			continue
		}

		// is collection sub

		// ignore delete operations
		if evt.op == "delete" {
			continue
		}

		// TODO: Improve equality check.

		// check all filters
		notEqual := false
		for key, value := range sub.filters {
			if !reflect.DeepEqual(evt.doc[key], value) {
				notEqual = true
			}
		}

		// return true if fields are equal
		if !notEqual {
			return true
		}
	}

	return false
}
