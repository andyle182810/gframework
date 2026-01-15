package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/config"
	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/goredis"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/logutil"
	"github.com/andyle182810/gframework/metricserver"
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/runner"
	"github.com/andyle182810/gframework/workerpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("Application exited with an error")
	}

	log.Info().Msg("Application shutdown complete")
}

func run() error {
	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	zerolog.SetGlobalLevel(logutil.ParseZerologLevel(cfg.LogLevel))

	db, err := initPostgres(cfg)
	if err != nil {
		return err
	}

	redis, err := initRedis(cfg)
	if err != nil {
		return err
	}

	repository := repo.New(db)

	svc := service.New(repository, db, redis)

	app := &application{
		cfg:   cfg,
		svc:   svc,
		db:    db,
		redis: redis,
	}

	appRunner := runner.New(
		runner.WithInfrastructureService(db),
		runner.WithInfrastructureService(redis),
		runner.WithCoreService(app.newMetricServer()),
		runner.WithCoreService(app.newHTTPServer()),
		runner.WithCoreService(app.newWorkerPool()),
	)

	appRunner.Run()

	return nil
}

type application struct {
	cfg   *config.Config
	svc   *service.Service
	db    *postgres.Postgres
	redis *goredis.Redis
}

func (app *application) newHTTPServer() *httpserver.Server {
	httpCfg := &httpserver.Config{
		Host:         app.cfg.HTTPServerHost,
		Port:         app.cfg.HTTPServerPort,
		EnableCors:   app.cfg.HTTPEnableCORS,
		BodyLimit:    app.cfg.HTTPBodyLimit,
		ReadTimeout:  app.cfg.HTTPServerReadTimeout,
		WriteTimeout: app.cfg.HTTPServerWriteTimeout,
		GracePeriod:  app.cfg.GracefulShutdownPeriod,
	}

	svr := httpserver.New(httpCfg)
	app.registerRoutes(svr.Echo, svr.Root)

	return svr
}

func (app *application) newMetricServer() *metricserver.Server {
	metricCfg := &metricserver.Config{
		Host:         app.cfg.MetricServerHost,
		Port:         app.cfg.MetricServerPort,
		ReadTimeout:  app.cfg.MetricServerReadTimeout,
		WriteTimeout: app.cfg.MetricServerWriteTimeout,
		GracePeriod:  app.cfg.GracefulShutdownPeriod,
	}

	return metricserver.New(metricCfg)
}

func (app *application) newWorkerPool() *workerpool.WorkerPool {
	executor := &dummyExecutor{}

	return workerpool.New(
		executor,
		workerpool.WithWorkerCount(2),
		workerpool.WithTickInterval(5*time.Second),
		workerpool.WithExecutionTimeout(10*time.Second),
	)
}

type dummyExecutor struct{}

func (e *dummyExecutor) Execute(ctx context.Context) error {
	sleepDuration := time.Duration(rand.IntN(10)+1) * time.Second
	log.Info().Dur("sleep_duration", sleepDuration).Msg("Dummy executor running...")

	select {
	case <-time.After(sleepDuration):
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Info().Msg("Dummy executor done")

	return nil
}

func (app *application) registerRoutes(_ *echo.Echo, root *echo.Group) {
	root.GET("/health", httpserver.Wrapper(app.svc.CheckHealth))
	root.GET("/ready", httpserver.Wrapper(app.svc.CheckReadiness))

	v1 := root.Group("/v1")
	v1.Use(middleware.RequestID(httpserver.RequestIDSkipper(app.cfg.HTTPSkipRequestID)))

	v1.POST("/users", httpserver.Wrapper(app.svc.CreateUser))
	v1.GET("/users/:userId", httpserver.Wrapper(app.svc.GetUser))
	v1.GET("/users", httpserver.Wrapper(app.svc.ListUsers))
}

func initPostgres(cfg *config.Config) (*postgres.Postgres, error) {
	maxConn := cfg.PostgresMaxConnection
	minConn := cfg.PostgresMinConnection

	pgCfg := &postgres.Config{
		URL:                      cfg.PostgresDSN(),
		MaxConnection:            int32(maxConn), //nolint:gosec
		MinConnection:            int32(minConn), //nolint:gosec
		MaxConnectionIdleTime:    cfg.PostgresMaxConnectionIdleTime,
		MaxConnectionLifetime:    cfg.PostgresMaxConnectionLifetime,
		HealthCheckPeriod:        cfg.PostgresHealthCheckPeriod,
		ConnectTimeout:           cfg.PostgresConnectTimeout,
		StatementTimeout:         cfg.PostgresStatementTimeout,
		LockTimeout:              cfg.PostgresLockTimeout,
		IdleInTransactionTimeout: cfg.PostgresIdleInTransactionTimeout,
		LogLevel:                 logutil.ParsePostgresLogLevel(cfg.PostgresLogLevel),
	}

	if cfg.MigrationEnabled {
		log.Info().Str("source", cfg.MigrationSource).Msg("Starting database migration process...")

		if err := postgres.MigrateUp(cfg.PostgresDSN(), cfg.MigrationSource); err != nil {
			return nil, fmt.Errorf("postgresql migration failed: %w", err)
		}

		log.Info().Msg("Database migration process completed successfully")
	}

	db, err := postgres.New(pgCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres: %w", err)
	}

	log.Info().Msg("PostgreSQL client initialized successfully")

	return db, nil
}

func initRedis(cfg *config.Config) (*goredis.Redis, error) {
	redisCfg := &goredis.Config{
		Host:            cfg.ValkeyHost,
		Port:            cfg.ValkeyPort,
		Password:        cfg.ValkeyPassword,
		DB:              cfg.ValkeyDB,
		DialTimeout:     cfg.ValkeyDialTimeout,
		ReadTimeout:     cfg.ValkeyReadTimeout,
		WriteTimeout:    cfg.ValkeyWriteTimeout,
		PoolSize:        cfg.ValkeyPoolSize,
		MinIdleConns:    cfg.ValkeyMinIdleConns,
		MaxIdleConns:    cfg.ValkeyMaxIdleConns,
		PingTimeout:     cfg.ValkeyPingTimeout,
		TLSEnabled:      false,
		MaxRetries:      0,
		MinRetryBackoff: 0,
		MaxRetryBackoff: 0,
		TLSSkipVerify:   false,
		TLSCertFile:     "",
		TLSKeyFile:      "",
		TLSCAFile:       "",
	}

	redis, err := goredis.New(redisCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize redis: %w", err)
	}

	log.Info().Msg("Redis client initialized successfully")

	return redis, nil
}
