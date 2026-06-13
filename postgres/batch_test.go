//nolint:exhaustruct
package postgres_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testUser struct {
	ID    int
	Name  string
	Email string
	Age   int
}

type testProduct struct {
	ID          int
	Name        string
	Price       float64
	Description string
}

const (
	userAlice   = "Alice"
	userBob     = "Bob"
	columnID    = "id"
	columnName  = "name"
	columnAge   = "age"
	columnEmail = "email"
)

func setupTestPostgres(t *testing.T) *postgres.Postgres {
	t.Helper()

	container := testutil.SetupPostgresContainer(t)

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

	return pg
}

func TestBulkInsert_Success(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	columns := []string{columnName, columnEmail, columnAge}
	rows := [][]any{
		{userAlice, "alice@example.com", 25},
		{userBob, "bob@example.com", 30},
		{"Charlie", "charlie@example.com", 35},
		{"Diana", "diana@example.com", 28},
		{"Eve", "eve@example.com", 32},
	}

	count, err := pg.BulkInsert(ctx, "test_users", columns, rows)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	var actualCount int64
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM test_users").Scan(&actualCount)
	require.NoError(t, err)
	assert.Equal(t, int64(5), actualCount)

	var name, email string

	var age int

	err = pg.QueryRow(ctx, "SELECT name, email, age FROM test_users WHERE name = $1", userAlice).
		Scan(&name, &email, &age)
	require.NoError(t, err)
	assert.Equal(t, userAlice, name)
	assert.Equal(t, "alice@example.com", email)
	assert.Equal(t, 25, age)
}

func TestBulkInsert_EmptyRows(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_empty (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	columns := []string{columnName}
	rows := [][]any{}

	count, err := pg.BulkInsert(ctx, "test_empty", columns, rows)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestBulkInsert_NilConnectionPool(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := &postgres.Postgres{}

	columns := []string{columnName}
	rows := [][]any{{"test"}}

	count, err := pg.BulkInsert(ctx, "test_table", columns, rows)
	require.Error(t, err)
	assert.Equal(t, int64(0), count)
	assert.ErrorIs(t, err, postgres.ErrConnectionPoolNil)
}

func TestBulkInsert_InvalidTable(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	columns := []string{columnName}
	rows := [][]any{{"test"}}

	count, err := pg.BulkInsert(ctx, "nonexistent_table", columns, rows)
	require.Error(t, err)
	assert.Equal(t, int64(0), count)
	assert.Contains(t, err.Error(), "postgres bulk insert failed")
}

func TestBulkInsert_MismatchedColumns(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_mismatch (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			age INTEGER NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	columns := []string{columnName, columnAge}
	rows := [][]any{
		{userAlice, 25, "extra_value"},
	}

	count, err := pg.BulkInsert(ctx, "test_mismatch", columns, rows)
	require.Error(t, err)
	assert.Equal(t, int64(0), count)
}

func TestBulkInsert_LargeDataset(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_large (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value INTEGER NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	columns := []string{columnName, "value"}

	rows := make([][]any, 1000)
	for i := range 1000 {
		rows[i] = []any{fmt.Sprintf("name_%d", i), i}
	}

	count, err := pg.BulkInsert(ctx, "test_large", columns, rows)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), count)

	var actualCount int64
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM test_large").Scan(&actualCount)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), actualCount)
}

func TestBulkInsertStructs_Success(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_struct_users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	users := []testUser{
		{ID: 1, Name: userAlice, Email: "alice@example.com", Age: 25},
		{ID: 2, Name: userBob, Email: "bob@example.com", Age: 30},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com", Age: 35},
	}

	columns := []string{columnID, columnName, columnEmail, columnAge}

	valueExtractor := func(u testUser) []any {
		return []any{u.ID, u.Name, u.Email, u.Age}
	}

	count, err := postgres.BulkInsertStructs(ctx, pg, "test_struct_users", columns, users, valueExtractor)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	var actualCount int64
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM test_struct_users").Scan(&actualCount)
	require.NoError(t, err)
	assert.Equal(t, int64(3), actualCount)

	var name, email string

	var age int

	err = pg.QueryRow(ctx, "SELECT name, email, age FROM test_struct_users WHERE id = $1", 1).
		Scan(&name, &email, &age)
	require.NoError(t, err)
	assert.Equal(t, userAlice, name)
	assert.Equal(t, "alice@example.com", email)
	assert.Equal(t, 25, age)
}

func TestBulkInsertStructs_EmptySlice(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_empty_structs (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	users := []testUser{}
	columns := []string{columnID, columnName}

	valueExtractor := func(u testUser) []any {
		return []any{u.ID, u.Name}
	}

	count, err := postgres.BulkInsertStructs(ctx, pg, "test_empty_structs", columns, users, valueExtractor)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestBulkInsertStructs_DifferentTypes(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_products (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			price NUMERIC(10, 2) NOT NULL,
			description TEXT
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	products := []testProduct{
		{ID: 1, Name: "Laptop", Price: 999.99, Description: "High-performance laptop"},
		{ID: 2, Name: "Mouse", Price: 29.99, Description: "Wireless mouse"},
		{ID: 3, Name: "Keyboard", Price: 79.99, Description: "Mechanical keyboard"},
	}

	columns := []string{columnID, columnName, "price", "description"}

	valueExtractor := func(p testProduct) []any {
		return []any{p.ID, p.Name, p.Price, p.Description}
	}

	count, err := postgres.BulkInsertStructs(ctx, pg, "test_products", columns, products, valueExtractor)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	var name, description string

	var price float64

	err = pg.QueryRow(ctx, "SELECT name, price, description FROM test_products WHERE id = $1", 1).
		Scan(&name, &price, &description)
	require.NoError(t, err)
	assert.Equal(t, "Laptop", name)
	assert.InDelta(t, 999.99, price, 0.01)
	assert.Equal(t, "High-performance laptop", description)
}

func TestBulkInsertStructs_WithPartialColumns(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_partial (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT DEFAULT 'no-email@example.com',
			age INTEGER DEFAULT 0
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	type partialUser struct {
		Name string
	}

	users := []partialUser{
		{Name: userAlice},
		{Name: userBob},
	}

	columns := []string{columnName}

	valueExtractor := func(u partialUser) []any {
		return []any{u.Name}
	}

	count, err := postgres.BulkInsertStructs(ctx, pg, "test_partial", columns, users, valueExtractor)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	var name, email string

	var age int

	err = pg.QueryRow(ctx, "SELECT name, email, age FROM test_partial WHERE name = $1", userAlice).
		Scan(&name, &email, &age)
	require.NoError(t, err)
	assert.Equal(t, userAlice, name)
	assert.Equal(t, "no-email@example.com", email)
	assert.Equal(t, 0, age)
}

func TestBulkInsertStructs_LargeDataset(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	pg := setupTestPostgres(t)

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_large_structs (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL
		)
	`
	_, err := pg.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	users := make([]testUser, 5000)
	for i := range 5000 {
		users[i] = testUser{
			ID:    i + 1,
			Name:  fmt.Sprintf("user_%d", i),
			Email: fmt.Sprintf("user_%d@example.com", i),
			Age:   20 + (i % 50),
		}
	}

	columns := []string{columnID, columnName, columnEmail, columnAge}

	valueExtractor := func(u testUser) []any {
		return []any{u.ID, u.Name, u.Email, u.Age}
	}

	count, err := postgres.BulkInsertStructs(ctx, pg, "test_large_structs", columns, users, valueExtractor)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), count)

	var actualCount int64
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM test_large_structs").Scan(&actualCount)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), actualCount)
}
