package httpclient

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace         = "gframework"
	metricsSubsystem         = "httpclient"
	labelUpstream            = "upstream"
	labelMethod              = "method"
	labelPath                = "path"
	labelStatus              = "status"
	labelResult              = "result"
	statusNone               = "none"
	dynamicPathSegmentRegexp = `(?i)^([0-9]+|[0-9a-f]{16,}|[0-9a-f-]{32,})$`
)

type clientMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func newClientMetrics() *clientMetrics {
	return &clientMetrics{
		requestsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"requests_total",
				"Total outbound HTTP requests by upstream, method, path, status, and result.",
			),
			[]string{labelUpstream, labelMethod, labelPath, labelStatus, labelResult},
		),
		requestDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"request_duration_seconds",
				"Outbound HTTP request duration by upstream, method, path, status, and result.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			),
			[]string{labelUpstream, labelMethod, labelPath, labelStatus, labelResult},
		),
	}
}

func recordRequest(upstream, method, path string, statusCode int, duration time.Duration, err error) {
	if !frameworkmetrics.Enabled() {
		return
	}

	status := statusNone

	if statusCode > 0 {
		status = strconv.Itoa(statusCode)
	}

	result := requestResult(err)
	clientMetrics := newClientMetrics()

	clientMetrics.requestsTotal.WithLabelValues(upstream, method, path, status, result).Inc()
	clientMetrics.requestDuration.WithLabelValues(upstream, method, path, status, result).Observe(duration.Seconds())
}

func (c *Client) recordRequest(upstream, method, path string, statusCode int, duration time.Duration, err error) {
	if c.metricsEnabled {
		recordRequest(upstream, method, path, statusCode, duration, err)
	}
}

func requestResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}

func upstreamLabel(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}

	return parsed.Host
}

func pathLabel(path string) string {
	parsed, err := url.Parse(path)
	if err == nil && parsed.Path != "" {
		path = parsed.Path
	}

	if path == "" {
		return "/"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	parts := strings.Split(path, "/")
	for index, part := range parts {
		if dynamicPathSegment(part) {
			parts[index] = ":id"
		}
	}

	return strings.Join(parts, "/")
}

func dynamicPathSegment(segment string) bool {
	matched, err := regexp.MatchString(dynamicPathSegmentRegexp, segment)
	if err != nil {
		return false
	}

	return matched
}
