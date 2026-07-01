// Package websocket upgrades HTTP connections to WebSocket connections and
// manages the resulting connection. It has no business logic: callers write
// their own read loop, decide what messages mean, and own any application
// concepts (rooms, hubs, auth) on top of the raw Conn.
//
// WS.Upgrade performs the HTTP-to-WebSocket handshake and returns a Conn.
// Conn.ReadMessage blocks for the next inbound frame; Write, WriteMessage,
// and WriteJSON queue outbound frames onto an internal channel drained by a
// single writer goroutine, so they are safe to call concurrently. A
// background ping/pong keeps idle connections alive and detects dead peers.
//
// Basic usage:
//
//	ws, err := websocket.New(&websocket.Config{AllowedOrigins: []string{"https://example.com"}})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	e.GET("/ws", func(c echo.Context) error {
//		conn, err := ws.Upgrade(c.Response(), c.Request())
//		if err != nil {
//			return err
//		}
//		defer conn.Close()
//
//		for {
//			_, data, err := conn.ReadMessage()
//			if err != nil {
//				return nil
//			}
//			// caller decides what data means
//		}
//	})
package websocket

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	gws "github.com/gorilla/websocket"
)

const (
	defaultReadBufferSize  = 1024
	defaultWriteBufferSize = 1024
	defaultPingInterval    = 30 * time.Second
	defaultPingTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultReadLimit       = 512 * 1024 // 512 KB
	defaultSendBufferSize  = 256
)

var (
	ErrSendBufferFull = errors.New("websocket: send buffer full")
	ErrPayloadEmpty   = errors.New("websocket: payload is empty")
	ErrConfigNil      = errors.New("websocket: configuration must not be nil")
)

type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PingTimeout     time.Duration
	WriteTimeout    time.Duration
	ReadLimit       int64
	AllowedOrigins  []string
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

type WS struct {
	cfg      Config
	upgrader gws.Upgrader
}

func New(cfg *Config) (*WS, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	cfg = cfg.WithDefaults()

	return &WS{
		cfg:      *cfg,
		upgrader: buildUpgrader(cfg),
	}, nil
}

type Payload []byte

func (p Payload) Unpack(v any) error {
	if len(p) == 0 {
		return ErrPayloadEmpty
	}

	return json.Unmarshal(p, v)
}

func (ws *WS) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	wsConn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	wsConn.SetReadLimit(ws.cfg.ReadLimit)

	pingDeadline := ws.cfg.PingInterval + ws.cfg.PingTimeout
	if setErr := wsConn.SetReadDeadline(time.Now().Add(pingDeadline)); setErr != nil {
		_ = wsConn.Close()

		return nil, setErr
	}

	conn := newConn(uuid.NewString(), wsConn, pingDeadline)

	go conn.writeLoop(ws.cfg.WriteTimeout, ws.cfg.PingInterval, ws.cfg.PingTimeout)

	return conn, nil
}

func buildUpgrader(cfg *Config) gws.Upgrader {
	checkOrigin := func(_ *http.Request) bool { return true }

	if len(cfg.AllowedOrigins) > 0 {
		allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
		for _, origin := range cfg.AllowedOrigins {
			allowed[origin] = struct{}{}
		}

		checkOrigin = func(r *http.Request) bool {
			_, ok := allowed[r.Header.Get("Origin")]

			return ok
		}
	}

	return gws.Upgrader{ //nolint:exhaustruct
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin:     checkOrigin,
	}
}
