package spaces

import (
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "spaces"
	labelBucket      = "bucket"
	labelOperation   = "operation"
	labelResult      = "result"
)

type spaceMetrics struct {
	operationsTotal   *prometheus.CounterVec
	operationDuration *prometheus.HistogramVec
	bytesReadTotal    *prometheus.CounterVec
}

func newSpaceMetrics() *spaceMetrics {
	return &spaceMetrics{
		operationsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"operations_total",
				"Total object storage operations by bucket, operation, and result.",
			),
			[]string{labelBucket, labelOperation, labelResult},
		),
		operationDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"operation_duration_seconds",
				"Object storage operation duration by bucket, operation, and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			),
			[]string{labelBucket, labelOperation, labelResult},
		),
		bytesReadTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"bytes_read_total",
				"Total bytes read from object storage by bucket and operation.",
			),
			[]string{labelBucket, labelOperation},
		),
	}
}

func recordSpaceOperation(bucket, operation string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	result := "success"

	if err != nil {
		result = "error"
	}

	metrics := newSpaceMetrics()

	metrics.operationsTotal.WithLabelValues(bucket, operation, result).Inc()
	metrics.operationDuration.WithLabelValues(bucket, operation, result).Observe(duration.Seconds())
}

func recordBytesRead(bucket, operation string, bytes int) {
	if frameworkmetrics.Enabled() && bytes > 0 {
		newSpaceMetrics().bytesReadTotal.WithLabelValues(bucket, operation).Add(float64(bytes))
	}
}
