# minimal-api

The smallest possible gframework service: an Echo v5 HTTP server with health and
hello endpoints, managed by the `runner` for graceful startup/shutdown.

## Run

```bash
go run .
```

```bash
curl http://localhost:8080/health
curl http://localhost:8080/hello/world
```

Stop with `Ctrl+C` — in-flight requests are drained for up to the configured
`GracePeriod` before the process exits.

## What it demonstrates

- `httpserver.New` with secure-by-default timeouts (zero values become 10s/30s/30s/120s)
- Route registration on `server.Root`
- `runner.New(...).Run()` lifecycle: starts services, blocks for SIGINT/SIGTERM,
  exits non-zero when a service fails (try starting it twice — the second process
  reports the bind error and exits instead of running without a listener)
