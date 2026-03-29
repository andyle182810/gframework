package service_test

import (
	"testing"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*service.Service, *repo.Repository, *valkey.Valkey) {
	t.Helper()
	ctx := t.Context()

	pgContainer := testutil.SetupPostgresContainer(t)
	valkeyContainer := testutil.SetupValkeyContainer(t)

	pg, err := postgres.New(&postgres.Config{
		URL:                      pgContainer.ConnectionString(),
		MaxConnection:            10,
		MinConnection:            2,
		MaxConnectionIdleTime:    5 * time.Minute,
		MaxConnectionLifetime:    30 * time.Minute,
		HealthCheckPeriod:        1 * time.Minute,
		ConnectTimeout:           5 * time.Second,
		LogLevel:                 tracelog.LogLevelInfo,
		StatementTimeout:         30 * time.Second,
		LockTimeout:              10 * time.Second,
		IdleInTransactionTimeout: 30 * time.Second,
	})
	require.NoError(t, err)

	err = testutil.RunMigrations(ctx, pg, "../../db/migrations")
	require.NoError(t, err)

	testutil.CleanupDatabase(t, ctx, pg)

	valkey, err := valkey.New(&valkey.Config{ //nolint:exhaustruct
		Host:     valkeyContainer.Host,
		Port:     valkeyContainer.Port.Int(),
		Password: "",
		DB:       0,
	})
	require.NoError(t, err)

	repository := repo.New(pg)
	service := service.New(repository, pg, valkey)

	return service, repository, valkey
}
