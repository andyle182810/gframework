package websocket_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
