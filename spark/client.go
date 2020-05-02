package spark

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ws *websocket.Conn
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

	// prepare conn
	conn := &Client{
		ws: ws,
	}

	return conn, nil
}

// Send will send the provided message.
func (c *Client) Send(ctx context.Context, msg Message) error {
	// acquire mutex
	c.wm.Lock()
	defer c.wm.Unlock()

	// validate message
	err := msg.Validate()
	if err != nil {
		return err
	}

	// encode message
	bytes, err := GetMeta(msg).Coding.Marshal(msg)
	if err != nil {
		return err
	}

	// get deadline
	deadline, hasDeadline := ctx.Deadline()

	// set write deadline
	if hasDeadline {
		err = c.ws.SetWriteDeadline(deadline)
		if err != nil {
			return err
		}
	}

	// write message
	err = c.ws.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		return err
	}

	// unset write deadline
	if hasDeadline {
		err = c.ws.SetWriteDeadline(time.Time{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Receive will receive the message from the connection.
func (c *Client) Receive(ctx context.Context, msg Message) error {
	// acquire mutex
	c.rm.Lock()
	defer c.rm.Unlock()

	// get deadline
	deadline, hasDeadline := ctx.Deadline()

	// set read deadline
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
	err = msg.Validate()
	if err != nil {
		return err
	}

	return nil
}

// Close will close the connection.
func (c *Client) Close() error {
	return c.ws.Close()
}
