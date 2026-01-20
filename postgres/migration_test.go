//nolint:usetesting
package postgres_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestMigrationsPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	return "file://" + filepath.Join(filepath.Dir(filename), "testdata", "migrations")
}

func TestMigrateUp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateUp(dbURI, migrationsPath)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(2), version.Version)
	assert.False(t, version.Dirty)
}

func TestMigrateUp_NoChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateUp(dbURI, migrationsPath)
	require.NoError(t, err)

	err = postgres.MigrateUp(dbURI, migrationsPath)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(2), version.Version)
}

func TestMigrateDown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateUp(dbURI, migrationsPath)
	require.NoError(t, err)

	err = postgres.MigrateDown(dbURI, migrationsPath)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(0), version.Version)
}

func TestMigrateSteps_Up(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateSteps(dbURI, migrationsPath, 1)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(1), version.Version)
	assert.False(t, version.Dirty)

	err = postgres.MigrateSteps(dbURI, migrationsPath, 1)
	require.NoError(t, err)

	version, err = postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(2), version.Version)
}

func TestMigrateSteps_Down(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateUp(dbURI, migrationsPath)
	require.NoError(t, err)

	err = postgres.MigrateSteps(dbURI, migrationsPath, -1)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(1), version.Version)
}

func TestGetMigrationVersion_NoMigrations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(0), version.Version)
	assert.False(t, version.Dirty)
}

func TestForceMigrationVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()
	migrationsPath := getTestMigrationsPath()

	err := postgres.ForceMigrationVersion(dbURI, migrationsPath, 1)
	require.NoError(t, err)

	version, err := postgres.GetMigrationVersion(dbURI, migrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(1), version.Version)
	assert.False(t, version.Dirty)
}

func TestMigrateUp_InvalidSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container := testutil.SetupPostgresContainer(ctx, t)
	dbURI := container.ConnectionString()

	err := postgres.MigrateUp(dbURI, "file:///nonexistent/path")
	require.Error(t, err)
}

func TestMigrateUp_InvalidDBURI(t *testing.T) {
	t.Parallel()

	migrationsPath := getTestMigrationsPath()

	err := postgres.MigrateUp("postgres://invalid:invalid@localhost:9999/invalid", migrationsPath)
	require.Error(t, err)
}
