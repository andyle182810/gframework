# websocket-echo

A WebSocket echo service and an interactive terminal client, both built on the
`websocket` package. The server upgrades HTTP requests with `httpserver` +
`websocket.Upgrade`; the client connects with `websocket.Dial`.

## Run

Start the server (listens on `:8090`):

```bash
go run ./cmd/server
```

Then, in another terminal, start the client:

```bash
go run ./cmd/client
```

Type a line and press Enter — the server echoes it back as JSON. Every 10
seconds the server also pushes its current time, written from a second
goroutine while the read loop runs. Exit the client with `Ctrl+C` or `Ctrl+D`;
stop the server with `Ctrl+C`.

```sh
connected to ws://localhost:8090/ws — type a message and press Enter
hello
[08:41:00] server: echo: hello
[08:41:07] server: the server time is 8:41AM
```

## What it demonstrates

- `websocket.New(&websocket.Config{})` — production defaults; same-origin
  protection only affects browsers, so the Go client (no `Origin` header) is
  always accepted
- `ws.Upgrade(c.Response(), c.Request())` inside an `httpserver` route,
  wired through the `runner` for graceful shutdown
- `ws.Dial(ctx, url)` — the client side gets the same managed `Conn`:
  concurrency-safe writes and automatic ping/pong keepalive
- Caller-owned read loop: `ReadMessage` from one goroutine, `WriteJSON` from
  any goroutine (the server's time pusher writes concurrently with its echoes)
- `Payload.Unpack` for inbound JSON, write errors as the stop signal
  (`ErrConnClosed` after `Close`, and a failed write closes the connection)
