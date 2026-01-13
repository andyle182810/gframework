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

### HTTP Server

```go
package main

import (
    "context"
    "time"

    "github.com/andyle182810/gframework/httpserver"
    "github.com/labstack/echo/v4"
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

### PostgreSQL

```go
import (
    "github.com/andyle182810/gframework/postgres"
)

cfg := &postgres.Config{
    URL:                      "postgres://user:pass@localhost:5432/dbname",
    MaxConnection:            25,
    MinConnection:            5,
    MaxConnectionIdleTime:    15 * time.Minute,
    MaxConnectionLifetime:    1 * time.Hour,
    HealthCheckPeriod:        1 * time.Minute,
    ConnectTimeout:           5 * time.Second,
    StatementTimeout:         30 * time.Second,
}

db, err := postgres.New(cfg)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to connect to database")
}
defer db.Close()

rows, err := db.Query(ctx, "SELECT * FROM users")
```

### Redis

```go
import (
    "github.com/andyle182810/gframework/goredis"
)

cfg := &goredis.Config{
    Host:         "localhost",
    Port:         6379,
    Password:     "",
    DB:           0,
    PoolSize:     10,
    MinIdleConns: 5,
}

client, err := goredis.New(cfg)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to connect to Redis")
}
defer client.Close()

err = client.Set(ctx, "key", "value", 0).Err()
```

### Redis Pub/Sub

```go
import (
    "github.com/andyle182810/gframework/redispub"
    "github.com/andyle182810/gframework/redissub"
    "github.com/ThreeDotsLabs/watermill/message"
)

// Publisher
publisher, err := redispub.New(redisClient, redispub.Options{
    MaxStreamEntries: 10000,
})

err = publisher.Publish(ctx, "events.user.created", []byte(`{"user_id": "123"}`))

// Subscriber
subscriber, err := redissub.NewSubscriber(
    redisClient,
    "consumer-group",
    "events.user.created",
    func(ctx context.Context, payload message.Payload) error {
        // Handle message
        return nil
    },
)

err = subscriber.Run(ctx)
```

### JWT Authentication

```go
import (
    "github.com/andyle182810/gframework/middleware"
    "github.com/andyle182810/gframework/keyprovider"
)

keyFunc := keyprovider.New("https://auth.example.com/.well-known/jwks.json")
auth := middleware.NewJwksAuth(keyFunc)

server.Root.GET("/protected", handler, auth.Middleware())
```

### Service Runner

```go
import (
    "github.com/andyle182810/gframework/runner"
)

httpService := httpserver.New(httpCfg)
metricsService := metricserver.New(metricsCfg)

r := runner.New(
    runner.WithCoreServices(httpService),
    runner.WithInfrastructureServices(metricsService),
)

if err := r.Run(context.Background()); err != nil {
    log.Fatal().Err(err).Msg("Runner failed")
}
```

## 📦 Package Overview

### Core Packages

| Package                       | Description                                              |
| ----------------------------- | -------------------------------------------------------- |
| **[httpserver](httpserver/)** | Echo-based HTTP server with preconfigured middleware     |
| **[postgres](postgres/)**     | PostgreSQL client with connection pooling and migrations |
| **[goredis](goredis/)**       | Redis client wrapper with health checks                  |
| **[runner](runner/)**         | Service lifecycle manager with graceful shutdown         |

### Messaging

| Package                   | Description                                   |
| ------------------------- | --------------------------------------------- |
| **[redispub](redispub/)** | Redis Streams publisher using Watermill       |
| **[redissub](redissub/)** | Redis Streams subscriber with consumer groups |

### Authentication & Authorization

| Package                         | Description                                         |
| ------------------------------- | --------------------------------------------------- |
| **[middleware](middleware/)**   | HTTP middleware (JWT auth, logging, error handling) |
| **[kctoken](kctoken/)**         | Keycloak service token client                       |
| **[keyprovider](keyprovider/)** | JWKS key provider for JWT validation                |

### Utilities

| Package                           | Description                        |
| --------------------------------- | ---------------------------------- |
| **[validator](validator/)**       | Request validation                 |
| **[pagination](pagination/)**     | Pagination utilities               |
| **[metricserver](metricserver/)** | Prometheus metrics server          |
| **[testutil](testutil/)**         | Testing helpers and testcontainers |

---

## 🔧 Configuration

All packages follow a consistent configuration pattern:

1. Create a `Config` struct with required parameters
2. Pass to `New()` constructor
3. Configuration is validated on initialization
4. Errors are returned for invalid configuration

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

## 🗄️ Database Migrations

PostgreSQL migrations are supported via golang-migrate:

```go
import (
    "github.com/andyle182810/gframework/postgres"
)

db, err := postgres.New(cfg)
err = db.Migrate(ctx, "file://migrations")
```

## 📝 Logging

All packages use [zerolog](https://github.com/rs/zerolog) for structured logging:

```go
import (
    "github.com/rs/zerolog/log"
)

log.Info().Str("component", "server").Msg("Starting server")
```

Request context is automatically propagated through the logger in HTTP handlers.

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

- [Echo v4](https://echo.labstack.com/) - HTTP framework
- [pgx v5](https://github.com/jackc/pgx) - PostgreSQL driver
- [go-redis v9](https://github.com/redis/go-redis) - Redis client
- [Watermill](https://watermill.io/) - Message streaming
- [zerolog](https://github.com/rs/zerolog) - Logging
- [testcontainers-go](https://golang.testcontainers.org/) - Testing

## 💡 Examples

See the [examples](examples/) directory for complete working examples.

## 🤝 Contributing

Contributions are welcome and appreciated.

1. Fork the repository
2. Create a feature branch

   ```bash
   git checkout -b feature/my-feature
   ```

3. Commit your changes

   ```bash
   git commit -m "Add my feature"
   ```

4. Push to your fork

   ```bash
   git push origin feature/my-feature
   ```

5. Open a Pull Request

## 📬 Support

For bugs, questions, or feature requests:

- Open an issue on GitHub
  👉 [https://github.com/andyle182810/gframework/issues](https://github.com/andyle182810/gframework/issues)

## 📄 License

**gframework** is licensed under the **MIT License**.
See the [LICENSE](LICENSE) file for details.
