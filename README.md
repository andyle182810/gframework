# gframework

**gframework** is a production-ready Go microservices framework providing essential building blocks for building scalable backend services.

It is designed with best practices, enterprise-grade patterns, and idiomatic Go conventions.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## ✨ Features

- 🌐 **HTTP Server**
  Echo-based REST API server with preconfigured middleware.

- 🗄️ **PostgreSQL Integration**
  Connection pooling with pgx/v5 and migration support.

- 🔴 **Redis Client**
  Connection pooling, health checks, and pub/sub messaging.

- 📨 **Message Streaming**
  Redis Streams-based pub/sub with Watermill integration.

- 🔐 **JWT Authentication**
  JWKS-based token validation middleware.

- 🎯 **Service Runner**
  Graceful lifecycle management for multiple services.

- 📝 **Request Logging**
  Structured logging with request-scoped context.

- ✅ **Validation**
  Request validation with go-playground/validator.

- 📊 **Metrics Server**
  Prometheus metrics endpoint built-in.

- 🧪 **Testing Utilities**
  Testcontainers integration for integration tests.

## 📦 Installation

```bash
go get github.com/andyle182810/gframework
```

## 🚀 Quick Start

```go
package main

import (
    "context"
    "time"

    "github.com/andyle182810/gframework/httpserver"
    "github.com/labstack/echo/v5"
    "github.com/rs/zerolog/log"
)

func main() {
    cfg := &httpserver.Config{
        Host:         "0.0.0.0",
        Port:         8080,
        EnableCors:   true,
        BodyLimit:    "10M",
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        GracePeriod:  10 * time.Second,
    }

    server := httpserver.New(cfg)

    server.Root.GET("/health", func(c echo.Context) error {
        return c.JSON(200, map[string]string{"status": "ok"})
    })

    if err := server.Run(context.Background()); err != nil {
        log.Fatal().Err(err).Msg("Server failed")
    }
}
```

## 🧪 Testing

The framework includes testcontainers integration for integration testing:

```go
import (
    "github.com/andyle182810/gframework/testutil"
)

func TestWithPostgres(t *testing.T) {
    container, connString := testutil.SetupPostgresContainer(t)
    defer container.Terminate(context.Background())

    db, err := postgres.New(&postgres.Config{URL: connString})
    // Run tests...
}
```

Run tests:

```bash
go test ./...
```

## ⚡ Middleware

The framework includes several built-in middleware:

- **Request Logger** - Structured request/response logging
- **Error Handler** - Standardized error responses
- **JWKS Auth** - JWT token validation with JWKS
- **CORS** - Cross-origin resource sharing
- **Body Limit** - Request body size limiting

## 🛠️ Error Handling

Errors follow Go best practices:

- All errors are defined as package-level variables
- Sentinel errors use the `Err` prefix (e.g., `ErrConfigNil`)
- Errors are wrapped with context using `fmt.Errorf` with `%w`

## 📚 Dependencies

Key dependencies:

- [Echo v5](https://echo.labstack.com/) - HTTP framework
- [pgx v5](https://github.com/jackc/pgx) - PostgreSQL driver
- [go-redis v9](https://github.com/redis/go-redis) - Redis client
- [Watermill](https://watermill.io/) - Message streaming
- [zerolog](https://github.com/rs/zerolog) - Logging
- [testcontainers-go](https://golang.testcontainers.org/) - Testing

## 💡 Examples

See the [examples](examples/) directory for complete working examples.

## 📬 Support

For bugs, questions, or feature requests:

- Open an issue on GitHub
  👉 [https://github.com/andyle182810/gframework/issues](https://github.com/andyle182810/gframework/issues)

## 📄 License

**gframework** is licensed under the **MIT License**.
See the [LICENSE](LICENSE) file for details.
