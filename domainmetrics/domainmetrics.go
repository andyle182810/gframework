package domainmetrics

import (
	"context"
	"errors"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "domain"
	labelDomain      = "domain"
	labelEvent       = "event"
	labelResult      = "result"
	resultUnknown    = "unknown"
)

type recorder struct {
	eventsTotal   *prometheus.CounterVec
	eventDuration *prometheus.HistogramVec
}

func newRecorder() *recorder {
	return &recorder{
		eventsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"events_total",
				"Total domain events by domain, event, and result.",
			),
			[]string{labelDomain, labelEvent, labelResult},
		),
		eventDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"event_duration_seconds",
				"Domain event duration by domain, event, and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			),
			[]string{labelDomain, labelEvent, labelResult},
		),
	}
}

// Record increments a domain event counter with a custom result value.
func Record(domain, event, result string) {
	if !frameworkmetrics.Enabled() {
		return
	}

	newRecorder().eventsTotal.WithLabelValues(domain, event, NormalizeResult(result)).Inc()
}

// RecordSuccess increments a successful domain event counter.
func RecordSuccess(domain, event string) {
	Record(domain, event, "success")
}

// RecordError increments a failed domain event counter.
func RecordError(domain, event string) {
	Record(domain, event, "error")
}

// Observe records both a domain event count and duration using the error as the result.
func Observe(domain, event string, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	result := ResultFromError(err)
	domainMetrics := newRecorder()

	domainMetrics.eventsTotal.WithLabelValues(domain, event, result).Inc()
	domainMetrics.eventDuration.WithLabelValues(domain, event, result).Observe(duration.Seconds())
}

func ResultFromError(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func NormalizeResult(result string) string {
	if result == "" {
		return resultUnknown
	}

	return result
}
