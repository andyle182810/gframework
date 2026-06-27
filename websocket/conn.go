package websocket

import (
	"encoding/json"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
)

type Conn struct {
	ID     string
	UserID string

	ws     *gws.Conn
	send   chan Message
	rooms  map[string]struct{}
	roomMu sync.RWMutex
}

func newConn(
	id string,
	userID string,
	ws *gws.Conn,
) *Conn {
	return &Conn{
		ID:     id,
		UserID: userID,
		ws:     ws,
		send:   make(chan Message, defaultSendBufferSize),
		rooms:  make(map[string]struct{}),
	}
}

func (c *Conn) addRoom(room string) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	c.rooms[room] = struct{}{}
}

func (c *Conn) removeRoom(room string) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	delete(c.rooms, room)
}

func (c *Conn) writeLoop(writeTimeout, pingInterval, pingTimeout time.Duration) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	c.ws.SetPongHandler(func(_ string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pingInterval + pingTimeout))
	})

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.ws.WriteMessage(gws.CloseMessage, []byte{})
				return
			}
			if err := c.writeJSON(msg, writeTimeout); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.ws.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}

	}
}

func (c *Conn) writeJSON(msg Message, timeout time.Duration) error {
	c.ws.SetWriteDeadline(time.Now().Add(timeout))
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.ws.WriteMessage(gws.TextMessage, data)
}
