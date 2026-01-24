package testutil

import (
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultValkeyPort = "6379/tcp"
)

type ValkeyTestContainer struct {
	Container testcontainers.Container
	Host      string
	Port      nat.Port
}

func (c *ValkeyTestContainer) Address() string {
	return c.Host + ":" + c.Port.Port()
}

func SetupValkeyContainer(t *testing.T) *ValkeyTestContainer {
	t.Helper()

	ctx := t.Context()

	//nolint:exhaustruct
	req := testcontainers.ContainerRequest{
		Image:        "valkey/valkey:latest",
		ExposedPorts: []string{defaultValkeyPort},
		WaitingFor:   wait.ForListeningPort(defaultValkeyPort).WithStartupTimeout(startupTimeout),
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

	return &ValkeyTestContainer{
		Container: container,
		Host:      host,
		Port:      port,
	}
}
