package main

import (
	"fmt"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/config"
	"github.com/andyle182810/gframework/examples/demo-api/internal/executor/consumer"
	"github.com/andyle182810/gframework/examples/demo-api/internal/executor/worker"
	"github.com/andyle182810/gframework/examples/demo-api/internal/handler"
	"github.com/andyle182810/gframework/examples/demo-api/internal/publisher"
	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/logutil"
	"github.com/andyle182810/gframework/metricserver"
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/redispub"
	"github.com/andyle182810/gframework/redissub"
	"github.com/andyle182810/gframework/runner"
	"github.com/andyle182810/gframework/taskqueue"
	"github.com/andyle182810/gframework/valkey"
	"github.com/andyle182810/gframework/workerpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// @title			Demo API
// @version		1.0
// @description	A demo API built with gframework showcasing users CRUD, task queues, and message publishing.
//
// @host		localhost:8080
// @BasePath	/
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

	valkey, err := initValkey(cfg)
	if err != nil {
		return err
	}

	repository := repo.New(db)

	svc := service.New(repository, db, valkey)

	taskQueue, err := initTaskQueue(valkey)
	if err != nil {
		return err
	}

	publisher, err := initValkeyPublisher(valkey)
	if err != nil {
		return err
	}

	multiSubscriber, err := initMultiSubscriber(valkey)
	if err != nil {
		return err
	}

	app := &application{
		cfg:       cfg,
		svc:       svc,
		db:        db,
		valkey:    valkey,
		taskQueue: taskQueue,
		publisher: publisher,
	}

	appRunner := runner.New(
		runner.WithInfrastructureService(db),
		runner.WithInfrastructureService(valkey),
		runner.WithCoreService(app.newMetricServer()),
		runner.WithCoreService(app.newHTTPServer()),
		runner.WithCoreService(app.newTaskProducerPool()),
		runner.WithCoreService(taskQueue),
		runner.WithCoreService(app.newMessagePublisherPool()),
		runner.WithCoreService(app.newTaskRecoveryPool()),
		runner.WithCoreService(multiSubscriber),
	)

	appRunner.Run()

	return nil
}

type application struct {
	cfg       *config.Config
	svc       *service.Service
	db        *postgres.Postgres
	valkey    *valkey.Valkey
	taskQueue *taskqueue.Queue
	publisher *redispub.RedisPublisher
}

