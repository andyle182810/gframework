package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultSlowQueryThreshold = time.Second
	metricsNamespace          = "gframework"
	metricsSubsystem          = "postgres"
	labelOperation            = "operation"
	labelResult               = "result"
	operationUnknown          = "unknown"
	operationCopyFrom         = "copy_from"
)

type queryMetricsContextKey struct{}

type queryMetricsState struct {
	operation string
	startedAt time.Time
}

type metricsTracer struct {
	slowQueryThreshold time.Duration
	metrics            *queryMetrics
}

type queryMetrics struct {
	queriesTotal     *prometheus.CounterVec
	queryDuration    *prometheus.HistogramVec
	slowQueriesTotal *prometheus.CounterVec
}

func newQueryMetrics() *queryMetrics {
	return &queryMetrics{
		queriesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"queries_total",
				"Total PostgreSQL queries by operation and result.",
			),
			[]string{labelOperation, labelResult},
		),
		queryDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"query_duration_seconds",
				"PostgreSQL query duration by operation and result.",
				[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			),
			[]string{labelOperation, labelResult},
		),
		slowQueriesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"slow_queries_total",
				"Total PostgreSQL queries slower than the configured slow query threshold.",
			),
			[]string{labelOperation},
		),
	}
}

func newMetricsTracer(slowQueryThreshold time.Duration) *metricsTracer {
	if slowQueryThreshold == 0 {
		slowQueryThreshold = defaultSlowQueryThreshold
	}

	return &metricsTracer{
		slowQueryThreshold: slowQueryThreshold,
		metrics:            newQueryMetrics(),
	}
}

func (t *metricsTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	if !frameworkmetrics.Enabled() {
		return ctx
	}

	return context.WithValue(ctx, queryMetricsContextKey{}, queryMetricsState{
		operation: sqlOperation(data.SQL),
		startedAt: time.Now(),
	})
}

func (t *metricsTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if !frameworkmetrics.Enabled() {
		return
	}

	state, ok := ctx.Value(queryMetricsContextKey{}).(queryMetricsState)
	if !ok {
		state = queryMetricsState{
			operation: operationUnknown,
			startedAt: time.Now(),
		}
	}

	t.recordQuery(state.operation, time.Since(state.startedAt), data.Err)
}

func (t *metricsTracer) TraceCopyFromStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TraceCopyFromStartData,
) context.Context {
	if !frameworkmetrics.Enabled() {
		return ctx
	}

	return context.WithValue(ctx, queryMetricsContextKey{}, queryMetricsState{
		operation: operationCopyFrom,
		startedAt: time.Now(),
	})
}

func (t *metricsTracer) TraceCopyFromEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceCopyFromEndData) {
	if !frameworkmetrics.Enabled() {
		return
	}

	state, ok := ctx.Value(queryMetricsContextKey{}).(queryMetricsState)
	if !ok {
		state = queryMetricsState{
			operation: operationCopyFrom,
			startedAt: time.Now(),
		}
	}

	t.recordQuery(state.operation, time.Since(state.startedAt), data.Err)
}

func (t *metricsTracer) recordQuery(operation string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	result := queryResult(err)

	t.metrics.queriesTotal.WithLabelValues(operation, result).Inc()
	t.metrics.queryDuration.WithLabelValues(operation, result).Observe(duration.Seconds())

	if t.slowQueryThreshold > 0 && duration >= t.slowQueryThreshold {
		t.metrics.slowQueriesTotal.WithLabelValues(operation).Inc()
	}
}

func queryResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func sqlOperation(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return operationUnknown
	}

	for {
		if strings.HasPrefix(sql, "/*") {
			end := strings.Index(sql, "*/")
			if end == -1 {
				return operationUnknown
			}

			sql = strings.TrimSpace(sql[end+2:])

			continue
		}

		if strings.HasPrefix(sql, "--") {
			end := strings.IndexByte(sql, '\n')
			if end == -1 {
				return operationUnknown
			}

			sql = strings.TrimSpace(sql[end+1:])

			continue
		}

		break
	}

	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return operationUnknown
	}

	return strings.ToLower(fields[0])
}

type poolMetricsCollector struct {
	pools sync.Map

	connectionsDesc           *prometheus.Desc
	acquiresTotalDesc         *prometheus.Desc
	acquireDurationTotalDesc  *prometheus.Desc
	canceledAcquiresTotalDesc *prometheus.Desc
	emptyAcquiresTotalDesc    *prometheus.Desc
	newConnectionsTotalDesc   *prometheus.Desc
	destroyedConnectionsDesc  *prometheus.Desc
}

