package spark

import (
	"net"
	"net/http"
	"reflect"
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

	// per-connection outbox buffer size
	outboxSize = 16
)

// Protocol describes the lifecycle and message dispatch hooks of a connection.
type Protocol struct {
	// Message is a template used to construct incoming messages.
	Message Message

	// Connect is called once after the connection has been established.
	Connect func(*Client) error

	// Handle is called for each message received from the client.
	Handle func(*Client, Message) error

	// Disconnect is called once before the connection is closed.
	Disconnect func(*Client) error
}

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

	// keep the tomb alive until close so connection goroutines can rely on
	// m.tomb.Dying() as a shutdown signal
	m.tomb.Go(func() error {
		<-m.tomb.Dying()
		return tomb.ErrDying
	})

	return m
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

	// ensure the connection gets closed
	defer ws.Close()

	// set read limit
	ws.SetReadLimit(maxMessageSize)

	// prepare client
	conn := &Client{
		ws:     ws,
		ctx:    ctx,
		outbox: make(chan Message, outboxSize),
		done:   make(chan struct{}),
	}

	// signal client done on exit
	defer close(conn.done)

	// call connect if available
	if m.protocol.Connect != nil {
		err = m.protocol.Connect(conn)
		if err != nil {
			return xo.WF(err, "connect failed")
		}
	}

	// ensure disconnect is called
	if m.protocol.Disconnect != nil {
		defer func() {
			_ = m.protocol.Disconnect(conn)
		}()
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
	msgs := make(chan Message, 16)

	// signal reader to exit when writer returns
	writerDone := make(chan struct{})
	defer close(writerDone)

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
				writeWebsocketError(ws, "not a text message")
				close(errs)
				return
			}

			// clone the protocol message template and decode into it
			msg := cloneMessage(m.protocol.Message)
			err = GetMeta(msg).Coding.Unmarshal(bytes, msg)
			if err != nil {
				errs <- xo.W(err)
				return
			}

			// validate message
			err = msg.Validate()
			if err != nil {
				errs <- xo.W(err)
				return
			}

			// reset pinger
			pinger.Reset(pingTimeout)

			// forward message
			select {
			case msgs <- msg:
			case <-m.tomb.Dying():
				close(errs)
				return
			case <-writerDone:
				close(errs)
				return
			}
		}
	}()

	// run writer
	for {
		select {
		// dispatch incoming messages
		case msg := <-msgs:
			if m.protocol.Handle != nil {
				err = m.protocol.Handle(conn, msg)
				if err != nil {
					return xo.WF(err, "handle failed")
				}
			}
		// drain outbox
		case msg := <-conn.outbox:
			err = conn.write(nil, msg)
			if err != nil {
				return err
			}
		// handle pings
		case <-pinger.C:
			err = ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				return err
			}
			err = ws.WriteMessage(websocket.PingMessage, nil)
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

func (m *manager) close() {
	m.tomb.Kill(nil)
	_ = m.tomb.Wait()
}

func writeWebsocketError(ws *websocket.Conn, msg string) {
	_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseUnsupportedData, msg), time.Time{})
}

func cloneMessage(template Message) Message {
	typ := reflect.TypeOf(template).Elem()
	return reflect.New(typ).Interface().(Message)
}
