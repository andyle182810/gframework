package service

import (
	"context"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/valkey"
	goredis "github.com/redis/go-redis/v9"
)

type valkeyClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *goredis.StatusCmd
}

type Service struct {
	repo   *repo.Repository
	db     *postgres.Postgres
	valkey *valkey.Valkey
}

func New(repo *repo.Repository, db *postgres.Postgres, valkey *valkey.Valkey) *Service {
	return &Service{
		repo:   repo,
		db:     db,
		valkey: valkey,
	}
}