func newPoolMetricsCollector() *poolMetricsCollector {
	return &poolMetricsCollector{
		pools: sync.Map{},
		connectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "connections"),
			"Current PostgreSQL pool connections by state.",
			[]string{"state"},
			nil,
		),
		acquiresTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "acquires_total"),
			"Total successful PostgreSQL pool connection acquires.",
			nil,
			nil,
		),
		acquireDurationTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "acquire_duration_seconds_total"),
			"Total time spent acquiring PostgreSQL pool connections.",
			nil,
			nil,
		),
		canceledAcquiresTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "canceled_acquires_total"),
			"Total PostgreSQL pool acquires canceled by context.",
			nil,
			nil,
		),
		emptyAcquiresTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "empty_acquires_total"),
			"Total successful PostgreSQL pool acquires that waited because the pool was empty.",
			nil,
			nil,
		),
		newConnectionsTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "new_connections_total"),
			"Total PostgreSQL pool connections opened.",
			nil,
			nil,
		),
		destroyedConnectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "postgres_pool", "destroyed_connections_total"),
			"Total PostgreSQL pool connections destroyed by reason.",
			[]string{"reason"},
			nil,
		),
	}
}

func (c *poolMetricsCollector) Describe(descriptions chan<- *prometheus.Desc) {
	for _, description := range []*prometheus.Desc{
		c.connectionsDesc,
		c.acquiresTotalDesc,
		c.acquireDurationTotalDesc,
		c.canceledAcquiresTotalDesc,
		c.emptyAcquiresTotalDesc,
		c.newConnectionsTotalDesc,
		c.destroyedConnectionsDesc,
	} {
		descriptions <- description
	}
}

func (c *poolMetricsCollector) Collect(metrics chan<- prometheus.Metric) {
	if !frameworkmetrics.Enabled() {
		return
	}

	var stats aggregatePoolStats

	c.pools.Range(func(_, value any) bool {
		pool, ok := value.(*pgxpool.Pool)
		if !ok || pool == nil {
			return true
		}

		poolStats := pool.Stat()
		stats.acquireCount += poolStats.AcquireCount()
		stats.acquireDuration += poolStats.AcquireDuration()
		stats.acquiredConns += int64(poolStats.AcquiredConns())
		stats.canceledAcquireCount += poolStats.CanceledAcquireCount()
		stats.constructingConns += int64(poolStats.ConstructingConns())
		stats.emptyAcquireCount += poolStats.EmptyAcquireCount()
		stats.idleConns += int64(poolStats.IdleConns())
		stats.maxConns += int64(poolStats.MaxConns())
		stats.totalConns += int64(poolStats.TotalConns())
		stats.newConnsCount += poolStats.NewConnsCount()
		stats.maxLifetimeDestroyCount += poolStats.MaxLifetimeDestroyCount()
		stats.maxIdleDestroyCount += poolStats.MaxIdleDestroyCount()

		return true
	})

	connectionStates := []struct {
		state string
		value float64
	}{
		{state: "acquired", value: float64(stats.acquiredConns)},
		{state: "constructing", value: float64(stats.constructingConns)},
		{state: "idle", value: float64(stats.idleConns)},
		{state: "max", value: float64(stats.maxConns)},
		{state: "total", value: float64(stats.totalConns)},
	}
	for _, connectionState := range connectionStates {
		metrics <- prometheus.MustNewConstMetric(
			c.connectionsDesc,
			prometheus.GaugeValue,
			connectionState.value,
			connectionState.state,
		)
	}

	metrics <- prometheus.MustNewConstMetric(c.acquiresTotalDesc, prometheus.CounterValue, float64(stats.acquireCount))

	metrics <- prometheus.MustNewConstMetric(
		c.acquireDurationTotalDesc,
		prometheus.CounterValue,
		stats.acquireDuration.Seconds(),
	)

	metrics <- prometheus.MustNewConstMetric(
		c.canceledAcquiresTotalDesc,
		prometheus.CounterValue,
		float64(stats.canceledAcquireCount),
	)

	metrics <- prometheus.MustNewConstMetric(c.emptyAcquiresTotalDesc, prometheus.CounterValue, float64(stats.emptyAcquireCount))

	metrics <- prometheus.MustNewConstMetric(c.newConnectionsTotalDesc, prometheus.CounterValue, float64(stats.newConnsCount))

	metrics <- prometheus.MustNewConstMetric(
		c.destroyedConnectionsDesc,
		prometheus.CounterValue,
		float64(stats.maxLifetimeDestroyCount),
		"max_lifetime",
	)

	metrics <- prometheus.MustNewConstMetric(
		c.destroyedConnectionsDesc,
		prometheus.CounterValue,
		float64(stats.maxIdleDestroyCount),
		"max_idle",
	)
}

func registerPoolMetrics(pool *pgxpool.Pool) {
	if frameworkmetrics.Enabled() && pool != nil {
		registeredPoolMetrics().pools.Store(pool, pool)
	}
}

func unregisterPoolMetrics(pool *pgxpool.Pool) {
	if pool != nil {
		registeredPoolMetrics().pools.Delete(pool)
	}
}

func registeredPoolMetrics() *poolMetricsCollector {
	collector := newPoolMetricsCollector()

	if err := prometheus.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*poolMetricsCollector)
			if ok {
				return existing
			}
		}

		panic(err)
	}

	return collector
}

type aggregatePoolStats struct {
	acquireCount            int64
	acquireDuration         time.Duration
	acquiredConns           int64
	canceledAcquireCount    int64
	constructingConns       int64
	emptyAcquireCount       int64
	idleConns               int64
	maxConns                int64
	totalConns              int64
	newConnsCount           int64
	maxLifetimeDestroyCount int64
	maxIdleDestroyCount     int64
}
