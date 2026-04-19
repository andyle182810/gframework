package postgres

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBPool interface {
	pgxscan.Querier
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Ping(ctx context.Context) error
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Close()
}

type DB interface {
	DBPool
	Name() string
	Start(ctx context.Context) error
	Stop() error
	IsHealthy(ctx context.Context) bool
	HealthCheck(ctx context.Context, opts ...HealthCheckOption) error
	GetPoolStats() (*PoolStats, error)
	BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]any) (int64, error)
	WithTransaction(ctx context.Context, fn TxFunc) error
	WithTransactionOptions(ctx context.Context, txOptions pgx.TxOptions, fn TxFunc) error
	WithReadOnlyTransaction(ctx context.Context, fn TxFunc) error
	WithSerializableTransaction(ctx context.Context, fn TxFunc) error
	WithRepeatableReadTransaction(ctx context.Context, fn TxFunc) error
	WithRetryTx(ctx context.Context, config RetryConfig, fn TxFunc) error
	WithRetryTxDefault(ctx context.Context, fn TxFunc) error
}
