# gframework

**gframework** is a production-ready Go microservices framework providing essential building blocks for scalable backend services: an Echo v5 HTTP server, Keycloak/JWT authentication, PostgreSQL, Redis/Valkey messaging, S3-compatible object storage, and coordinated service lifecycle management.

[![CI](https://github.com/andyle182810/gframework/actions/workflows/ci.yml/badge.svg)](https://github.com/andyle182810/gframework/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.26.4-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## 📦 Installation

```bash
go get github.com/andyle182810/gframework
```

## 🧩 Packages

| Package                                           | Description                                                                                                           |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| [`httpserver`](httpserver/)                       | Echo v5 REST server with request logging, body limits, transformation, validation, error handling, CORS, and timeouts |
| [`runner`](runner/)                               | Application lifecycle manager: tiered startup (infrastructure → core), graceful shutdown, failure detection           |
| [`middleware`](middleware/)                       | JWT validation (JWKS), internal service-to-service auth, request ID, request logging, role checks, error handler      |
| [`jwks`](jwks/)                                   | JWKS key function with background refresh and rate-limited unknown-KID lookups                                        |
| [`authtoken`](authtoken/)                         | Keycloak service-account token cache with automatic refresh                                                           |
| [`keycloak`](keycloak/)                           | Keycloak admin client (users, realm roles) and UMA 2.0 permission checks                                              |
| [`httpclient`](httpclient/)                       | Type-safe JSON REST client with bearer-token injection, request-ID propagation, response size limits                  |
| [`postgres`](postgres/)                           | pgx/v5 connection pool with tracing, session timeouts, migrations, transactions, retry, batching                      |
| [`valkey`](valkey/)                               | Valkey/Redis client wrapper with TLS, pooling, and health checks                                                      |
| [`redispub`](redispub/) / [`redissub`](redissub/) | Redis Streams pub/sub (Watermill) with retries, execution timeouts, and dead-letter queues                            |
| [`taskqueue`](taskqueue/)                         | Redis-backed task queue with delayed execution and stale-task recovery                                                |
| [`distlock`](distlock/)                           | Redis distributed locks (fail-hard and fail-silent modes)                                                             |
| [`workerpool`](workerpool/)                       | Tick-driven worker pool for periodic background jobs                                                                  |
| [`cache`](cache/)                                 | Generic cache helpers and collision-safe key builders                                                                 |
| [`spaces`](spaces/)                               | DigitalOcean Spaces / S3-compatible storage with presigned URLs and key validation                                    |
| [`metricserver`](metricserver/)                   | Standalone Prometheus `/metrics` + `/status` server                                                                   |
| [`validator`](validator/)                         | Request validation via go-playground/validator with custom tags (`regexp`, …)                                         |
| [`transformer`](transformer/)                     | Struct-field transformation via go-playground/mold (`mod:` conform before validation, `scrub:` PII redaction)         |
| [`logutil`](logutil/)                             | zerolog level helpers                                                                                                 |
| [`testutil`](testutil/)                           | Testcontainers helpers (PostgreSQL, Valkey) and Echo test contexts                                                    |
| [`util`](util/)                                   | Date parsing and date-range validation helpers                                                                        |

## 🚀 Quick Start

A minimal service wired through the runner (see [examples/minimal-api](examples/minimal-api/) for the full program):

```go
package main

import (
	"net/http"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/runner"
	"github.com/labstack/echo/v5"
)

func main() {
	server := httpserver.New(&httpserver.Config{
		Host:        "0.0.0.0",
		Port:        8080,
		BodyLimit:   "10M",
		GracePeriod: 10 * time.Second,
	})

	server.Root.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Blocks until SIGINT/SIGTERM or a service failure; exits non-zero on error.
	runner.New(
		runner.WithCoreService(server),
	).Run()
}
```

Services implement the `runner.Service` interface (`Start(ctx) error`, `Stop() error`, `Name() string`). Register databases, caches, and queues with `WithInfrastructureService` — they start first and stop last; register HTTP servers and subscribers with `WithCoreService`. If you need to manage signals or exit codes yourself (or drive the runner from a test), use `RunContext(ctx) error` instead of `Run()`.

## 🔐 Security configuration

The framework applies secure defaults, but several knobs are deployment-specific — review these before going to production:

**JWT validation** (`middleware.JWTWithConfig`): algorithm pinning is on by default (asymmetric algorithms only — HS\* is rejected). Set `Issuer` and `Audiences` so tokens minted by other realms or for other services are rejected:

```go
kf, _ := jwks.New(ctx, []string{"https://auth.example.com/realms/my-realm/protocol/openid-connect/certs"})

config := middleware.DefaultJWTConfig()
config.Keyfunc = kf
config.Issuer = "https://auth.example.com/realms/my-realm"
config.Audiences = []string{"my-service"}

e.Use(middleware.JWTWithConfig(config))
```

**HTTP server timeouts** (`httpserver`, `metricserver`): when unset, `ReadHeaderTimeout` 10s, `ReadTimeout` 30s, `WriteTimeout` 30s, `IdleTimeout` 120s. Override explicitly for streaming or long-polling endpoints.

**CORS**: `EnableCors` with an empty `AllowOrigins` allows every origin (and logs a warning). Always set `AllowOrigins` in production.

**Internal service-to-service auth**: protect internal endpoints with `middleware.InternalAuth(kf, allowedClients)`; on the calling side, use `httpclient` with `WithInternalAuthHeader()` (the internal header is _not_ sent by default — see Breaking changes below).

**Error responses**: never enable `ErrorHandlerConfig.IncludeInternalErrors` in production — it exposes internal error strings to clients.

**Metrics**: `metricserver` has no authentication. Bind it to an internal interface and keep it off the public network.

**Request logs**: values of sensitive query parameters (`token`, `code`, `api_key`, `password`, …) are automatically redacted.

**Dependency scanning**: CI runs `govulncheck`. Known remaining findings are confined to `github.com/docker/docker` via testcontainers (test-only dependency, no fixed release at the time of writing).

## 🧪 Testing

The framework ships testcontainers helpers for integration tests (Docker required):

```go
func TestWithPostgres(t *testing.T) {
	container := testutil.SetupPostgresContainer(t)

	db, err := postgres.New(&postgres.Config{URL: container.ConnectionString()})
	require.NoError(t, err)
	// Run tests...
}
```

Run everything:

```bash
make test       # go test ./... (no cache)
make lint       # go vet + golangci-lint v2 (fmt + run)
```

## 💡 Examples

| Example                                                 | Shows                                                                                  |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| [examples/minimal-api](examples/minimal-api/)           | Smallest possible service: httpserver + runner + graceful shutdown                     |
| [examples/jwt-auth](examples/jwt-auth/)                 | JWKS + JWT middleware with issuer/audience pinning, realm roles, internal service auth |
| [examples/messaging-worker](examples/messaging-worker/) | Redis Streams pub/sub with retries + DLQ, periodic worker pool, distributed locks      |
| [examples/demo-api](examples/demo-api/)                 | Full showcase: PostgreSQL, migrations, Swagger, Docker Compose                         |

## 🛠️ Error handling conventions

- Sentinel errors are package-level variables with the `Err` prefix (e.g. `ErrConfigNil`)
- Errors are wrapped with `fmt.Errorf("...: %w", err)` for `errors.Is`/`errors.As`
- HTTP errors use `echo.HTTPError` and are rendered by `middleware.ErrorHandler`

## 📚 Key dependencies

- [Echo v5](https://echo.labstack.com/) — HTTP framework
- [pgx v5](https://github.com/jackc/pgx) — PostgreSQL driver
- [go-redis v9](https://github.com/redis/go-redis) — Redis/Valkey client
- [Watermill](https://watermill.io/) — message streaming
- [golang-jwt v5](https://github.com/golang-jwt/jwt) + [MicahParks/keyfunc](https://github.com/MicahParks/keyfunc) — JWT/JWKS
- [zerolog](https://github.com/rs/zerolog) — structured logging
- [testcontainers-go](https://golang.testcontainers.org/) — integration testing

## 📬 Support

For bugs, questions, or feature requests, open an issue:
👉 [https://github.com/andyle182810/gframework/issues](https://github.com/andyle182810/gframework/issues)

## 📄 License

**gframework** is licensed under the **MIT License**. See [LICENSE](LICENSE) for details.
