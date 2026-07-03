// Interactive WebSocket client for the echo server, using websocket.Dial.
//
// Start the server first, then:
//
//	go run ./cmd/client
//
// Each line you type is sent as a JSON message; echoes and server pushes
// print as they arrive. Exit with Ctrl+C or Ctrl+D.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andyle182810/gframework/websocket"
)

const serverURL = "ws://localhost:8090/ws"

type chatMessage struct {
	Text string `json:"text"`
}

type serverMessage struct {
	From string    `json:"from"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ws, err := websocket.New(&websocket.Config{}) //nolint:exhaustruct
	if err != nil {
		log.Fatal(err)
	}

	conn, err := ws.Dial(ctx, serverURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// The read loop prints everything the server sends. It must keep running:
	// reading is also what answers the server's keepalive pings.
	readDone := make(chan struct{})

	go func() {
		defer close(readDone)

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg serverMessage
			if err := data.Unpack(&msg); err != nil {
				continue
			}

			fmt.Printf("[%s] %s: %s\n", msg.At.Format(time.TimeOnly), msg.From, msg.Text)
		}
	}()

	// Reading stdin in a goroutine keeps the select below responsive to
	// Ctrl+C and to the server going away.
	lines := make(chan string)

	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	fmt.Println("connected to " + serverURL + " — type a message and press Enter")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nclosing connection")
			_ = conn.Close()
			<-readDone

			return
		case <-readDone:
			fmt.Println("server closed the connection")

			return
		case text, ok := <-lines:
			if !ok { // stdin EOF (Ctrl+D)
				_ = conn.Close()
				<-readDone

				return
			}

			if text == "" {
				continue
			}

			if err := conn.WriteJSON(chatMessage{Text: text}); err != nil {
				log.Println("write failed:", err)

				return
			}
		}
	}
}
