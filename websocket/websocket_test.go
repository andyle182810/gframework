package websocket_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/websocket"
	gws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startUpgradeServer(t *testing.T, wsServer *websocket.WebSocket) (*httptest.Server, <-chan *websocket.Conn) {
	t.Helper()

	connCh := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsServer.Upgrade(w, r)
		if err != nil {
			return
		}

		connCh <- conn
	}))

	t.Cleanup(server.Close)

	return server, connCh
}

func newTestWS(t *testing.T) *websocket.WebSocket {
	t.Helper()

	ws, err := websocket.New(&websocket.Config{}) //nolint:exhaustruct
	require.NoError(t, err)

	return ws
}

func dialClient(t *testing.T, server *httptest.Server) *gws.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client, resp, err := gws.DefaultDialer.DialContext(testutil.Context(t), wsURL, nil)
	require.NoError(t, err)

	if resp != nil {
		defer resp.Body.Close()
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestNew_NilConfig(t *testing.T) {
	t.Parallel()

	ws, err := websocket.New(nil)
	assert.Nil(t, ws)
	assert.ErrorIs(t, err, websocket.ErrConfigNil)
}

func TestNew_EmptyConfig(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, newTestWS(t))
}

func TestConfig_WithDefaults(t *testing.T) {
	t.Parallel()

	cfg := (&websocket.Config{}).WithDefaults() //nolint:exhaustruct

	assert.Positive(t, cfg.ReadBufferSize)
	assert.Positive(t, cfg.WriteBufferSize)
	assert.Positive(t, cfg.PingInterval)
	assert.Positive(t, cfg.PingTimeout)
	assert.Positive(t, cfg.WriteTimeout)
	assert.Positive(t, cfg.ReadLimit)
}

func TestConfig_WithDefaults_PreservesCustomValues(t *testing.T) {
	t.Parallel()

	custom := &websocket.Config{ //nolint:exhaustruct
		ReadBufferSize: 42,
		PingInterval:   time.Second,
	}

	cfg := custom.WithDefaults()

	assert.Equal(t, 42, cfg.ReadBufferSize)
	assert.Equal(t, time.Second, cfg.PingInterval)
}

func TestPayload_Unpack(t *testing.T) {
	t.Parallel()

	type payloadBody struct {
		Name string `json:"name"`
	}

	payload := websocket.Payload(`{"name":"gframework"}`)

	var got payloadBody

	require.NoError(t, payload.Unpack(&got))
	assert.Equal(t, "gframework", got.Name)
}

func TestPayload_Unpack_Empty(t *testing.T) {
	t.Parallel()

	var got map[string]any

	err := websocket.Payload(nil).Unpack(&got)
	assert.ErrorIs(t, err, websocket.ErrPayloadEmpty)
}

func TestWS_Upgrade(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	_ = dialClient(t, server)

	serverConn := <-connCh

	assert.NotNil(t, serverConn)
}

func TestWS_Upgrade_BadRequest(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, _ := startUpgradeServer(t, ws)

	req, err := http.NewRequestWithContext(testutil.Context(t), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestConn_ReadMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		messageType int
		payload     []byte
	}{
		{name: "text", messageType: gws.TextMessage, payload: []byte("hello")},
		{name: "binary", messageType: gws.BinaryMessage, payload: []byte{0x01, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := newTestWS(t)
			server, connCh := startUpgradeServer(t, ws)
			client := dialClient(t, server)

			serverConn := <-connCh

			require.NoError(t, client.WriteMessage(tt.messageType, tt.payload))

			msgType, payload, err := serverConn.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, tt.messageType, msgType)
			assert.Equal(t, tt.payload, []byte(payload))
		})
	}
}

func TestConn_Write(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	client := dialClient(t, server)

	serverConn := <-connCh

	require.NoError(t, serverConn.Write([]byte("hi")))

	msgType, payload, err := client.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gws.TextMessage, msgType)
	assert.Equal(t, []byte("hi"), payload)
}

func TestConn_WriteMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		messageType int
		payload     []byte
	}{
		{name: "text", messageType: gws.TextMessage, payload: []byte("hello")},
		{name: "binary", messageType: gws.BinaryMessage, payload: []byte{0x01, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := newTestWS(t)
			server, connCh := startUpgradeServer(t, ws)
			client := dialClient(t, server)

			serverConn := <-connCh

			require.NoError(t, serverConn.WriteMessage(tt.messageType, tt.payload))

			msgType, payload, err := client.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, tt.messageType, msgType)
			assert.Equal(t, tt.payload, payload)
		})
	}
}

func TestConn_WriteJSON(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	client := dialClient(t, server)

	serverConn := <-connCh

	type event struct {
		Name string `json:"name"`
	}

	require.NoError(t, serverConn.WriteJSON(event{Name: "ping"}))

	_, payload, err := client.ReadMessage()
	require.NoError(t, err)

	var got event

	require.NoError(t, json.Unmarshal(payload, &got))
	assert.Equal(t, "ping", got.Name)
}

func TestConn_Close(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	_ = dialClient(t, server)

	serverConn := <-connCh

	assert.NoError(t, serverConn.Close())
}

func TestConn_WriteMessage_AfterClose(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	_ = dialClient(t, server)

	serverConn := <-connCh

	require.NoError(t, serverConn.Close())

	err := serverConn.WriteMessage(gws.TextMessage, []byte("hi"))
	assert.ErrorIs(t, err, websocket.ErrConnClosed)
}

