package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	ufrpostgres "github.com/andyle182810/gframework/postgres"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultPostgresUser              = "testuser"
	defaultPostgresPassword          = "testpass"
	defaultPostgresDatabase          = "testdb"
	defaultPostgresPort     nat.Port = "5432"
	defaultPostgresImage             = "postgres:16-alpine"
)

const (
	startupTimeout    = 60 * time.Second
	startupOccurrence = 2
)

type PostgresTestContainer struct {
	Container testcontainers.Container
	User      string
	Password  string
	Host      string
	Database  string
	Port      nat.Port
}

func SetupPostgresContainer(ctx context.Context, t *testing.T) *PostgresTestContainer {
	t.Helper()

	container, err := postgres.Run(ctx,
		defaultPostgresImage,
		postgres.WithDatabase(defaultPostgresDatabase),
		postgres.WithUsername(defaultPostgresUser),
		postgres.WithPassword(defaultPostgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(startupOccurrence).
				WithStartupTimeout(startupTimeout),
		),
	)

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, defaultPostgresPort)
	require.NoError(t, err)

	return &PostgresTestContainer{
		Container: container,
		User:      defaultPostgresUser,
		Password:  defaultPostgresPassword,
		Host:      host,
		Database:  defaultPostgresDatabase,
		Port:      mappedPort,
	}
}

var errNoMigrationFile = errors.New("no migration files found")

func RunMigrations(ctx context.Context, pool *ufrpostgres.Postgres, migrationPath string) error {
	migrationFiles, err := filepath.Glob(filepath.Join(migrationPath, "*.up.sql"))
	if err != nil {
		return err
	}

	if len(migrationFiles) == 0 {
		return fmt.Errorf("%w in path: %s", errNoMigrationFile, migrationPath)
	}

	for _, file := range migrationFiles {
		migrationSQL, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		_, err = pool.Exec(ctx, string(migrationSQL))
		if err != nil {
			return err
		}
	}

	return nil
}
