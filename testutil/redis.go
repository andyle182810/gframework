package testutil

import (
	"context"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultRedisPort = "6379/tcp"
)

type RedisTestContainer struct {
	Container testcontainers.Container
	Host      string
	Port      nat.Port
}

func (c *RedisTestContainer) Address() string {
	return c.Host + ":" + c.Port.Port()
}

func (c *RedisTestContainer) ConnectionString() string {
	return "redis://" + c.Address()
}

func SetupRedisContainer(ctx context.Context, t *testing.T) *RedisTestContainer {
	t.Helper()

	//nolint:exhaustruct
	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{defaultRedisPort},
		WaitingFor:   wait.ForListeningPort(defaultRedisPort).WithStartupTimeout(startupTimeout),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		ProviderType:     testcontainers.ProviderDocker,
		Logger:           &log.Logger,
		Reuse:            false,
	})

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	return &RedisTestContainer{
		Container: container,
		Host:      host,
		Port:      port,
	}
}
