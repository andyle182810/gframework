package domainmetrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andyle182810/gframework/domainmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

var errDomainFailed = errors.New("domain failed")

func TestResultFromError(t *testing.T) {
	t.Parallel()

	require.Equal(t, "success", domainmetrics.ResultFromError(nil))
	require.Equal(t, "canceled", domainmetrics.ResultFromError(context.DeadlineExceeded))
	require.Equal(t, "error", domainmetrics.ResultFromError(errDomainFailed))
}

func TestRecordAndObserve(t *testing.T) {
	t.Parallel()

	before := domainCounterValue(t, "purchase_plan", "created", "success")

	domainmetrics.RecordSuccess("purchase_plan", "created")
	domainmetrics.Observe("purchase_plan", "created", time.Millisecond, nil)

	require.InDelta(t, before+2, domainCounterValue(t, "purchase_plan", "created", "success"), 0)
}

func domainCounterValue(t *testing.T, domain, event, result string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "gframework_domain_events_total" {
			continue
		}

		for _, metric := range family.GetMetric() {
			labels := metric.GetLabel()
			if labels[0].GetValue() == domain && labels[1].GetValue() == event && labels[2].GetValue() == result {
				return metric.GetCounter().GetValue()
			}
		}
	}

	return 0
}
