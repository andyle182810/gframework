package redissub

import (
	"context"
	"errors"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace   = "gframework"
	metricsSubsystem   = "redissub"
	labelTopic         = "topic"
	labelConsumerGroup = "consumer_group"
	labelResult        = "result"
)

type subscriberMetrics struct {
	receivedTotal      *prometheus.CounterVec
	processedTotal     *prometheus.CounterVec
	processingDuration *prometheus.HistogramVec
	attemptsTotal      *prometheus.CounterVec
	acksTotal          *prometheus.CounterVec
	nacksTotal         *prometheus.CounterVec
	sentToDLQTotal     *prometheus.CounterVec
}

func newSubscriberMetrics() *subscriberMetrics {
	return &subscriberMetrics{
		receivedTotal: newTopicConsumerCounter(
			"messages_received_total",
			"Total Redis Stream messages received by topic and consumer group.",
		),
		processedTotal: newTopicConsumerResultCounter(
			"messages_processed_total",
			"Total Redis Stream messages processed by topic, consumer group, and result.",
		),
		processingDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"message_processing_duration_seconds",
				"Redis Stream message processing duration by topic, consumer group, and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			),
			[]string{labelTopic, labelConsumerGroup, labelResult},
		),
		attemptsTotal: newTopicConsumerResultCounter(
			"message_attempts_total",
			"Total Redis Stream message processing attempts by topic, consumer group, and result.",
		),
		acksTotal: newTopicConsumerCounter(
			"message_acks_total",
			"Total Redis Stream message acknowledgements by topic and consumer group.",
		),
		nacksTotal: newTopicConsumerCounter(
			"message_nacks_total",
			"Total Redis Stream message negative acknowledgements by topic and consumer group.",
		),
		sentToDLQTotal: newTopicConsumerCounter(
			"messages_sent_to_dlq_total",
			"Total Redis Stream messages sent to a dead-letter queue by topic and consumer group.",
		),
	}
}

func newTopicConsumerCounter(name, help string) *prometheus.CounterVec {
	return frameworkmetrics.RegisterCounterVec(
		frameworkmetrics.CounterOpts(metricsNamespace, metricsSubsystem, name, help),
		[]string{labelTopic, labelConsumerGroup},
	)
}

func newTopicConsumerResultCounter(name, help string) *prometheus.CounterVec {
	return frameworkmetrics.RegisterCounterVec(
		frameworkmetrics.CounterOpts(metricsNamespace, metricsSubsystem, name, help),
		[]string{labelTopic, labelConsumerGroup, labelResult},
	)
}

func messageResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrExecTimeout):
		return "canceled"
	default:
		return "error"
	}
}

func recordMessageReceived(topic, consumerGroup string) {
	if frameworkmetrics.Enabled() {
		newSubscriberMetrics().receivedTotal.WithLabelValues(topic, consumerGroup).Inc()
	}
}

func recordMessageProcessed(topic, consumerGroup, result string, durationSeconds float64) {
	if !frameworkmetrics.Enabled() {
		return
	}

	metrics := newSubscriberMetrics()

	metrics.processedTotal.WithLabelValues(topic, consumerGroup, result).Inc()
	metrics.processingDuration.WithLabelValues(topic, consumerGroup, result).Observe(durationSeconds)
}

func recordMessageAttempt(topic, consumerGroup, result string) {
	if frameworkmetrics.Enabled() {
		newSubscriberMetrics().attemptsTotal.WithLabelValues(topic, consumerGroup, result).Inc()
	}
}

func recordMessageAck(topic, consumerGroup string) {
	if frameworkmetrics.Enabled() {
		newSubscriberMetrics().acksTotal.WithLabelValues(topic, consumerGroup).Inc()
	}
}

func recordMessageNack(topic, consumerGroup string) {
	if frameworkmetrics.Enabled() {
		newSubscriberMetrics().nacksTotal.WithLabelValues(topic, consumerGroup).Inc()
	}
}

func recordMessageSentToDLQ(topic, consumerGroup string) {
	if frameworkmetrics.Enabled() {
		newSubscriberMetrics().sentToDLQTotal.WithLabelValues(topic, consumerGroup).Inc()
	}
}
