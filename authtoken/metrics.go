package authtoken

import (
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "authtoken"
	labelRealm       = "realm"
	labelResult      = "result"
)

type tokenMetrics struct {
	requestsTotal      *prometheus.CounterVec
	fetchDuration      *prometheus.HistogramVec
	invalidationsTotal *prometheus.CounterVec
}

func newTokenMetrics() *tokenMetrics {
	return &tokenMetrics{
		requestsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"requests_total",
				"Total auth token requests by realm and result.",
			),
			[]string{labelRealm, labelResult},
		),
		fetchDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"fetch_duration_seconds",
				"Auth token fetch duration by realm and result.",
				[]float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			),
			[]string{labelRealm, labelResult},
		),
		invalidationsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"invalidations_total",
				"Total auth token cache invalidations by realm.",
			),
			[]string{labelRealm},
		),
	}
}

func (m *tokenMetrics) recordRequest(realm, result string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.requestsTotal.WithLabelValues(realm, result).Inc()
	}
}

func (m *tokenMetrics) recordFetch(realm, result string, duration time.Duration) {
	if frameworkmetrics.Enabled() && m != nil {
		m.fetchDuration.WithLabelValues(realm, result).Observe(duration.Seconds())
	}
}

func (m *tokenMetrics) recordInvalidation(realm string) {
	if frameworkmetrics.Enabled() && m != nil {
		m.invalidationsTotal.WithLabelValues(realm).Inc()
	}
}

func (c *Client) recordTokenRequest(result string) {
	if c.metricsEnabled {
		c.metrics.recordRequest(c.realm, result)
	}
}

func (c *Client) recordTokenFetch(result string, duration time.Duration) {
	if c.metricsEnabled {
		c.metrics.recordFetch(c.realm, result, duration)
	}
}

func (c *Client) recordTokenInvalidation() {
	if c.metricsEnabled {
		c.metrics.recordInvalidation(c.realm)
	}
}
