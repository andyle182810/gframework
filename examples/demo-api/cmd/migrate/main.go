package main

import (
	"os"
	"strconv"

	"github.com/andyle182810/gframework/examples/demo-api/internal/config"
	"github.com/andyle182810/gframework/postgres"
	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	runCommand(args, cfg)
}

func runCommand(args []string, cfg *config.Config) {
	dsn := cfg.PostgresDSN()
	source := cfg.MigrationSource

	switch args[0] {
	case "version":
		handleVersion(dsn, source)
	case "up":
		handleUp(dsn, source)
	case "down":
		handleDown(args[1:], dsn, source)
	case "force":
		handleForce(args[1:], dsn, source)
	default:
		log.Error().Str("command", args[0]).Msg("Unknown command")
		printUsage()
		os.Exit(1)
	}
}

func handleVersion(dsn, source string) {
	v, err := postgres.GetMigrationVersion(dsn, source)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get migration version")
	}

	log.Info().Uint("version", v.Version).Bool("dirty", v.Dirty).Msg("Current migration version")
}

func handleUp(dsn, source string) {
	if err := postgres.MigrateUp(dsn, source); err != nil {
		log.Fatal().Err(err).Msg("Migration up failed")
	}
}

func handleDown(args []string, dsn, source string) {
	if len(args) == 0 {
		log.Fatal().Msg("Usage: migrate down <N>")
	}

	steps, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatal().Err(err).Str("value", args[0]).Msg("Invalid step count")
	}

	if err := postgres.MigrateSteps(dsn, source, -steps); err != nil {
		log.Fatal().Err(err).Int("steps", steps).Msg("Migration down failed")
	}
}

func handleForce(args []string, dsn, source string) {
	if len(args) == 0 {
		log.Fatal().Msg("Usage: migrate force <version>")
	}

	version, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatal().Err(err).Str("value", args[0]).Msg("Invalid version")
	}

	if err := postgres.ForceMigrationVersion(dsn, source, version); err != nil {
		log.Fatal().Err(err).Int("version", version).Msg("Force migration version failed")
	}
}

func printUsage() {
	log.Info().Msg("Usage: migrate <command>")
	log.Info().Msg("Commands:")
	log.Info().Msg("  version          Show current migration version")
	log.Info().Msg("  up               Run all pending up migrations")
	log.Info().Msg("  down <N>         Roll back N migrations")
	log.Info().Msg("  force <version>  Force set migration version (for dirty state recovery)")
}
