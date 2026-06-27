package websocket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andyle182810/gframework/websocket"
	gws "github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func startHub(t *testing.T, cfg *websocket.Config) *websocket.Hub {
	t.Helper()

	hub := websocket.New(cfg)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	started := make(chan struct{})
	go func() {
		close(started)
		_ = hub.Start(ctx)
	}()

	<-started
	require.Eventually(
		t,
		hub.IsHealthy,
		5*time.Second,
		50*time.Millisecond,
		"hub should be healthy after start",
	)

	return hub
}

func setupServer(
	t *testing.T,
	hub *websocket.Hub,
	message websocket.MessageHandler,
) *httptest.Server {
	t.Helper()

	echo := echo.New()
	echo.GET("/ws", hub.Handler(message))

	service := httptest.NewServer(echo)
	t.Cleanup(service.Close)

	return service
}

func dialWS(t *testing.T, service *httptest.Server) *gws.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(service.URL, "http") + "/ws"

	conn, _, err := gws.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func TestHub_Name(t *testing.T) {
	t.Parallel()

	hub := websocket.New(nil)
	require.Equal(t, "websocket", hub.Name())
}

func TestHub_IsHealthy_BeforeStart(t *testing.T) {
	t.Parallel()

	hub := startHub(t, nil)
	require.True(t, hub.IsHealthy())
}

func TestHub_Start_AlreadyRunning(t *testing.T) {
	t.Parallel()

	hub := startHub(t, nil)

	err := hub.Start(t.Context())
	require.ErrorIs(t, err, websocket.ErrHubAlreadyRunning)
}

func TestHub_Stop_NotRunning(t *testing.T) {
	t.Parallel()

	hub := websocket.New(nil)

	require.NoError(t, hub.Stop())
	require.NoError(t, hub.Stop())
}

func TestHub_Stop_Graceful(t *testing.T) {
	t.Parallel()

	hub := websocket.New(nil)

	stopped := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		stopped <- hub.Start(t.Context())
	}()

	<-started
	require.Eventually(t, hub.IsHealthy, 5*time.Second, 50*time.Millisecond)

	require.NoError(t, hub.Stop())

	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("hub did not stop within timeout")
	}

	require.False(t, hub.IsHealthy())
}

func TestHub_ConnectionCount(t *testing.T) {
	t.Parallel()

	hub := startHub(t, nil)
	srv := setupServer(t, hub, nil)

	require.Equal(t, 0, hub.ConnectionCount())

	wsConn := dialWS(t, srv)

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 1
	}, 5*time.Second, 50*time.Millisecond)

	wsConn.Close()

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 0
	}, 5*time.Second, 50*time.Millisecond)
}

func TestHub_Broadcast(t *testing.T) {
	t.Parallel()

	hub := startHub(t, nil)
	srv := setupServer(t, hub, nil)

	client1 := dialWS(t, srv)
	client2 := dialWS(t, srv)

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 2
	}, 5*time.Second, 50*time.Millisecond)

	payload, _ := json.Marshal("hello")
	err := hub.Broadcast(websocket.Message{Type: websocket.MessageTypeText, Payload: payload})
	require.NoError(t, err)

	for _, c := range []*gws.Conn{client1, client2} {
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, readErr := c.ReadMessage()
		require.NoError(t, readErr)

		var msg websocket.Message
		require.NoError(t, json.Unmarshal(data, &msg))
		require.Equal(t, websocket.MessageTypeText, msg.Type)
	}
}

func TestHub_Broadcast_WhenNotRunning(t *testing.T) {
	t.Parallel()

	hub := websocket.New(nil)
	payload, _ := json.Marshal("test")
	msg := websocket.Message{Type: websocket.MessageTypeText, Payload: payload}

	require.ErrorIs(t, hub.Broadcast(msg), websocket.ErrHubNotRunning)
	require.ErrorIs(t, hub.BroadcastToRoom("room-1", msg), websocket.ErrHubNotRunning)
	require.ErrorIs(t, hub.Send("conn-id", msg), websocket.ErrHubNotRunning)
}

func TestHub_MessageHandler(t *testing.T) {
	t.Parallel()

	received := make(chan websocket.Message, 1)
	mh := func(_ context.Context, _ *websocket.Conn, msg websocket.Message) error {
		select {
		case received <- msg:
		default:
		}
		return nil
	}

	hub := startHub(t, nil)
	srv := setupServer(t, hub, mh)

	wsConn := dialWS(t, srv)
	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 1
	}, 5*time.Second, 50*time.Millisecond)

	payload, _ := json.Marshal("test payload")
	err := wsConn.WriteJSON(websocket.Message{Type: websocket.MessageTypeText, Payload: payload})
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, websocket.MessageTypeText, msg.Type)
	case <-time.After(5 * time.Second):
		t.Fatal("message handler was not called")
	}
}

