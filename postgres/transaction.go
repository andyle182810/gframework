//nolint:varnamelen,wsl,exhaustruct
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDBPoolCastFailed = errors.New("postgres: unable to cast DBPool to *pgxpool.Pool")
	ErrBeginTxFailed    = errors.New("postgres: failed to begin transaction")
	ErrTxRollbackFailed = errors.New("postgres: transaction rollback failed")
	ErrTxRolledBack     = errors.New("postgres: transaction rolled back")
	ErrTxCommitFailed   = errors.New("postgres: failed to commit transaction")
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
		return ErrDBPoolCastFailed
	}

	tx, err := pool.BeginTx(ctx, txOptions)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBeginTxFailed, err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("%w: %w, %w: %w", ErrTxRolledBack, err, ErrTxRollbackFailed, rbErr)
		}

		return fmt.Errorf("%w: %w", ErrTxRolledBack, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrTxCommitFailed, err)
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
