package service

import (
	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/goredis"
	"github.com/andyle182810/gframework/postgres"
)

type Service struct {
	repo  *repo.Repository
	db    *postgres.Postgres
	redis *goredis.Redis
}

func New(repo *repo.Repository, db *postgres.Postgres, redis *goredis.Redis) *Service {
	return &Service{
		repo:  repo,
		db:    db,
		redis: redis,
	}
}
