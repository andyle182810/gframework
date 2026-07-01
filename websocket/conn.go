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
	ID   string
	Meta map[string]any

	ws           *gws.Conn
	send         chan sendMsg
	once         sync.Once
	done         chan struct{}
	pingDeadline time.Duration
}

func newConn(
	id string,
	wsConn *gws.Conn,
	pingDeadline time.Duration,
) *Conn {
	return &Conn{ //nolint:exhaustruct
		ID:           id,
		Meta:         make(map[string]any),
		ws:           wsConn,
		send:         make(chan sendMsg, defaultSendBufferSize),
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
	c.once.Do(func() { close(c.send) })
	<-c.done

	return c.ws.Close()
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
		case msg, ok := <-c.send:
			if !ok {
				_ = c.ws.WriteMessage(gws.CloseMessage, []byte{})

				return
			}

			if err := c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				return
			}

			if err := c.ws.WriteMessage(msg.msgType, msg.data); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				return
			}

			if err := c.ws.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
