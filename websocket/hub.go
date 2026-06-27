// Package websocket provides a WebSocket hub for managing concurrent connections,
// rooms, and message broadcasting, integrated with Echo v5 and the runner lifecycle.
//
// Basic usage:
//
//	hub := websocket.New(
//	    websocket.WithMaxConnections(1000),
//	    websocket.WithAllowedOrigins([]string{"https://example.com"}),
//	)
//
//	server.Echo.GET("/ws", hub.Handler(func(ctx context.Context, conn *websocket.Conn, msg websocket.Message) error {
//	    if msg.Type == websocket.MessageTypeSubscribe {
//	        var req struct{ Room string `json:"room"` }
//	        json.Unmarshal(msg.Payload, &req)
//	        hub.JoinRoom(conn.ID, req.Room)
//	    }
//	    return nil
//	}))
//
//	r := runner.New(
//	    runner.WithCoreService(hub),
//	    runner.WithCoreService(server),
//	)
//	r.Run()
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/google/uuid"
	gws "github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog/log"
)

var (
	ErrHubNotRunning     = errors.New("websocket: hub is not running")
	ErrHubAlreadyRunning = errors.New("websocket: hub is already running")
	ErrMaxConnections    = errors.New("websocket: maximum connections reached")
)

const (
	MessageTypeText      = "text"
	MessageTypeSystem    = "system"
	MessageTypeSubscribe = "subscribe"
)

type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type MessageHandler func(ctx context.Context, conn *Conn, msg Message) error

type broadcastMsg struct {
	msg  Message
	room string
	id   string
}

const (
	defaultReadBufferSize  = 1024
	defaultWriteBufferSize = 1024
	defaultPingInterval    = 30 * time.Second
	defaultPingTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultReadLimit       = 512 * 1024 // 512 KB
	defaultSendBufferSize  = 256
)

type Config struct {
	MaxConnections  int
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PingTimeout     time.Duration
	WriteTimeout    time.Duration
	ReadLimit       int64
	AllowedOrigins  []string
}

type Hub struct {
	config      Config
	connections map[string]*Conn
	rooms       map[string]map[string]*Conn
	mu          sync.RWMutex
	register    chan *Conn
	unregister  chan *Conn
	broadcast   chan broadcastMsg
	shutdown    chan struct{}
	stopped     chan struct{}
	running     atomic.Bool
	healthy     atomic.Bool
}

func (cfg *Config) WithDefaults() *Config {
	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = defaultReadBufferSize
	}
	if cfg.WriteBufferSize == 0 {
		cfg.WriteBufferSize = defaultWriteBufferSize
	}
	if cfg.PingInterval == 0 {
		cfg.PingInterval = defaultPingInterval
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = defaultPingTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.ReadLimit == 0 {
		cfg.ReadLimit = defaultReadLimit
	}
	return cfg
}

func New(cfg *Config) *Hub {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg = cfg.WithDefaults()
	return &Hub{
		config:      *cfg,
		connections: make(map[string]*Conn),
		rooms:       make(map[string]map[string]*Conn),
		register:    make(chan *Conn, 64),
		unregister:  make(chan *Conn, 64),
		broadcast:   make(chan broadcastMsg, 256),
		shutdown:    make(chan struct{}),
		stopped:     make(chan struct{}),
	}
}

func (h *Hub) Name() string {
	return "websocket"
}

func (h *Hub) IsHealthy() bool {
	return h.healthy.Load()
}

func (h *Hub) Start(ctx context.Context) error {
	if !h.running.CompareAndSwap(false, true) {
		return ErrHubAlreadyRunning
	}
	defer func() {
		close(h.stopped)
		h.running.Store(false)
	}()

	if len(h.config.AllowedOrigins) == 0 {
		log.Warn().
			Str("source", "gframework").
			Str("service_name", h.Name()).
			Msg("WebSocket AllowedOrigins is empty — all origins are permitted; set AllowedOrigins in production")
	}

	log.Info().
		Str("source", "gframework").
		Str("service_name", h.Name()).
		Msg("WebSocket hub started")

	h.healthy.Store(true)

	for {
		select {
		case <-ctx.Done():
			h.healthy.Store(false)
			h.closeAllConnections()
			return ctx.Err()

		case <-h.shutdown:
			h.healthy.Store(false)
			h.closeAllConnections()

			log.Info().
				Str("source", "gframework").
				Str("service_name", h.Name()).
				Msg("WebSocket hub shutdown complete")

			return nil

		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn.ID] = conn
			h.mu.Unlock()

			log.Debug().
				Str("source", "gframework").
				Str("conn_id", conn.ID).
				Int("total", h.ConnectionCount()).
				Msg("WebSocket client connected")

		case conn := <-h.unregister:
			h.mu.Lock()
			h.removeConnLocked(conn)
			h.mu.Unlock()

			log.Debug().
				Str("source", "gframework").
				Str("conn_id", conn.ID).
				Int("total", h.ConnectionCount()).
				Msg("WebSocket client disconnected")

		case bcast := <-h.broadcast:
			h.deliverBroadcast(bcast)
		}
	}
}

func (h *Hub) Stop() error {
	if !h.running.Load() {
		return nil
	}
	close(h.shutdown)
	timeout := h.config.PingInterval + h.config.PingTimeout
	select {
	case <-h.stopped:
	case <-time.After(timeout):
		log.Warn().Str("source", "gframework").Str("service_name", h.Name()).
			Msg("WebSocket hub stop timed out")
	}
	return nil
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

func (h *Hub) RoomSize(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

func (h *Hub) JoinRoom(connID, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn, ok := h.connections[connID]
	if !ok {
		return
	}
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[string]*Conn)
	}
	h.rooms[room][connID] = conn
	conn.addRoom(room)
}

