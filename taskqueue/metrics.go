package taskqueue

import (
	"context"
	"errors"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "taskqueue"
	labelQueue       = "queue"
	labelResult      = "result"
)

type queueMetrics struct {
	pushCallsTotal         *prometheus.CounterVec
	tasksPushedTotal       *prometheus.CounterVec
	tasksFetchedTotal      *prometheus.CounterVec
	tasksProcessedTotal    *prometheus.CounterVec
	taskProcessingDuration *prometheus.HistogramVec
	tasksInFlight          *prometheus.GaugeVec
	queueLength            *prometheus.GaugeVec
	processingCount        *prometheus.GaugeVec
	staleRecoveriesTotal   *prometheus.CounterVec
	running                *prometheus.GaugeVec
	workerCount            *prometheus.GaugeVec
}

func newQueueMetrics() *queueMetrics {
	return &queueMetrics{
		pushCallsTotal: newQueueCounter("push_calls_total", "Total task queue push calls by queue and result."),
		tasksPushedTotal: newQueueCounter(
			"tasks_pushed_total",
			"Total tasks pushed by queue and result.",
		),
		tasksFetchedTotal: newQueueCounter(
			"tasks_fetched_total",
			"Total tasks fetched by queue and result.",
		),
		tasksProcessedTotal: newQueueCounter(
			"tasks_processed_total",
			"Total tasks processed by queue and result.",
		),
		taskProcessingDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"task_processing_duration_seconds",
				"Task processing duration by queue and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			),
			[]string{labelQueue, labelResult},
		),
		tasksInFlight: newQueueGauge("tasks_in_flight", "Current tasks executing by queue."),
		queueLength:   newQueueGauge("queue_length", "Current queued task count by queue."),
		processingCount: newQueueGauge(
			"processing_tasks",
			"Current processing-set task count by queue.",
		),
		staleRecoveriesTotal: newQueueCounter(
			"stale_recoveries_total",
			"Total stale task recovery outcomes by queue and result.",
		),
		running:     newQueueGauge("running", "Whether a task queue is currently running."),
		workerCount: newQueueGauge("workers", "Configured task queue worker count by queue."),
	}
}

func newQueueCounter(name, help string) *prometheus.CounterVec {
	return frameworkmetrics.RegisterCounterVec(
		frameworkmetrics.CounterOpts(metricsNamespace, metricsSubsystem, name, help),
		[]string{labelQueue, labelResult},
	)
}

func newQueueGauge(name, help string) *prometheus.GaugeVec {
	return frameworkmetrics.RegisterGaugeVec(
		frameworkmetrics.GaugeOpts(metricsNamespace, metricsSubsystem, name, help),
		[]string{labelQueue},
	)
}

func taskResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, redis.Nil):
		return "empty"
	default:
		return "error"
	}
}

func (m *queueMetrics) recordPush(queue string, count int, err error) {
	if !frameworkmetrics.Enabled() || m == nil {
		return
	}

	result := taskResult(err)

	m.pushCallsTotal.WithLabelValues(queue, result).Inc()
	m.tasksPushedTotal.WithLabelValues(queue, result).Add(float64(count))
}

func (m *queueMetrics) recordFetched(queue string, err error) {
	if frameworkmetrics.Enabled() && m != nil {
		m.tasksFetchedTotal.WithLabelValues(queue, taskResult(err)).Inc()
	}
}

func (m *queueMetrics) recordProcessed(queue string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() || m == nil {
		return
	}

	result := taskResult(err)

	m.tasksProcessedTotal.WithLabelValues(queue, result).Inc()
	m.taskProcessingDuration.WithLabelValues(queue, result).Observe(duration.Seconds())
}

func (m *queueMetrics) recordRecovered(queue string, recovered, failed int) {
	if !frameworkmetrics.Enabled() || m == nil {
		return
	}

	if recovered > 0 {
		m.staleRecoveriesTotal.WithLabelValues(queue, "success").Add(float64(recovered))
	}

	if failed > 0 {
		m.staleRecoveriesTotal.WithLabelValues(queue, "error").Add(float64(failed))
	}
}

func (m *queueMetrics) recordWorkers(queue string, workers int) {
	if frameworkmetrics.Enabled() && m != nil {
		m.workerCount.WithLabelValues(queue).Set(float64(workers))
	}
}

func (m *queueMetrics) recordRunning(queue string, value float64) {
	if frameworkmetrics.Enabled() && m != nil {
		m.running.WithLabelValues(queue).Set(value)
	}
}

func (m *queueMetrics) incInFlight(queue string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.tasksInFlight.WithLabelValues(queue).Inc()
	}
}

func (m *queueMetrics) decInFlight(queue string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.tasksInFlight.WithLabelValues(queue).Dec()
	}
}

func (m *queueMetrics) recordQueueLength(queue string, value int64) {
	if frameworkmetrics.Enabled() && m != nil {
		m.queueLength.WithLabelValues(queue).Set(float64(value))
	}
}

func (m *queueMetrics) recordProcessingCount(queue string, value int64) {
	if frameworkmetrics.Enabled() && m != nil {
		m.processingCount.WithLabelValues(queue).Set(float64(value))
	}
}
