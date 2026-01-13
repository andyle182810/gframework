package repo

import (
	"github.com/andyle182810/gframework/postgres"
)

type Repository struct {
	User *UserRepo
}

func New(db *postgres.Postgres) *Repository {
	return &Repository{
		User: NewUserRepo(db.DBPool),
	}
}
