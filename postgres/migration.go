//nolint:revive,exhaustruct
package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx_migrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

const (
	DriverName        = "postgres"
	ConnectionTimeout = 10 * time.Second
)

//nolint:cyclop
func MigrateUp(dbURI, source string) error {
	db, err := sql.Open(DriverName, dbURI)
	if err != nil {
		return fmt.Errorf("failed to open database connection with driver '%s': %w", DriverName, err)
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after migration attempt")
		}
	}()

	db.SetConnMaxLifetime(ConnectionTimeout)

	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database at %s: %w", dbURI, err)
	}

	log.Info().
		Str("migration_source", source).
		Msg("The database migration is being started")

	driver, err := pgx_migrate.WithInstance(db, &pgx_migrate.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver instance: %w", err)
	}

	migrator, err := migrate.NewWithDatabaseInstance(source, DriverName, driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}

	err = migrator.Up()

	switch {
	case errors.Is(err, migrate.ErrNoChange):
		log.Info().
			Msg("The database migration has been completed with no new migrations to apply")
	case err != nil:
		log.Error().
			Err(err).
			Str("migration_source", source).
			Msg("The database migration has failed")

		return fmt.Errorf("failed to run migrations up: %w", err)
	default:
		log.Info().
			Msg("The database migration has been completed successfully")
	}

	sourceErr, databaseErr := migrator.Close()
	if sourceErr != nil {
		return fmt.Errorf("migration cleanup error (source): %w", sourceErr)
	}

	if databaseErr != nil {
		return fmt.Errorf("migration cleanup error (database): %w", databaseErr)
	}

	return nil
}
