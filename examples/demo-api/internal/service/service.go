package service

import (
	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/valkey"
)

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
