//nolint:exhaustruct
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/require"
)

var (
	errIntentional  = errors.New("intentional error")
	errAfterInserts = errors.New("error after inserts")
)

func setupTransactionTestPostgres(t *testing.T) (*postgres.Postgres, context.Context) {
	t.Helper()

	ctx := t.Context()

	container := testutil.SetupPostgresContainer(ctx, t)

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		container.User,
		container.Password,
		net.JoinHostPort(container.Host, container.Port.Port()),
		container.Database,
	)

	opts := &postgres.Config{
		URL:                   dbURL,
		MaxConnection:         5,
		MinConnection:         1,
		MaxConnectionIdleTime: 60 * time.Second,
		HealthCheckPeriod:     10 * time.Second,
		LogLevel:              tracelog.LogLevelTrace,
	}

	pg, err := postgres.New(opts)
	require.NoError(t, err)

	t.Cleanup(func() {
		pg.Close()
	})

	return pg, ctx
}

func TestWithTransaction_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_test (value) VALUES ($1)", "test_value")

		return execErr
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestWithTransaction_Rollback(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_rollback_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_rollback_test (value) VALUES ($1)", "should_rollback")
		if execErr != nil {
			return execErr
		}

		return errIntentional
	})

	require.Error(t, err)
	require.ErrorIs(t, err, postgres.ErrTxRolledBack)
	require.ErrorIs(t, err, errIntentional)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_rollback_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestWithTransaction_NilConnectionPool(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pg := &postgres.Postgres{}

	err := pg.WithTransaction(ctx, func(_ context.Context, _ pgx.Tx) error {
		return nil
	})

	require.Error(t, err)
	require.ErrorIs(t, err, postgres.ErrConnectionPoolNil)
}

func TestWithTransaction_MultipleOperations(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_multi_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for i := range 5 {
			_, err := tx.Exec(ctx, "INSERT INTO tx_multi_test (value) VALUES ($1)", fmt.Sprintf("value_%d", i))
			if err != nil {
				return err
			}
		}

		return nil
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_multi_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

func TestWithTransaction_PartialRollback(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_partial_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for i := range 3 {
			_, execErr := tx.Exec(ctx, "INSERT INTO tx_partial_test (value) VALUES ($1)", fmt.Sprintf("value_%d", i))
			if execErr != nil {
				return execErr
			}
		}

		return errAfterInserts
	})

	require.Error(t, err)
	require.ErrorIs(t, err, postgres.ErrTxRolledBack)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_partial_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestWithReadOnlyTransaction_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_readonly_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = pg.Exec(ctx, "INSERT INTO tx_readonly_test (value) VALUES ($1)", "existing_value")
	require.NoError(t, err)

	var value string

	err = pg.WithReadOnlyTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT value FROM tx_readonly_test WHERE id = 1").Scan(&value)
	})
	require.NoError(t, err)
	require.Equal(t, "existing_value", value)
}

func TestWithReadOnlyTransaction_WriteAttemptFails(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_readonly_write_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithReadOnlyTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_readonly_write_test (value) VALUES ($1)", "should_fail")

		return execErr
	})

	require.Error(t, err)
	require.ErrorIs(t, err, postgres.ErrTxRolledBack)
}

func TestWithSerializableTransaction_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_serializable_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithSerializableTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_serializable_test (value) VALUES ($1)", "serializable_value")

		return execErr
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_serializable_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestWithRepeatableReadTransaction_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_repeatable_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithRepeatableReadTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_repeatable_test (value) VALUES ($1)", "repeatable_value")

		return execErr
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_repeatable_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestWithTransactionOptions_CustomOptions(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_custom_opts_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	customOpts := pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	}

	err = pg.WithTransactionOptions(ctx, customOpts, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_custom_opts_test (value) VALUES ($1)", "custom_value")

		return execErr
	})
	require.NoError(t, err)

	var value string
	err = pg.QueryRow(ctx, "SELECT value FROM tx_custom_opts_test WHERE id = 1").Scan(&value)
	require.NoError(t, err)
	require.Equal(t, "custom_value", value)
}

func TestWithTransaction_NestedQueries(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_nested_test (
			id SERIAL PRIMARY KEY,
			parent_id INTEGER,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var parentID int

		queryErr := tx.QueryRow(ctx, "INSERT INTO tx_nested_test (value) VALUES ($1) RETURNING id", "parent").Scan(&parentID)
		if queryErr != nil {
			return queryErr
		}

		_, execErr := tx.Exec(ctx, "INSERT INTO tx_nested_test (parent_id, value) VALUES ($1, $2)", parentID, "child")

		return execErr
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM tx_nested_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	var childValue string
	err = pg.QueryRow(ctx, "SELECT value FROM tx_nested_test WHERE parent_id IS NOT NULL").Scan(&childValue)
	require.NoError(t, err)
	require.Equal(t, "child", childValue)
}

func TestWithTransaction_CancelledContext(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTransactionTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_cancelled_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = pg.WithTransaction(cancelCtx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO tx_cancelled_test (value) VALUES ($1)", "should_not_insert")

		return execErr
	})

	require.Error(t, err)
}