func TestConn_Close_DoesNotHang(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	_ = dialClient(t, server)

	serverConn := <-connCh

	closeDone := make(chan error, 1)

	go func() { closeDone <- serverConn.Close() }()

	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Close() did not return within 1s")
	}
}

func TestNew_NegativeConfig(t *testing.T) {
	t.Parallel()

	ws, err := websocket.New(&websocket.Config{PingInterval: -time.Second}) //nolint:exhaustruct
	assert.Nil(t, ws)
	assert.ErrorIs(t, err, websocket.ErrConfigInvalid)
}

func TestConfig_WithDefaults_DoesNotMutateReceiver(t *testing.T) {
	t.Parallel()

	original := &websocket.Config{} //nolint:exhaustruct
	_ = original.WithDefaults()

	assert.Zero(t, original.PingInterval)
}

func dialWithOrigin(t *testing.T, server *httptest.Server, origin string) error {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{"Origin": []string{origin}}

	client, resp, err := gws.DefaultDialer.DialContext(testutil.Context(t), wsURL, header)
	if resp != nil {
		defer resp.Body.Close()
	}

	if client != nil {
		t.Cleanup(func() { _ = client.Close() })
	}

	return err
}

func TestWS_Upgrade_OriginAllowed(t *testing.T) {
	t.Parallel()

	ws, err := websocket.New(&websocket.Config{AllowedOrigins: []string{"https://ok.example"}}) //nolint:exhaustruct
	require.NoError(t, err)

	server, connCh := startUpgradeServer(t, ws)

	require.NoError(t, dialWithOrigin(t, server, "https://ok.example"))
	assert.NotNil(t, <-connCh)
}

func TestWS_Upgrade_OriginDenied(t *testing.T) {
	t.Parallel()

	ws, err := websocket.New(&websocket.Config{AllowedOrigins: []string{"https://ok.example"}}) //nolint:exhaustruct
	require.NoError(t, err)

	server, _ := startUpgradeServer(t, ws)

	err = dialWithOrigin(t, server, "https://evil.example")
	assert.ErrorIs(t, err, gws.ErrBadHandshake)
}

func TestWS_Upgrade_OriginWildcard(t *testing.T) {
	t.Parallel()

	ws, err := websocket.New(&websocket.Config{AllowedOrigins: []string{"*"}}) //nolint:exhaustruct
	require.NoError(t, err)

	server, connCh := startUpgradeServer(t, ws)

	require.NoError(t, dialWithOrigin(t, server, "https://anything.example"))
	assert.NotNil(t, <-connCh)
}

func TestWS_Upgrade_CrossOriginDeniedByDefault(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, _ := startUpgradeServer(t, ws)

	err := dialWithOrigin(t, server, "https://evil.example")
	assert.ErrorIs(t, err, gws.ErrBadHandshake)
}

func TestWS_Dial_RoundTrip(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	clientConn, err := newTestWS(t).Dial(testutil.Context(t), wsURL)
	require.NoError(t, err)

	t.Cleanup(func() { _ = clientConn.Close() })

	serverConn := <-connCh

	require.NoError(t, clientConn.Write([]byte("ping")))

	msgType, payload, err := serverConn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gws.TextMessage, msgType)
	assert.Equal(t, "ping", string(payload))

	require.NoError(t, serverConn.Write([]byte("pong")))

	msgType, payload, err = clientConn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gws.TextMessage, msgType)
	assert.Equal(t, "pong", string(payload))
}

func TestWS_Dial_Refused(t *testing.T) {
	t.Parallel()

	conn, err := newTestWS(t).Dial(testutil.Context(t), "ws://127.0.0.1:1/ws")
	assert.Nil(t, conn)
	assert.Error(t, err)
}

func TestConn_ConcurrentWriteAndClose(t *testing.T) {
	t.Parallel()

	ws := newTestWS(t)
	server, connCh := startUpgradeServer(t, ws)
	client := dialClient(t, server)

	serverConn := <-connCh

	// Drain the client side so server writes never back up.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()

	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			for range 100 {
				if err := serverConn.Write([]byte("payload")); err != nil {
					assert.ErrorIs(t, err, websocket.ErrConnClosed)
				}
			}
		})
	}

	require.NoError(t, serverConn.Close())
	wg.Wait()

	err := serverConn.WriteMessage(gws.TextMessage, []byte("late"))
	assert.ErrorIs(t, err, websocket.ErrConnClosed)
}

func TestConn_PingKeepsConnectionAlive(t *testing.T) {
	t.Parallel()

	wsServer, err := websocket.New(&websocket.Config{ //nolint:exhaustruct
		PingInterval: 50 * time.Millisecond,
		PingTimeout:  250 * time.Millisecond,
	})
	require.NoError(t, err)

	server, connCh := startUpgradeServer(t, wsServer)
	client := dialClient(t, server)

	serverConn := <-connCh

	// Reading makes the client process pings, which gorilla answers with
	// pongs by default; those pongs keep the server read deadline fresh.
	go func() {
		for {
			if _, _, readErr := client.ReadMessage(); readErr != nil {
				return
			}
		}
	}()

	serverRead := make(chan error, 1)

	go func() {
		_, _, readErr := serverConn.ReadMessage()
		serverRead <- readErr
	}()

	// Sit idle for several read-deadline windows (deadline is 300ms); only
	// the ping/pong exchange can keep the connection alive this long.
	time.Sleep(900 * time.Millisecond)

	require.NoError(t, client.WriteMessage(gws.TextMessage, []byte("still alive")))

	select {
	case readErr := <-serverRead:
		require.NoError(t, readErr)
	case <-time.After(time.Second):
		t.Fatal("server ReadMessage did not return")
	}
}
