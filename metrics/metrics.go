// Package metrics controls built-in gframework Prometheus instrumentation.
package metrics

import "expvar"

const enabledVarName = "gframework_metrics_enabled"

// Enabled reports whether built-in gframework Prometheus metrics should be recorded.
func Enabled() bool {
	value, ok := expvar.Get(enabledVarName).(*expvar.Int)
	if !ok {
		return true
	}

	return value.Value() != 0
}

// SetEnabled enables or disables built-in gframework Prometheus metrics globally.
func SetEnabled(value bool) {
	metric := enabledVar()

	if value {
		metric.Set(1)

		return
	}

	metric.Set(0)
}

// Enable turns on built-in gframework Prometheus metrics globally.
func Enable() {
	SetEnabled(true)
}

// Disable turns off built-in gframework Prometheus metrics globally.
func Disable() {
	SetEnabled(false)
}

func enabledVar() *expvar.Int {
	value, ok := expvar.Get(enabledVarName).(*expvar.Int)
	if ok {
		return value
	}

	return expvar.NewInt(enabledVarName)
}