func (h *Hub) LeaveRoom(connID, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.leaveRoomLocked(connID, room)
}

func (h *Hub) Broadcast(msg Message) error {
	return h.enqueue(broadcastMsg{
		msg: msg,
	})
}

func (h *Hub) BroadcastToRoom(room string, msg Message) error {
	return h.enqueue(broadcastMsg{
		msg:  msg,
		room: room,
	})
}

func (h *Hub) Send(connID string, msg Message) error {
	return h.enqueue(broadcastMsg{
		msg: msg,
		id:  connID,
	})
}

func (h *Hub) enqueue(bcast broadcastMsg) error {
	if !h.running.Load() {
		return ErrHubNotRunning
	}
	select {
	case h.broadcast <- bcast:
		return nil
	default:
		return fmt.Errorf("%w: broadcast channel is full", ErrHubNotRunning)
	}
}

func (h *Hub) deliverBroadcast(bcast broadcastMsg) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch {
	case bcast.id != "":
		if conn, ok := h.connections[bcast.id]; ok {
			h.sendToConn(conn, bcast.msg)
		}
	case bcast.room != "":
		for _, conn := range h.rooms[bcast.room] {
			h.sendToConn(conn, bcast.msg)
		}
	default:
		for _, conn := range h.connections {
			h.sendToConn(conn, bcast.msg)
		}
	}
}

func (h *Hub) sendToConn(conn *Conn, msg Message) {
	select {
	case conn.send <- msg:
	default:
		log.Warn().Str("source", "gframework").Str("conn_id", conn.ID).
			Msg("WebSocket send buffer full, dropping message")
	}
}

func (h *Hub) closeAllConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, conn := range h.connections {
		close(conn.send)
	}
	h.connections = make(map[string]*Conn)
	h.rooms = make(map[string]map[string]*Conn)
}

func (h *Hub) removeConnLocked(conn *Conn) {
	if _, ok := h.connections[conn.ID]; !ok {
		return
	}

	conn.roomMu.RLock()
	rooms := make([]string, 0, len(conn.rooms))
	for room := range conn.rooms {
		rooms = append(rooms, room)
	}
	conn.roomMu.RUnlock()

	for _, room := range rooms {
		h.leaveRoomLocked(conn.ID, room)
	}
	delete(h.connections, conn.ID)
	close(conn.send)
}

func (h *Hub) leaveRoomLocked(connID, room string) {
	if conn, ok := h.connections[connID]; ok {
		conn.removeRoom(room)
	}
	if members, ok := h.rooms[room]; ok {
		delete(members, connID)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
}

func (h *Hub) Handler(mh MessageHandler) echo.HandlerFunc {
	upgrader := h.buildUpgrader()
	return func(c *echo.Context) error {
		if h.config.MaxConnections > 0 && h.ConnectionCount() >= h.config.MaxConnections {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "max connections reached")
		}

		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return fmt.Errorf("websocket upgrade failed: %w", err)
		}
		ws.SetReadLimit(h.config.ReadLimit)

		userID := ""
		if claims, err := middleware.GetExtendedClaimsFromContext(c); err == nil {
			userID = claims.Subject
		}

		conn := newConn(uuid.NewString(), userID, ws)
		h.register <- conn
		go conn.writeLoop(h.config.WriteTimeout, h.config.PingInterval, h.config.PingTimeout)
		defer func() { h.unregister <- conn }()

		return h.readLoop(c.Request().Context(), conn, mh)
	}
}

func (h *Hub) readLoop(ctx context.Context, conn *Conn, mh MessageHandler) error {
	conn.ws.SetReadDeadline(time.Now().Add(h.config.PingInterval + h.config.PingTimeout))
	for {
		_, data, err := conn.ws.ReadMessage()
		if err != nil {
			if gws.IsUnexpectedCloseError(err, gws.CloseGoingAway, gws.CloseNormalClosure, gws.CloseNoStatusReceived) {
				log.Debug().Str("source", "gframework").Str("conn_id", conn.ID).
					Err(err).Msg("WebSocket connection closed unexpectedly")
			}
			return nil
		}
		conn.ws.SetReadDeadline(time.Now().Add(h.config.PingInterval + h.config.PingTimeout))

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if mh != nil {
			if err := mh(ctx, conn, msg); err != nil {
				log.Error().Str("source", "gframework").Str("conn_id", conn.ID).
					Str("msg_type", msg.Type).Err(err).Msg("WebSocket message handler error")
			}
		}
	}
}

func (h *Hub) buildUpgrader() gws.Upgrader {
	checkOrigin := func(r *http.Request) bool { return true }
	if len(h.config.AllowedOrigins) > 0 {
		allowed := make(map[string]struct{}, len(h.config.AllowedOrigins))
		for _, o := range h.config.AllowedOrigins {
			allowed[o] = struct{}{}
		}
		checkOrigin = func(r *http.Request) bool {
			_, ok := allowed[r.Header.Get("Origin")]
			return ok
		}
	}
	return gws.Upgrader{
		ReadBufferSize:  h.config.ReadBufferSize,
		WriteBufferSize: h.config.WriteBufferSize,
		CheckOrigin:     checkOrigin,
	}
}
