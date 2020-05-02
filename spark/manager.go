package spark

import (
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
	maxMessageSize = 4096 // 4 KB

	// the time after write times out
	writeTimeout = 10 * time.Second

	// the interval at which a ping is sent to keep the connection alive
	pingTimeout = 45 * time.Second

	// the time after a connection is closed when there is no ping response
	receiveTimeout = 90 * time.Second
)

type Protocol struct {
	Message        Message
	Connect        func(*Client) error
	Handle         func(*Client, Message) error
	Disconnect     func(*Client) error
	MaxMessageSize int
	WriteTimeout   time.Duration
	PingTimeout    time.Duration
	ReceiveTimeout time.Duration
}

type request struct {
	Subscribe   map[string]Map `json:"subscribe"`
	Unsubscribe []string       `json:"unsubscribe"`
}

type response map[string]map[string]string

type manager struct {
	protocol *Protocol
	upgrader *websocket.Upgrader
	tomb     tomb.Tomb
}

func newManager(p *Protocol) *manager {
	// create manager
	m := &manager{
		protocol: p,
		upgrader: &websocket.Upgrader{},
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
		// message subscribes
		case q := <-m.subscribes:
			// store queue
			queues[q] = true
		// message events
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
		// message unsubscribes
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

func (m *manager) handle(ctx *fire.Context) error {
	// check if alive
	if !m.tomb.Alive() {
		return tomb.ErrDying
	}

	// try to upgrade connection
	ws, err := m.upgrader.Upgrade(ctx.ResponseWriter, ctx.HTTPRequest, nil)
	if err != nil {
		// error has already been written to client
		return nil
	}

	// ensure the connections gets closed
	defer ws.Close()

	// set read limit
	ws.SetReadLimit(maxMessageSize)

	// prepare conn
	conn := &Client{ws: ws}

	// call connect if available
	if m.protocol.Connect != nil {
		err = m.protocol.Connect(conn)
		if err != nil {
			return err
		}
	}

	// prepare pinger ticker
	pinger := time.NewTimer(pingTimeout)

	// reset read deadline if a pong has been received
	ws.SetPongHandler(func(string) error {
		pinger.Reset(pingTimeout)
		return ws.SetReadDeadline(time.Now().Add(receiveTimeout))
	})

	// prepare channels
	errs := make(chan error, 1)
	reqs := make(chan request, 10)

	// run reader
	go func() {
		for {
			// reset read timeout
			err := ws.SetReadDeadline(time.Now().Add(receiveTimeout))
			if err != nil {
				errs <- xo.W(err)
				return
			}

			// read next message from connection
			typ, bytes, err := ws.ReadMessage()
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
				writeWebsocketError(conn, "not a text message")
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

	// run writer
	for {
		select {
		// handle pings
		case <-pinger.C:
			// set write deadline
			err := ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				return err
			}

			// write ping message
			err = ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return err
			}
		// message errors
		case err := <-errs:
			return err
		// message close
		case <-m.tomb.Dying():
			return nil
		}
	}
}

func (m *manager) close() {
	m.tomb.Kill(nil)
	_ = m.tomb.Wait()
}

func writeWebsocketError(conn *websocket.Conn, msg string) {
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseUnsupportedData, msg), time.Time{})
}