func (app *application) newHTTPServer() *httpserver.Server {
	httpCfg := &httpserver.Config{
		Host:         app.cfg.HTTPServerHost,
		Port:         app.cfg.HTTPServerPort,
		EnableCors:   app.cfg.HTTPEnableCORS,
		AllowOrigins: app.cfg.HTTPAllowOrigins,
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

func (app *application) newTaskProducerPool() *workerpool.WorkerPool {
	producer := worker.NewTaskProducer(app.taskQueue)

	return workerpool.New(
		producer,
		workerpool.WithName("task-producer-pool"),
		workerpool.WithWorkerCount(2),                   //nolint:mnd
		workerpool.WithTickInterval(5*time.Second),      //nolint:mnd
		workerpool.WithExecutionTimeout(10*time.Second), //nolint:mnd
	)
}

func (app *application) newMessagePublisherPool() *workerpool.WorkerPool {
	orderPub := publisher.NewOrderPublisher(app.publisher, "demo-api:orders")
	notificationPub := publisher.NewNotificationPublisher(app.publisher, "demo-api:notifications")
	analyticsPub := publisher.NewAnalyticsPublisher(app.publisher, "demo-api:analytics")

	msgPublisher := worker.NewMessagePublisher(&worker.MessagePublisherConfig{
		OrderPublisher:        orderPub,
		NotificationPublisher: notificationPub,
		AnalyticsPublisher:    analyticsPub,
	})

	return workerpool.New(
		msgPublisher,
		workerpool.WithName("message-publisher-pool"),
		workerpool.WithWorkerCount(1),
		workerpool.WithTickInterval(10*time.Second),     //nolint:mnd
		workerpool.WithExecutionTimeout(30*time.Second), //nolint:mnd
	)
}

func (app *application) newTaskRecoveryPool() *workerpool.WorkerPool {
	recoveryExecutor := worker.NewTaskRecovery(app.taskQueue)

	return workerpool.New(
		recoveryExecutor,
		workerpool.WithName("task-recovery-pool"),
		workerpool.WithWorkerCount(1),
		workerpool.WithTickInterval(time.Minute),
		workerpool.WithExecutionTimeout(10*time.Second), //nolint:mnd
	)
}

func (app *application) registerRoutes(_ *echo.Echo, root *echo.Group) {
	root.GET("/health", httpserver.Wrapper(app.svc.CheckHealth))
	root.GET("/ready", httpserver.Wrapper(app.svc.CheckReadiness))

	v1 := root.Group("/v1")
	v1.Use(middleware.RequestID(httpserver.RequestIDSkipper(app.cfg.HTTPSkipRequestID)))

	v1.POST("/users", httpserver.Wrapper(app.svc.CreateUser))
	v1.GET("/users/:userId", httpserver.Wrapper(app.svc.GetUser))
	v1.PATCH("/users/:userId", httpserver.Wrapper(app.svc.UpdateUser))
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
		if err := postgres.RunMigration(cfg.PostgresDSN(), cfg.MigrationSource); err != nil {
			return nil, err
		}
	}

	db, err := postgres.New(pgCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres: %w", err)
	}

	log.Info().Msg("PostgreSQL client initialized successfully")

	return db, nil
}

func initValkey(cfg *config.Config) (*valkey.Valkey, error) {
	redisCfg := &valkey.Config{
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

	redis, err := valkey.New(redisCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize redis: %w", err)
	}

	log.Info().Msg("Redis client initialized successfully")

	return redis, nil
}

func initTaskQueue(valkeyClient *valkey.Valkey) (*taskqueue.Queue, error) {
	taskConsumer := consumer.New()

	queue, err := taskqueue.New(
		valkeyClient.Client,
		"demo-api:tasks",
		taskConsumer,
		taskqueue.WithWorkerCount(3),              //nolint:mnd
		taskqueue.WithExecTimeout(10*time.Second), //nolint:mnd
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task queue: %w", err)
	}

	log.Info().Msg("Task queue initialized successfully")

	return queue, nil
}

func initValkeyPublisher(valkeyClient *valkey.Valkey) (*redispub.RedisPublisher, error) {
	publisher, err := redispub.New(valkeyClient.Client, redispub.Options{
		MaxStreamEntries: 1000,            //nolint:mnd
		Timeout:          5 * time.Second, //nolint:mnd
		Logger:           nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize redis publisher: %w", err)
	}

	log.Info().Msg("Valkey publisher initialized successfully")

	return publisher, nil
}

func initMultiSubscriber(valkeyClient *valkey.Valkey) (*redissub.MultiSubscriber, error) {
	multiSub := redissub.NewMultiSubscriber(
		"demo-api-subscriber",
		valkeyClient.Client,
		"demo-api-consumer-group",
		redissub.WithExecTimeout(30*time.Second),                    //nolint:mnd
		redissub.WithShutdownTimeout(10*time.Second),                //nolint:mnd
		redissub.WithRetry(3, 100*time.Millisecond, "demo-api:dlq"), //nolint:mnd
	)

	handler := handler.New()

	// Subscribe to message topics
	topics := map[string]redissub.MessageHandler{
		"demo-api:orders":        handler.HandleOrder,
		"demo-api:notifications": handler.HandleNotification,
		"demo-api:analytics":     handler.HandleAnalytics,
	}

	for topic, topicHandler := range topics {
		if err := multiSub.Subscribe(topic, topicHandler); err != nil {
			return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
		}
	}

	log.Info().
		Int("subscriber_count", multiSub.SubscriberCount()).
		Msg("Multi subscriber initialized successfully")

	return multiSub, nil
}
