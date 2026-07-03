// WebSocket echo server built from the httpserver and websocket packages.
//
// Each connection gets a JSON echo of every message it sends, plus a server
// time push every 10 seconds. The pushes come from a separate goroutine to
// show that writes are safe from any goroutine while the read loop runs.
//
// Run it:
//
//	go run ./cmd/server
//
// then connect with the client:
//
//	go run ./cmd/client
package main

import (
	"log"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/runner"
	"github.com/andyle182810/gframework/websocket"
	"github.com/labstack/echo/v5"
)

type chatMessage struct {
	Text string `json:"text"`
}

type serverMessage struct {
	From string    `json:"from"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func main() {
	ws, err := websocket.New(&websocket.Config{}) //nolint:exhaustruct
	if err != nil {
		log.Fatal(err)
	}

	server := httpserver.New(&httpserver.Config{ //nolint:exhaustruct
		Host:        "0.0.0.0",
		Port:        8090,
		BodyLimit:   "1M",
		GracePeriod: 10 * time.Second,
	})

	server.Root.GET("/ws", func(c *echo.Context) error {
		conn, err := ws.Upgrade(c.Response(), c.Request())
		if err != nil {
			return err
		}
		defer conn.Close()

		// Closed when this handler returns, stopping the pusher goroutine.
		done := make(chan struct{})
		defer close(done)

		go pushServerTime(conn, done)

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return nil // client closed, timed out, or went away
			}

			var msg chatMessage
			if err := data.Unpack(&msg); err != nil {
				_ = conn.WriteJSON(serverMessage{
					From: "server",
					Text: `expected {"text": "..."}`,
					At:   time.Now(),
				})

				continue
			}

			reply := serverMessage{From: "server", Text: "echo: " + msg.Text, At: time.Now()}
			if err := conn.WriteJSON(reply); err != nil {
				return nil // a failed write already closed the connection
			}
		}
	})

	runner.New(
		runner.WithCoreService(server),
	).Run()
}

func pushServerTime(conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			msg := serverMessage{
				From: "server",
				Text: "the server time is " + time.Now().Format(time.Kitchen),
				At:   time.Now(),
			}
			if err := conn.WriteJSON(msg); err != nil {
				return // ErrConnClosed or a dead peer: stop pushing
			}
		}
	}
}
