package testutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	gfrpostgres "github.com/andyle182810/gframework/postgres"
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
	defaultPostgresImage             = "postgres:18-alpine3.22"
	defaultPostgresPort     nat.Port = "5432"
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

func (c *PostgresTestContainer) ConnectionString() string {
	hostPort := net.JoinHostPort(c.Host, c.Port.Port())

	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		c.User, c.Password, hostPort, c.Database)
}

func (c *PostgresTestContainer) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port.Port(), c.User, c.Password, c.Database)
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

func RunMigrations(ctx context.Context, pool *gfrpostgres.Postgres, migrationPath string) error {
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

func CleanupDatabase(t *testing.T, ctx context.Context, pg *gfrpostgres.Postgres) { //nolint:revive
	t.Helper()

	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
	`

	rows, err := pg.Query(ctx, query)
	require.NoError(t, err)
	defer rows.Close()

	var tables []string

	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		require.NoError(t, err)

		tables = append(tables, tableName)
	}

	require.NoError(t, rows.Err())

	if len(tables) == 0 {
		return
	}

	for _, table := range tables {
		_, err := pg.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		require.NoError(t, err)
	}
}
