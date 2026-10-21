package keycloak

import (
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "keycloak"
	labelOperation   = "operation"
	labelResult      = "result"
)

type operationMetrics struct {
	duration *prometheus.HistogramVec
	total    *prometheus.CounterVec
}

func newOperationMetrics() *operationMetrics {
	return &operationMetrics{
		duration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"operation_duration_seconds",
				"Keycloak operation duration by operation and result.",
				[]float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			),
			[]string{labelOperation, labelResult},
		),
		total: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"operations_total",
				"Total Keycloak operations by operation and result.",
			),
			[]string{labelOperation, labelResult},
		),
	}
}

func recordOperation(operation, result string, duration time.Duration) {
	if !frameworkmetrics.Enabled() {
		return
	}

	metrics := newOperationMetrics()

	metrics.total.WithLabelValues(operation, result).Inc()
	metrics.duration.WithLabelValues(operation, result).Observe(duration.Seconds())
}

func operationResult(err error) string {
	if err != nil {
		return "error"
	}

	return "success"
}
