package spark

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/256dpi/fire"
)

// Client represents a single spark connection. It is used by the server-side
// protocol callbacks as well as by clients connecting to a spark server.
type Client struct {
	ws *websocket.Conn

	// server-side only fields
	ctx    *fire.Context
	outbox chan Message
	done   chan struct{}

	rm sync.Mutex
	wm sync.Mutex
}

// Connect will connect to the manager on the provided url.
func Connect(url string) (*Client, error) {
	// dial server
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return &Client{ws: ws}, nil
}

// Context returns the original fire context for server-side connections. It
// returns nil for clients created via Connect.
func (c *Client) Context() *fire.Context {
	return c.ctx
}

// Send will write the message synchronously to the underlying connection. It
// is intended for client-side use; server-side callbacks should use Enqueue
// to benefit from per-connection backpressure.
func (c *Client) Send(ctx context.Context, msg Message) error {
	// validate message
	err := msg.Validate()
	if err != nil {
		return err
	}

	return c.write(ctx, msg)
}

// Enqueue will queue the message for sending. It returns an error if the
// connection has been closed or the outbox is full. A full outbox indicates
// a slow consumer; the caller should drop the connection in that case.
func (c *Client) Enqueue(msg Message) error {
	// validate message
	err := msg.Validate()
	if err != nil {
		return err
	}

	select {
	case <-c.done:
		return fmt.Errorf("client closed")
	default:
	}

	select {
	case c.outbox <- msg:
		return nil
	case <-c.done:
		return fmt.Errorf("client closed")
	default:
		return fmt.Errorf("outbox full")
	}
}

// Receive will read the next message from the connection.
func (c *Client) Receive(ctx context.Context, msg Message) error {
	// acquire mutex
	c.rm.Lock()
	defer c.rm.Unlock()

	// set read deadline
	deadline, hasDeadline := deadlineFromCtx(ctx)
	if hasDeadline {
		err := c.ws.SetReadDeadline(deadline)
		if err != nil {
			return err
		}
	}

	// read next message
	typ, bytes, err := c.ws.ReadMessage()
	if err != nil {
		return err
	} else if typ != websocket.TextMessage {
		return fmt.Errorf("expected text message, got: %d", typ)
	}

	// unset read deadline
	if hasDeadline {
		err = c.ws.SetReadDeadline(time.Time{})
		if err != nil {
			return err
		}
	}

	// decode message
	err = GetMeta(msg).Coding.Unmarshal(bytes, msg)
	if err != nil {
		return err
	}

	// validate message
	return msg.Validate()
}

// Close will close the connection.
func (c *Client) Close() error {
	return c.ws.Close()
}

// write encodes and writes a message to the underlying socket. The caller
// must have validated the message.
func (c *Client) write(ctx context.Context, msg Message) error {
	// acquire mutex
	c.wm.Lock()
	defer c.wm.Unlock()

	// encode message
	bytes, err := GetMeta(msg).Coding.Marshal(msg)
	if err != nil {
		return err
	}

	// derive write deadline
	deadline, hasDeadline := deadlineFromCtx(ctx)
	if !hasDeadline {
		deadline = time.Now().Add(writeTimeout)
		hasDeadline = true
	}

	// set write deadline
	err = c.ws.SetWriteDeadline(deadline)
	if err != nil {
		return err
	}

	// write message
	err = c.ws.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		return err
	}

	// unset write deadline
	return c.ws.SetWriteDeadline(time.Time{})
}

func deadlineFromCtx(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	return ctx.Deadline()
}
