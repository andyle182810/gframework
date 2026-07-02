package websocket

import (
	"encoding/json"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
)

type sendMsg struct {
	msgType int
	data    []byte
}

type Conn struct {
	ws           *gws.Conn
	send         chan sendMsg
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
	done         chan struct{}
	closeErr     error
	pingDeadline time.Duration
}

func newConn(wsConn *gws.Conn, pingDeadline time.Duration) *Conn {
	return &Conn{ //nolint:exhaustruct
		ws:           wsConn,
		send:         make(chan sendMsg, defaultSendBufferSize),
		closed:       make(chan struct{}),
		done:         make(chan struct{}),
		pingDeadline: pingDeadline,
	}
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
	c.wg.Add(1)
	defer c.wg.Done()

	select {
	case <-c.closed:
		return ErrConnClosed
	default:
	}

	msg := sendMsg{msgType: messageType, data: data}

	select {
	case c.send <- msg:
		return nil
	default:
		return ErrSendBufferFull
	}
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

func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.wg.Wait()
	})

	<-c.done

	return c.closeErr
}

func (c *Conn) writeLoop(writeTimeout, pingInterval, pingTimeout time.Duration) {
	defer close(c.done)

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	c.ws.SetPongHandler(func(_ string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pingInterval + pingTimeout))
	})

	for {
		select {
		case <-c.closed:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			_ = c.ws.WriteMessage(gws.CloseMessage, []byte{})
			c.closeErr = c.ws.Close()

			return
		case msg := <-c.send:
			if err := c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				c.closeErr = c.ws.Close()

				return
			}

			if err := c.ws.WriteMessage(msg.msgType, msg.data); err != nil {
				c.closeErr = c.ws.Close()

				return
			}
		case <-ticker.C:
			if err := c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				c.closeErr = c.ws.Close()

				return
			}

			if err := c.ws.WriteMessage(gws.PingMessage, nil); err != nil {
				c.closeErr = c.ws.Close()

				return
			}
		}
	}
}
