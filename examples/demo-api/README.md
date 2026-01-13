# demo-api

**demo-api** is a complete example application demonstrating the usage of **gframework** with PostgreSQL and Redis/Valkey.

This example showcases best practices for building production-ready microservices with gframework.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.23-blue)](https://go.dev/)

## ✨ Features

- 🌐 **HTTP Server**
  REST API with health checks and request validation.

- 📊 **Metrics Server**
  Prometheus metrics endpoint on separate port.

- 🗄️ **PostgreSQL Integration**
  Database operations with connection pooling and transactions.

- 🔴 **Redis/Valkey Integration**
  Caching layer with connection pooling.

- 📝 **Structured Logging**
  JSON logging with request-scoped context.

- ✅ **Request Validation**
  Automatic request validation with go-playground/validator.

- 🎯 **Graceful Shutdown**
  Proper service lifecycle management with cleanup.

- 🐳 **Docker Compose**
  Easy local development environment setup.

## 🏗️ Architecture

```sh
┌─────────────────────────────────────────────────────────────┐
│                      Demo API Application                   │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │      Runner       │
                    └─────────┬─────────┘
         ┌──────────┬─────────┴────────┬──────────┐
         │          │                  │          │
    ┌────▼────┐ ┌───▼────┐      ┌──────▼─────┐ ┌──▼─────┐
    │  HTTP   │ │ Metric │      │ PostgreSQL │ │ Redis  │
    │ Server  │ │ Server │      │            │ │        │
    │ :8080   │ │ :9090  │      │ :5441      │ │ :6379  │
    └─────────┘ └────────┘      └────────────┘ └────────┘
```

The application demonstrates:

- **Multi-service orchestration** using gframework's runner
- **Separation of concerns** with infrastructure and core services
- **Clean architecture** with repository and service layers

## 📋 Prerequisites

- Go 1.23.4 or later
- Docker and Docker Compose
- Make (optional, for convenience commands)

## 🚀 Quick Start

### 1. Start Infrastructure Services

```bash
cd examples/demo-api
make docker-up
```

This will start:

- **PostgreSQL** on port `5441`
- **Valkey (Redis)** on port `6379`

### 2. Install Dependencies

```bash
make deps
```

### 3. Run the Application

```bash
make run
```

The application will start with:

- **HTTP API Server**: <http://localhost:8080>
- **Metrics Server**: <http://localhost:9090>

## 🔧 Available Make Commands

| Command            | Description           |
| ------------------ | --------------------- |
| `make run`         | Run the application   |
| `make build`       | Build the binary      |
| `make deps`        | Install dependencies  |
| `make docker-up`   | Start Docker services |
| `make docker-down` | Stop Docker services  |
| `make clean`       | Clean build artifacts |
| `make test`        | Run tests             |

## 📊 Metrics

### View Prometheus Metrics

```bash
curl http://localhost:9090/metrics
```

### View Metrics Server Status

```bash
curl http://localhost:9090/status
```

Example metrics available:

- Go runtime metrics (goroutines, memory, GC)
- HTTP request metrics (duration, status codes)
- Database connection pool metrics
- Redis connection pool metrics

## 🧪 Testing

### Run All Tests

```bash
make test
```

### Test API Endpoints

A test script is provided for manual API testing:

```bash
./test-api.sh
```

This script tests:

- Health check endpoint
- Metrics endpoint
- Sample API operations

## 📁 Project Structure

```sh
demo-api/
├── internal/
│   ├── config/          # Configuration management
│   ├── repo/            # Data repository layer
│   └── service/         # Business logic layer
├── db/
│   └── migrations/      # Database migrations
├── .env                 # Environment variables
├── docker-compose.yml   # Docker services
├── main.go             # Application entry point
├── Makefile            # Build and run commands
└── README.md           # This file
```

### Environment Variables

Create a `.env` file for local development:

```bash
cp .env.example .env
# Edit .env with your settings
```

The application uses [godotenv](https://github.com/joho/godotenv) to auto-load `.env` files.

## 📄 License

This example is part of **gframework** and follows the same **MIT License**.

See the [LICENSE](../../LICENSE) file for details.
