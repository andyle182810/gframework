package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetrics_RecordsRouteTemplateAndStatus(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.Use(middleware.HTTPMetrics())
	e.GET("/widgets/:widgetId", func(ctx *echo.Context) error {
		return ctx.NoContent(http.StatusAccepted)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/123", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.InDelta(
		t,
		float64(1),
		counterValue(t, "/widgets/:widgetId", "202"),
		0,
	)
}

func TestHTTPMetrics_InfersHTTPErrorStatus(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.Use(middleware.HTTPMetrics())
	e.GET("/metrics-test/teapot", func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "short and stout")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics-test/teapot", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.InDelta(
		t,
		float64(1),
		counterValue(t, "/metrics-test/teapot", "418"),
		0,
	)
}

func counterValue(t *testing.T, route, status string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "gframework_http_server_requests_total" {
			continue
		}

		for _, metric := range family.GetMetric() {
			labels := metric.GetLabel()
			if labels[0].GetValue() == http.MethodGet && labels[1].GetValue() == route && labels[2].GetValue() == status {
				return metric.GetCounter().GetValue()
			}
		}
	}

	return 0
}
