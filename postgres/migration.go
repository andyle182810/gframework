//nolint:revive,exhaustruct
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx_migrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

const (
	driverName        = "pgx"
	connectionTimeout = 10 * time.Second
)

type MigrationVersion struct {
	Version uint
	Dirty   bool
}

func openAndPingDB(dbURI string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dbURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection with driver '%s': %w", driverName, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func createMigrator(db *sql.DB, source string) (*migrate.Migrate, error) {
	driver, err := pgx_migrate.WithInstance(db, &pgx_migrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver instance: %w", err)
	}

	migrator, err := migrate.NewWithDatabaseInstance(source, driverName, driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator instance: %w", err)
	}

	return migrator, nil
}

func closeMigrator(migrator *migrate.Migrate) error {
	sourceErr, databaseErr := migrator.Close()
	if sourceErr != nil {
		return fmt.Errorf("migration cleanup error (source): %w", sourceErr)
	}

	if databaseErr != nil {
		return fmt.Errorf("migration cleanup error (database): %w", databaseErr)
	}

	return nil
}

func logMigrationResult(migrator *migrate.Migrate, err error, operation, source string) error {
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		log.Info().
			Str("operation", operation).
			Msg("The database migration completed with no changes to apply")
	case err != nil:
		log.Error().
			Err(err).
			Str("migration_source", source).
			Str("operation", operation).
			Msg("The database migration has failed")

		return fmt.Errorf("failed to run migration %s: %w", operation, err)
	default:
		version, dirty, _ := migrator.Version()
		log.Info().
			Uint("version", version).
			Bool("dirty", dirty).
			Str("operation", operation).
			Msg("The database migration has been completed successfully")
	}

	return nil
}

func MigrateUp(dbURI, source string) error {
	db, err := openAndPingDB(dbURI)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after migration attempt")
		}
	}()

	log.Info().
		Str("migration_source", source).
		Msg("The database migration is being started")

	migrator, err := createMigrator(db, source)
	if err != nil {
		return err
	}

	err = migrator.Up()
	if migErr := logMigrationResult(migrator, err, "up", source); migErr != nil {
		_ = closeMigrator(migrator)

		return migErr
	}

	return closeMigrator(migrator)
}

func MigrateDown(dbURI, source string) error {
	db, err := openAndPingDB(dbURI)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after migration attempt")
		}
	}()

	log.Info().
		Str("migration_source", source).
		Msg("The database migration rollback is being started")

	migrator, err := createMigrator(db, source)
	if err != nil {
		return err
	}

	err = migrator.Down()
	if migErr := logMigrationResult(migrator, err, "down", source); migErr != nil {
		_ = closeMigrator(migrator)

		return migErr
	}

	return closeMigrator(migrator)
}

func MigrateSteps(dbURI, source string, steps int) error {
	db, err := openAndPingDB(dbURI)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after migration attempt")
		}
	}()

	log.Info().
		Str("migration_source", source).
		Int("steps", steps).
		Msg("The database migration steps are being applied")

	migrator, err := createMigrator(db, source)
	if err != nil {
		return err
	}

	err = migrator.Steps(steps)
	if migErr := logMigrationResult(migrator, err, "steps", source); migErr != nil {
		_ = closeMigrator(migrator)

		return migErr
	}

	return closeMigrator(migrator)
}

func GetMigrationVersion(dbURI, source string) (*MigrationVersion, error) {
	db, err := openAndPingDB(dbURI)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after version check")
		}
	}()

	migrator, err := createMigrator(db, source)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = closeMigrator(migrator)
	}()

	version, dirty, err := migrator.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return nil, fmt.Errorf("failed to get migration version: %w", err)
	}

	return &MigrationVersion{
		Version: version,
		Dirty:   dirty,
	}, nil
}

func RunMigration(dbURI, source string) error {
	log.Info().Str("source", source).Msg("Starting database migration process...")

	logPreMigrationState(dbURI, source)

	if err := MigrateUp(dbURI, source); err != nil {
		return err
	}

	logPostMigrationState(dbURI, source)

	return nil
}

func logPreMigrationState(dbURI, source string) {
	currentVersion, err := GetMigrationVersion(dbURI, source)
	if err != nil {
		return
	}

	log.Info().
		Uint("current_version", currentVersion.Version).
		Bool("dirty", currentVersion.Dirty).
		Msg("Pre-migration state")

	if currentVersion.Dirty {
		log.Warn().Msg("Database is in dirty state from previous failed migration")
	}
}

func logPostMigrationState(dbURI, source string) {
	finalVersion, err := GetMigrationVersion(dbURI, source)
	if err != nil {
		log.Info().Msg("Database migration process completed successfully")

		return
	}

	log.Info().
		Uint("version", finalVersion.Version).
		Msg("Database migration process completed successfully")
}

func ForceMigrationVersion(dbURI, source string, version int) error {
	db, err := openAndPingDB(dbURI)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().
				Err(closeErr).
				Msg("The database connection failed to close after force version")
		}
	}()

	log.Warn().
		Str("migration_source", source).
		Int("version", version).
		Msg("Forcing migration version - use with caution")

	migrator, err := createMigrator(db, source)
	if err != nil {
		return err
	}

	if err = migrator.Force(version); err != nil {
		_ = closeMigrator(migrator)

		return fmt.Errorf("failed to force migration version: %w", err)
	}

	log.Info().
		Int("version", version).
		Msg("Migration version has been forced successfully")

	return closeMigrator(migrator)
}
