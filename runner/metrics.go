package runner

import (
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "runner"
	labelTier        = "tier"
	labelService     = "service"
	labelOperation   = "operation"
	labelResult      = "result"
)

type lifecycleMetrics struct {
	total            *prometheus.CounterVec
	duration         *prometheus.HistogramVec
	shutdownTimeouts *prometheus.CounterVec
}

func newLifecycleMetrics() *lifecycleMetrics {
	return &lifecycleMetrics{
		total: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"service_lifecycle_total",
				"Total runner service lifecycle operations by tier, service, operation, and result.",
			),
			[]string{labelTier, labelService, labelOperation, labelResult},
		),
		duration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"service_lifecycle_duration_seconds",
				"Runner service lifecycle operation duration by tier, service, operation, and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			),
			[]string{labelTier, labelService, labelOperation, labelResult},
		),
		shutdownTimeouts: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"shutdown_timeouts_total",
				"Total runner shutdown timeouts.",
			),
			nil,
		),
	}
}

func recordLifecycle(tier, service, operation string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	result := "success"

	if err != nil {
		result = "error"
	}

	metrics := newLifecycleMetrics()

	metrics.total.WithLabelValues(tier, service, operation, result).Inc()
	metrics.duration.WithLabelValues(tier, service, operation, result).Observe(duration.Seconds())
}

func recordShutdownTimeout() {
	if frameworkmetrics.Enabled() {
		newLifecycleMetrics().shutdownTimeouts.WithLabelValues().Inc()
	}
}
