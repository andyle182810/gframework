package workerpool

import (
	"context"
	"errors"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "workerpool"
	labelPool        = "pool"
	labelResult      = "result"
)

type workerMetrics struct {
	executionsTotal    *prometheus.CounterVec
	executionDuration  *prometheus.HistogramVec
	executionsInFlight *prometheus.GaugeVec
	workerCount        *prometheus.GaugeVec
	running            *prometheus.GaugeVec
}

func newWorkerMetrics() *workerMetrics {
	return &workerMetrics{
		executionsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"executions_total",
				"Total worker pool executions by pool and result.",
			),
			[]string{labelPool, labelResult},
		),
		executionDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"execution_duration_seconds",
				"Worker pool execution duration by pool and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			),
			[]string{labelPool, labelResult},
		),
		executionsInFlight: frameworkmetrics.RegisterGaugeVec(
			frameworkmetrics.GaugeOpts(
				metricsNamespace,
				metricsSubsystem,
				"executions_in_flight",
				"Current worker pool executions by pool.",
			),
			[]string{labelPool},
		),
		workerCount: frameworkmetrics.RegisterGaugeVec(
			frameworkmetrics.GaugeOpts(
				metricsNamespace,
				metricsSubsystem,
				"workers",
				"Configured worker count by pool.",
			),
			[]string{labelPool},
		),
		running: frameworkmetrics.RegisterGaugeVec(
			frameworkmetrics.GaugeOpts(
				metricsNamespace,
				metricsSubsystem,
				"running",
				"Whether a worker pool is currently running.",
			),
			[]string{labelPool},
		),
	}
}

func executionResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func (m *workerMetrics) recordExecution(poolName string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() || m == nil {
		return
	}

	result := executionResult(err)

	m.executionsTotal.WithLabelValues(poolName, result).Inc()
	m.executionDuration.WithLabelValues(poolName, result).Observe(duration.Seconds())
}

func (m *workerMetrics) recordWorkers(poolName string, workers int) {
	if frameworkmetrics.Enabled() && m != nil {
		m.workerCount.WithLabelValues(poolName).Set(float64(workers))
	}
}

func (m *workerMetrics) recordRunning(poolName string, value float64) {
	if frameworkmetrics.Enabled() && m != nil {
		m.running.WithLabelValues(poolName).Set(value)
	}
}

func (m *workerMetrics) incInFlight(poolName string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.executionsInFlight.WithLabelValues(poolName).Inc()
	}
}

func (m *workerMetrics) decInFlight(poolName string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.executionsInFlight.WithLabelValues(poolName).Dec()
	}
}
