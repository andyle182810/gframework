package repo

import (
	"context"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
)

type User struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type UserRepo struct {
	pool postgres.DBPool
}

func NewUserRepo(pool postgres.DBPool) *UserRepo {
	return &UserRepo{
		pool: pool,
	}
}

func (r *UserRepo) CreateUser(ctx context.Context, name, email string) (*User, error) {
	user := &User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
		INSERT INTO users (id, name, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, email, created_at, updated_at
	`

	err := pgxscan.Get(ctx, r.pool, user, query, user.ID, user.Name, user.Email, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, name, email, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	var user User

	err := pgxscan.Get(ctx, r.pool, &user, query, id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, name, email, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	var user User

	err := pgxscan.Get(ctx, r.pool, &user, query, email)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, name, email, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var users []*User

	err := pgxscan.Select(ctx, r.pool, &users, query, limit, offset)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepo) CountUsers(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`

	var count int

	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, id, name, email string) (*User, error) {
	query := `
		UPDATE users
		SET name = $1, email = $2, updated_at = $3
		WHERE id = $4
		RETURNING id, name, email, created_at, updated_at
	`

	var user User

	err := pgxscan.Get(ctx, r.pool, &user, query, name, email, time.Now(), id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)

	return err
}
