package websocket

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
)

type Conn struct {
	ws           *gws.Conn
	writeTimeout time.Duration
	pingDeadline time.Duration
	writeMu      sync.Mutex
	closeOnce    sync.Once
	closed       chan struct{}
	closeErr     error
}

func newConn(wsConn *gws.Conn, writeTimeout, pingDeadline time.Duration) *Conn {
	conn := &Conn{ //nolint:exhaustruct
		ws:           wsConn,
		writeTimeout: writeTimeout,
		pingDeadline: pingDeadline,
		closed:       make(chan struct{}),
	}

	// Installed before the caller can read and before the ping loop starts,
	// so the handler is never mutated concurrently.
	wsConn.SetPongHandler(func(_ string) error {
		return wsConn.SetReadDeadline(time.Now().Add(pingDeadline))
	})

	return conn
}

func (c *Conn) ReadMessage() (int, Payload, error) {
	msgType, raw, err := c.ws.ReadMessage()
	if err != nil {
		return 0, nil, err
	}

	_ = c.ws.SetReadDeadline(time.Now().Add(c.pingDeadline))

	return msgType, Payload(raw), nil
}

func (c *Conn) WriteMessage(messageType int, data []byte) error {
	err := c.writeFrame(messageType, data)
	if err != nil && !errors.Is(err, ErrConnClosed) {
		_ = c.Close()
	}

	return err
}

func (c *Conn) Write(data []byte) error {
	return c.WriteMessage(gws.TextMessage, data)
}

func (c *Conn) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return c.Write(data)
}

// Close sends a close frame, closes the underlying connection, and stops the
// ping loop. It is safe to call from any goroutine and more than once;
// every call returns the result of the first.
func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)

		c.writeMu.Lock()
		defer c.writeMu.Unlock()

		_ = c.ws.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		_ = c.ws.WriteMessage(gws.CloseMessage, gws.FormatCloseMessage(gws.CloseNormalClosure, ""))
		c.closeErr = c.ws.Close()
	})

	return c.closeErr
}

func (c *Conn) writeFrame(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	select {
	case <-c.closed:
		return ErrConnClosed
	default:
	}

	_ = c.ws.SetWriteDeadline(time.Now().Add(c.writeTimeout))

	return c.ws.WriteMessage(messageType, data)
}

func (c *Conn) pingLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			if err := c.writeFrame(gws.PingMessage, nil); err != nil {
				_ = c.Close()

				return
			}
		}
	}
}
