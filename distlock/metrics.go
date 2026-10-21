package distlock

import (
	"regexp"
	"strings"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace         = "gframework"
	metricsSubsystem         = "distlock"
	labelLock                = "lock"
	labelResult              = "result"
	dynamicLockSegmentRegexp = `(?i)^([0-9]+|[0-9a-f]{16,}|[0-9a-f-]{32,})$`
)

type lockMetrics struct {
	attemptsTotal      *prometheus.CounterVec
	handlerDuration    *prometheus.HistogramVec
	releaseErrorsTotal *prometheus.CounterVec
}

func newLockMetrics() *lockMetrics {
	return &lockMetrics{
		attemptsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"attempts_total",
				"Total distributed lock attempts by lock and result.",
			),
			[]string{labelLock, labelResult},
		),
		handlerDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"handler_duration_seconds",
				"Distributed lock handler duration by lock and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			),
			[]string{labelLock, labelResult},
		),
		releaseErrorsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"release_errors_total",
				"Total distributed lock release errors by lock.",
			),
			[]string{labelLock},
		),
	}
}

func lockLabel(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "unknown"
	}

	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == ':' || r == '/' || r == '|'
	})
	for index, part := range parts {
		if dynamicLockSegment(part) {
			parts[index] = "id"
		}
	}

	return strings.Join(parts, ":")
}

func dynamicLockSegment(segment string) bool {
	matched, err := regexp.MatchString(dynamicLockSegmentRegexp, segment)
	if err != nil {
		return false
	}

	return matched
}

func recordLockAttempt(lock, result string) {
	if !frameworkmetrics.Enabled() {
		return
	}

	newLockMetrics().attemptsTotal.WithLabelValues(lockLabel(lock), result).Inc()
}

func recordLockHandler(lock, result string, duration time.Duration) {
	if !frameworkmetrics.Enabled() {
		return
	}

	newLockMetrics().handlerDuration.WithLabelValues(lockLabel(lock), result).Observe(duration.Seconds())
}

func recordLockReleaseError(lock string) {
	if !frameworkmetrics.Enabled() {
		return
	}

	newLockMetrics().releaseErrorsTotal.WithLabelValues(lockLabel(lock)).Inc()
}
