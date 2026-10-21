package cache

import (
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "cache"
	labelHash        = "hash"
	labelOperation   = "operation"
	labelResult      = "result"
)

type cacheMetrics struct {
	operationsTotal   *prometheus.CounterVec
	operationDuration *prometheus.HistogramVec
}

func newCacheMetrics() *cacheMetrics {
	return &cacheMetrics{
		operationsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"operations_total",
				"Total cache operations by hash, operation, and result.",
			),
			[]string{labelHash, labelOperation, labelResult},
		),
		operationDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"operation_duration_seconds",
				"Cache operation duration by hash, operation, and result.",
				[]float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			),
			[]string{labelHash, labelOperation, labelResult},
		),
	}
}

func recordOperation(hash, operation, result string, duration time.Duration) {
	if !frameworkmetrics.Enabled() {
		return
	}

	cacheMetrics := newCacheMetrics()
	cacheMetrics.operationsTotal.WithLabelValues(hash, operation, result).Inc()
	cacheMetrics.operationDuration.WithLabelValues(hash, operation, result).Observe(duration.Seconds())
}
