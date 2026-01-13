//nolint:varnamelen
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) BulkInsert(
	ctx context.Context,
	tableName string,
	columns []string,
	rows [][]any,
) (int64, error) {
	if p.DBPool == nil {
		return 0, ErrConnectionPoolNil
	}

	if len(rows) == 0 {
		return 0, nil
	}

	copySource := pgx.CopyFromRows(rows)

	count, err := p.DBPool.CopyFrom(
		ctx,
		pgx.Identifier{tableName},
		columns,
		copySource,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres bulk insert failed: %w", err)
	}

	return count, nil
}

func BulkInsertStructs[T any](
	ctx context.Context,
	p *Postgres,
	tableName string,
	columns []string,
	structs []T,
	valueExtractor func(T) []any,
) (int64, error) {
	if len(structs) == 0 {
		return 0, nil
	}

	rows := make([][]any, len(structs))
	for idx, item := range structs {
		rows[idx] = valueExtractor(item)
	}

	return p.BulkInsert(ctx, tableName, columns, rows)
}
