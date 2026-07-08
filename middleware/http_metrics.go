package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "gframework"
	metricsSubsystem = "http_server"
	labelMethod      = "method"
	labelRoute       = "route"
	labelStatus      = "status"
)

type httpMetrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight *prometheus.GaugeVec
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		requestsTotal: frameworkmetrics.RegisterCounterVec(
			frameworkmetrics.CounterOpts(
				metricsNamespace,
				metricsSubsystem,
				"requests_total",
				"Total HTTP requests handled by the application HTTP server.",
			),
			[]string{labelMethod, labelRoute, labelStatus},
		),
		requestDuration: frameworkmetrics.RegisterHistogramVec(
			frameworkmetrics.HistogramOpts(
				metricsNamespace,
				metricsSubsystem,
				"request_duration_seconds",
				"Duration of HTTP requests handled by the application HTTP server.",
				[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			),
			[]string{labelMethod, labelRoute, labelStatus},
		),
		requestsInFlight: frameworkmetrics.RegisterGaugeVec(
			frameworkmetrics.GaugeOpts(
				metricsNamespace,
				metricsSubsystem,
				"requests_in_flight",
				"Current number of HTTP requests being handled by the application HTTP server.",
			),
			[]string{labelMethod, labelRoute},
		),
	}
}

func HTTPMetrics() echo.MiddlewareFunc {
	httpMetrics := newHTTPMetrics()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if !frameworkmetrics.Enabled() {
				return next(ctx)
			}

			method := ctx.Request().Method
			route := routeLabel(ctx)
			startedAt := time.Now()

			httpMetrics.requestsInFlight.WithLabelValues(method, route).Inc()
			defer httpMetrics.requestsInFlight.WithLabelValues(method, route).Dec()

			err := next(ctx)

			route = routeLabel(ctx)
			status := strconv.Itoa(statusCode(ctx, err))
			duration := time.Since(startedAt).Seconds()

			httpMetrics.requestsTotal.WithLabelValues(method, route, status).Inc()
			httpMetrics.requestDuration.WithLabelValues(method, route, status).Observe(duration)

			return err
		}
	}
}

func routeLabel(ctx *echo.Context) string {
	route := ctx.Path()
	if route == "" {
		return "unmatched"
	}

	return route
}

func statusCode(ctx *echo.Context, err error) int {
	status := http.StatusOK

	resp, unwrapErr := echo.UnwrapResponse(ctx.Response())
	if unwrapErr == nil && resp.Status != 0 {
		status = resp.Status
	}

	if err == nil {
		return status
	}

	if status != http.StatusOK {
		return status
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}

	return http.StatusInternalServerError
}
