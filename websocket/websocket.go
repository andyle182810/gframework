// Package websocket upgrades HTTP connections to WebSocket connections and
// manages the resulting connection. It has no business logic: callers write
// their own read loop, decide what messages mean, and own any application
// concepts (rooms, hubs, auth) on top of the raw Conn.
//
// WebSocket.Upgrade performs the HTTP-to-WebSocket handshake and returns a
// Conn; WebSocket.Dial establishes an outbound client connection managed the
// same way. Conn.ReadMessage blocks for the next inbound frame and must be
// used from a single goroutine. Write, WriteMessage, and WriteJSON are safe for
// concurrent use: a mutex serializes them onto the underlying connection and
// each call blocks until its frame is written or WriteTimeout elapses. A
// background goroutine pings the peer every PingInterval; a peer that stops
// answering trips the read deadline, which surfaces as an error from
// ReadMessage.
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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	gws "github.com/gorilla/websocket"
)

const (
	defaultReadBufferSize  = 1024
	defaultWriteBufferSize = 1024
	defaultPingInterval    = 30 * time.Second
	defaultPingTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultReadLimit       = 512 * 1024 // 512 KB
)

var (
	ErrPayloadEmpty  = errors.New("websocket: payload is empty")
	ErrConfigNil     = errors.New("websocket: configuration must not be nil")
	ErrConfigInvalid = errors.New("websocket: configuration values must not be negative")
	ErrConnClosed    = errors.New("websocket: connection is closed")
)

type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PingTimeout     time.Duration
	WriteTimeout    time.Duration
	ReadLimit       int64
	AllowedOrigins  []string
	DisableMetrics  bool
}

func (cfg *Config) WithDefaults() *Config {
	out := *cfg

	if out.ReadBufferSize == 0 {
		out.ReadBufferSize = defaultReadBufferSize
	}

	if out.WriteBufferSize == 0 {
		out.WriteBufferSize = defaultWriteBufferSize
	}

	if out.PingInterval == 0 {
		out.PingInterval = defaultPingInterval
	}

	if out.PingTimeout == 0 {
		out.PingTimeout = defaultPingTimeout
	}

	if out.WriteTimeout == 0 {
		out.WriteTimeout = defaultWriteTimeout
	}

	if out.ReadLimit == 0 {
		out.ReadLimit = defaultReadLimit
	}

	return &out
}

func (cfg *Config) validate() error {
	if cfg.ReadBufferSize < 0 || cfg.WriteBufferSize < 0 || cfg.ReadLimit < 0 ||
		cfg.PingInterval < 0 || cfg.PingTimeout < 0 || cfg.WriteTimeout < 0 {
		return ErrConfigInvalid
	}

	return nil
}

type WebSocket struct {
	cfg            *Config
	upgrader       gws.Upgrader
	metricsEnabled bool
}

func New(cfg *Config) (*WebSocket, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg = cfg.WithDefaults()

	return &WebSocket{
		cfg:            cfg,
		upgrader:       buildUpgrader(cfg),
		metricsEnabled: !cfg.DisableMetrics,
	}, nil
}

type Payload []byte

func (p Payload) Unpack(v any) error {
	if len(p) == 0 {
		return ErrPayloadEmpty
	}

	return json.Unmarshal(p, v)
}

func (ws *WebSocket) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	wsConn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.recordConnectionAttempt("server", err)

		return nil, err
	}

	conn, err := ws.adopt(wsConn, "server")
	ws.recordConnectionAttempt("server", err)

	return conn, err
}

func (ws *WebSocket) Dial(ctx context.Context, url string) (*Conn, error) {
	dialer := gws.Dialer{ //nolint:exhaustruct
		ReadBufferSize:   ws.cfg.ReadBufferSize,
		WriteBufferSize:  ws.cfg.WriteBufferSize,
		HandshakeTimeout: ws.cfg.WriteTimeout,
	}

	wsConn, resp, err := dialer.DialContext(ctx, url, nil)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		ws.recordConnectionAttempt("client", err)

		return nil, err
	}

	conn, err := ws.adopt(wsConn, "client")
	ws.recordConnectionAttempt("client", err)

	return conn, err
}

func (ws *WebSocket) adopt(wsConn *gws.Conn, direction string) (*Conn, error) {
	wsConn.SetReadLimit(ws.cfg.ReadLimit)

	pingDeadline := ws.cfg.PingInterval + ws.cfg.PingTimeout
	if setErr := wsConn.SetReadDeadline(time.Now().Add(pingDeadline)); setErr != nil {
		_ = wsConn.Close()

		return nil, setErr
	}

	conn := newConn(wsConn, ws.cfg.WriteTimeout, pingDeadline, direction, ws.metricsEnabled)
	if ws.metricsEnabled {
		incActiveConnection(direction)
	}

	go conn.pingLoop(ws.cfg.PingInterval)

	return conn, nil
}

func buildUpgrader(cfg *Config) gws.Upgrader {
	return gws.Upgrader{ //nolint:exhaustruct
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin:     buildCheckOrigin(cfg.AllowedOrigins),
	}
}

func buildCheckOrigin(origins []string) func(*http.Request) bool {
	if len(origins) == 0 {
		return nil
	}

	allowAll := false
	allowed := make(map[string]struct{}, len(origins))

	for _, origin := range origins {
		if origin == "*" {
			allowAll = true
		}

		allowed[origin] = struct{}{}
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || allowAll {
			return true
		}

		_, ok := allowed[origin]

		return ok
	}
}