func TestHub_Send(t *testing.T) {
	t.Parallel()

	connIDCh := make(chan string, 1)
	mh := func(_ context.Context, conn *websocket.Conn, _ websocket.Message) error {
		select {
		case connIDCh <- conn.ID:
		default:
		}
		return nil
	}

	hub := startHub(t, nil)
	srv := setupServer(t, hub, mh)

	client1 := dialWS(t, srv)
	client2 := dialWS(t, srv)

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 2
	}, 5*time.Second, 50*time.Millisecond)

	err := client1.WriteJSON(websocket.Message{Type: websocket.MessageTypeText})
	require.NoError(t, err)

	var connID string
	select {
	case connID = <-connIDCh:
	case <-time.After(5 * time.Second):
		t.Fatal("message handler was not called")
	}

	payload, _ := json.Marshal("direct")
	err = hub.Send(connID, websocket.Message{Type: websocket.MessageTypeText, Payload: payload})
	require.NoError(t, err)

	client1.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := client1.ReadMessage()
	require.NoError(t, err)

	var msg websocket.Message
	require.NoError(t, json.Unmarshal(data, &msg))
	require.Equal(t, websocket.MessageTypeText, msg.Type)

	client2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = client2.ReadMessage()
	require.Error(t, err)
}

func TestHub_BroadcastToRoom(t *testing.T) {
	t.Parallel()

	connIDCh := make(chan string, 1)
	mh := func(_ context.Context, conn *websocket.Conn, _ websocket.Message) error {
		select {
		case connIDCh <- conn.ID:
		default:
		}
		return nil
	}

	hub := startHub(t, nil)
	srv := setupServer(t, hub, mh)

	client1 := dialWS(t, srv)
	client2 := dialWS(t, srv)

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 2
	}, 5*time.Second, 50*time.Millisecond)

	err := client1.WriteJSON(websocket.Message{Type: websocket.MessageTypeText})
	require.NoError(t, err)

	var connID string
	select {
	case connID = <-connIDCh:
	case <-time.After(5 * time.Second):
		t.Fatal("message handler was not called")
	}

	hub.JoinRoom(connID, "room-A")
	require.Equal(t, 1, hub.RoomSize("room-A"))

	payload, _ := json.Marshal("room message")
	err = hub.BroadcastToRoom("room-A", websocket.Message{
		Type:    websocket.MessageTypeText,
		Room:    "room-A",
		Payload: payload,
	})
	require.NoError(t, err)

	client1.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := client1.ReadMessage()
	require.NoError(t, err)

	var msg websocket.Message
	require.NoError(t, json.Unmarshal(data, &msg))
	require.Equal(t, "room-A", msg.Room)

	client2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = client2.ReadMessage()
	require.Error(t, err)
}

func TestHub_JoinRoom_LeaveRoom(t *testing.T) {
	t.Parallel()

	connIDCh := make(chan string, 1)
	mh := func(_ context.Context, conn *websocket.Conn, _ websocket.Message) error {
		select {
		case connIDCh <- conn.ID:
		default:
		}
		return nil
	}

	hub := startHub(t, nil)
	srv := setupServer(t, hub, mh)

	wsConn := dialWS(t, srv)
	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 1
	}, 5*time.Second, 50*time.Millisecond)

	err := wsConn.WriteJSON(websocket.Message{Type: websocket.MessageTypeText})
	require.NoError(t, err)

	var connID string
	select {
	case connID = <-connIDCh:
	case <-time.After(5 * time.Second):
		t.Fatal("message handler was not called")
	}

	require.Equal(t, 0, hub.RoomSize("room-1"))

	hub.JoinRoom(connID, "room-1")
	require.Equal(t, 1, hub.RoomSize("room-1"))

	hub.JoinRoom(connID, "room-1")
	require.Equal(t, 1, hub.RoomSize("room-1"))

	hub.LeaveRoom(connID, "room-1")
	require.Equal(t, 0, hub.RoomSize("room-1"))

	hub.LeaveRoom(connID, "room-1")
}

func TestHub_MaxConnections(t *testing.T) {
	t.Parallel()

	hub := startHub(t, &websocket.Config{MaxConnections: 1})
	srv := setupServer(t, hub, nil)

	_ = dialWS(t, srv)

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 1
	}, 5*time.Second, 50*time.Millisecond)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	_, resp, err := gws.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestHub_Disconnect_CleansUp(t *testing.T) {
	t.Parallel()

	connIDCh := make(chan string, 1)
	mh := func(_ context.Context, conn *websocket.Conn, _ websocket.Message) error {
		select {
		case connIDCh <- conn.ID:
		default:
		}
		return nil
	}

	hub := startHub(t, nil)
	srv := setupServer(t, hub, mh)

	wsConn := dialWS(t, srv)
	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 1
	}, 5*time.Second, 50*time.Millisecond)

	err := wsConn.WriteJSON(websocket.Message{Type: websocket.MessageTypeText})
	require.NoError(t, err)

	var connID string
	select {
	case connID = <-connIDCh:
	case <-time.After(5 * time.Second):
		t.Fatal("message handler was not called")
	}

	hub.JoinRoom(connID, "room-cleanup")
	require.Equal(t, 1, hub.RoomSize("room-cleanup"))

	wsConn.Close()

	require.Eventually(t, func() bool {
		return hub.ConnectionCount() == 0
	}, 5*time.Second, 50*time.Millisecond)

	require.Equal(t, 0, hub.RoomSize("room-cleanup"))
}
