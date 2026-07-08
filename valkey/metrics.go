package valkey

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "valkey"
	labelCommand     = "command"
	labelResult      = "result"
	commandPipeline  = "pipeline"
)

type commandMetrics struct {
	duration *prometheus.HistogramVec
	total    *prometheus.CounterVec
}

func newCommandMetrics() *commandMetrics {
	return &commandMetrics{
		duration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"command_duration_seconds",
				"Valkey command duration by command and result.",
				[]float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			),
			[]string{labelCommand, labelResult},
		),
		total: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"commands_total",
				"Total Valkey commands by command and result.",
			),
			[]string{labelCommand, labelResult},
		),
	}
}

type metricsHook struct {
	metrics *commandMetrics
}

func newMetricsHook() *metricsHook {
	return &metricsHook{metrics: newCommandMetrics()}
}

func (h *metricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *metricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if !frameworkmetrics.Enabled() {
			return next(ctx, cmd)
		}

		startedAt := time.Now()
		err := next(ctx, cmd)
		h.recordCommand(commandName(cmd), time.Since(startedAt), err)

		return err
	}
}

func (h *metricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if !frameworkmetrics.Enabled() {
			return next(ctx, cmds)
		}

		startedAt := time.Now()
		err := next(ctx, cmds)
		h.recordCommand(commandPipeline, time.Since(startedAt), err)

		return err
	}
}

func commandName(cmd redis.Cmder) string {
	if cmd == nil || cmd.Name() == "" {
		return "unknown"
	}

	return strings.ToLower(cmd.Name())
}

func commandResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, redis.Nil):
		return "nil"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func (h *metricsHook) recordCommand(command string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() || h.metrics == nil {
		return
	}

	result := commandResult(err)

	h.metrics.total.WithLabelValues(command, result).Inc()
	h.metrics.duration.WithLabelValues(command, result).Observe(duration.Seconds())
}

var _ redis.Hook = (*metricsHook)(nil)

type poolMetricsCollector struct {
	clients sync.Map

	connectionsDesc     *prometheus.Desc
	eventsTotalDesc     *prometheus.Desc
	waitDurationDesc    *prometheus.Desc
	pendingRequestsDesc *prometheus.Desc
}

func newPoolMetricsCollector() *poolMetricsCollector {
	return &poolMetricsCollector{
		clients: sync.Map{},
		connectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "valkey_pool", "connections"),
			"Current Valkey pool connections by state.",
			[]string{"state"},
			nil,
		),
		eventsTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "valkey_pool", "events_total"),
			"Total Valkey pool events by type.",
			[]string{"event"},
			nil,
		),
		waitDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "valkey_pool", "wait_duration_seconds_total"),
			"Total time spent waiting for Valkey pool connections.",
			nil,
			nil,
		),
		pendingRequestsDesc: prometheus.NewDesc(
			prometheus.BuildFQName("gframework", "valkey_pool", "pending_requests"),
			"Current pending Valkey pool requests.",
			nil,
			nil,
		),
	}
}

func (c *poolMetricsCollector) Describe(descriptions chan<- *prometheus.Desc) {
	for _, description := range []*prometheus.Desc{
		c.connectionsDesc,
		c.eventsTotalDesc,
		c.waitDurationDesc,
		c.pendingRequestsDesc,
	} {
		descriptions <- description
	}
}

func (c *poolMetricsCollector) Collect(metrics chan<- prometheus.Metric) {
	if !frameworkmetrics.Enabled() {
		return
	}

	var stats aggregatePoolStats

	c.clients.Range(func(_, value any) bool {
		client, ok := value.(*redis.Client)
		if !ok || client == nil {
			return true
		}

		poolStats := client.PoolStats()
		if poolStats == nil {
			return true
		}

		stats.hits += uint64(poolStats.Hits)
		stats.misses += uint64(poolStats.Misses)
		stats.timeouts += uint64(poolStats.Timeouts)
		stats.waitCount += uint64(poolStats.WaitCount)
		stats.unusable += uint64(poolStats.Unusable)
		stats.waitDuration += time.Duration(poolStats.WaitDurationNs)
		stats.totalConns += uint64(poolStats.TotalConns)
		stats.idleConns += uint64(poolStats.IdleConns)
		stats.staleConns += uint64(poolStats.StaleConns)
		stats.pendingRequests += uint64(poolStats.PendingRequests)

		return true
	})

	metrics <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stats.totalConns),
		"total",
	)

	metrics <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stats.idleConns),
		"idle",
	)

	metrics <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stats.staleConns),
		"stale",
	)

	events := map[string]uint64{
		"hit":      stats.hits,
		"miss":     stats.misses,
		"timeout":  stats.timeouts,
		"wait":     stats.waitCount,
		"unusable": stats.unusable,
	}
	for event, value := range events {
		metrics <- prometheus.MustNewConstMetric(
			c.eventsTotalDesc,
			prometheus.CounterValue,
			float64(value),
			event,
		)
	}

	metrics <- prometheus.MustNewConstMetric(
		c.waitDurationDesc,
		prometheus.CounterValue,
		stats.waitDuration.Seconds(),
	)

	metrics <- prometheus.MustNewConstMetric(
		c.pendingRequestsDesc,
		prometheus.GaugeValue,
		float64(stats.pendingRequests),
	)
}

type aggregatePoolStats struct {
	hits            uint64
	misses          uint64
	timeouts        uint64
	waitCount       uint64
	unusable        uint64
	waitDuration    time.Duration
	totalConns      uint64
	idleConns       uint64
	staleConns      uint64
	pendingRequests uint64
}

func registerPoolMetrics(client *redis.Client) {
	if frameworkmetrics.Enabled() && client != nil {
		registeredPoolMetrics().clients.Store(client, client)
	}
}

func unregisterPoolMetrics(client *redis.Client) {
	if client != nil {
		registeredPoolMetrics().clients.Delete(client)
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
