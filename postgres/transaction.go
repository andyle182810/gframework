//nolint:varnamelen,err113,wsl,exhaustruct,perfsprint
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxFunc func(ctx context.Context, tx pgx.Tx) error

func (p *Postgres) WithTransaction(ctx context.Context, fn TxFunc) error {
	return p.WithTransactionOptions(ctx, pgx.TxOptions{}, fn)
}

func (p *Postgres) WithTransactionOptions(
	ctx context.Context,
	txOptions pgx.TxOptions,
	fn TxFunc,
) error {
	if p.DBPool == nil {
		return ErrConnectionPoolNil
	}

	pool, ok := p.DBPool.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("postgres: unable to cast DBPool to *pgxpool.Pool")
	}

	tx, err := pool.BeginTx(ctx, txOptions)
	if err != nil {
		return fmt.Errorf("postgres: failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("postgres: transaction error: %w, rollback error: %w", err, rbErr)
		}

		return fmt.Errorf("postgres: transaction rolled back: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: failed to commit transaction: %w", err)
	}

	return nil
}

func (p *Postgres) WithReadOnlyTransaction(ctx context.Context, fn TxFunc) error {
	return p.WithTransactionOptions(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadOnly,
	}, fn)
}

func (p *Postgres) WithSerializableTransaction(ctx context.Context, fn TxFunc) error {
	return p.WithTransactionOptions(ctx, pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}, fn)
}

func (p *Postgres) WithRepeatableReadTransaction(ctx context.Context, fn TxFunc) error {
	return p.WithTransactionOptions(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadWrite,
	}, fn)
}
