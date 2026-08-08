package websocket

import (
	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	gws "github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "websocket"
	labelDirection   = "direction"
	labelIO          = "io"
	labelMessageType = "message_type"
	labelResult      = "result"
)

type socketMetrics struct {
	connectionAttemptsTotal *prometheus.CounterVec
	activeConnections       *prometheus.GaugeVec
	messagesTotal           *prometheus.CounterVec
	messageBytesTotal       *prometheus.CounterVec
	closesTotal             *prometheus.CounterVec
}

func newSocketMetrics() *socketMetrics {
	return &socketMetrics{
		connectionAttemptsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"connection_attempts_total",
				"Total WebSocket connection attempts by direction and result.",
			),
			[]string{labelDirection, labelResult},
		),
		activeConnections: frameworkmetrics.RegisterGaugeVec(
			frameworkmetrics.GaugeOpts(
				metricsNamespace,
				metricsSubsystem,
				"active_connections",
				"Current active WebSocket connections by direction.",
			),
			[]string{labelDirection},
		),
		messagesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"messages_total",
				"Total WebSocket messages by connection direction, IO direction, message type, and result.",
			),
			[]string{labelDirection, labelIO, labelMessageType, labelResult},
		),
		messageBytesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"message_bytes_total",
				"Total WebSocket message bytes by connection direction, IO direction, and message type.",
			),
			[]string{labelDirection, labelIO, labelMessageType},
		),
		closesTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"closes_total",
				"Total WebSocket closes by direction and result.",
			),
			[]string{labelDirection, labelResult},
		),
	}
}

func recordConnectionAttempt(direction string, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	newSocketMetrics().connectionAttemptsTotal.WithLabelValues(direction, resultLabel(err)).Inc()
}

func recordMessage(direction, ioDirection string, messageType int, bytes int, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	msgType := messageTypeLabel(messageType)
	metrics := newSocketMetrics()

	metrics.messagesTotal.WithLabelValues(direction, ioDirection, msgType, resultLabel(err)).Inc()

	if err == nil && bytes > 0 {
		metrics.messageBytesTotal.WithLabelValues(direction, ioDirection, msgType).Add(float64(bytes))
	}
}

func incActiveConnection(direction string) {
	if frameworkmetrics.Enabled() {
		newSocketMetrics().activeConnections.WithLabelValues(direction).Inc()
	}
}

func decActiveConnection(direction string) {
	if frameworkmetrics.Enabled() {
		newSocketMetrics().activeConnections.WithLabelValues(direction).Dec()
	}
}

func recordClose(direction string, err error) {
	if frameworkmetrics.Enabled() {
		newSocketMetrics().closesTotal.WithLabelValues(direction, resultLabel(err)).Inc()
	}
}

func (ws *WebSocket) recordConnectionAttempt(direction string, err error) {
	if ws.metricsEnabled {
		recordConnectionAttempt(direction, err)
	}
}

func resultLabel(err error) string {
	if err != nil {
		return "error"
	}

	return "success"
}

func messageTypeLabel(messageType int) string {
	switch messageType {
	case gws.TextMessage:
		return "text"
	case gws.BinaryMessage:
		return "binary"
	case gws.CloseMessage:
		return "close"
	case gws.PingMessage:
		return "ping"
	case gws.PongMessage:
		return "pong"
	default:
		return "other"
	}
}
