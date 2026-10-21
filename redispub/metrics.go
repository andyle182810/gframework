package redispub

import (
	"context"
	"errors"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "redispub"
	labelTopic       = "topic"
	labelResult      = "result"
)

type publishMetrics struct {
	callsTotal      *prometheus.CounterVec
	messagesTotal   *prometheus.CounterVec
	publishDuration *prometheus.HistogramVec
}

func newPublishMetrics() *publishMetrics {
	return &publishMetrics{
		callsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"publish_calls_total",
				"Total Redis Stream publish calls by topic and result.",
			),
			[]string{labelTopic, labelResult},
		),
		messagesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"messages_published_total",
				"Total Redis Stream messages published by topic and result.",
			),
			[]string{labelTopic, labelResult},
		),
		publishDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"publish_duration_seconds",
				"Redis Stream publish duration by topic and result.",
				[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			),
			[]string{labelTopic, labelResult},
		),
	}
}

func recordPublishMetrics(topic string, messageCount int, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	result := publishResult(err)
	metrics := newPublishMetrics()

	metrics.callsTotal.WithLabelValues(topic, result).Inc()
	metrics.messagesTotal.WithLabelValues(topic, result).Add(float64(messageCount))
	metrics.publishDuration.WithLabelValues(topic, result).Observe(duration.Seconds())
}

func publishResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}
