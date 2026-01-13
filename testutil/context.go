package testutil

import (
	"context"
	"testing"
	"time"
)

const defaultTimeout = 30 * time.Second

func ContextWithTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	return ContextWithCustomTimeout(t, defaultTimeout)
}

func ContextWithCustomTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	t.Cleanup(cancel)

	return ctx, cancel
}

func ContextWithDeadline(t *testing.T, deadline time.Time) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	t.Cleanup(cancel)

	return ctx, cancel
}

func Context(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	return ctx
}
