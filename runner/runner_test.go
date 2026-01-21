package runner_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/runner"
	"github.com/stretchr/testify/require"
)

var errStart = errors.New("start error")

var errStop = errors.New("stop error")

type mockService struct {
	name         string
	startErr     error
	stopErr      error
	startDelay   time.Duration
	stopDelay    time.Duration
	panicOnStart bool
	started      atomic.Bool
	stopped      atomic.Bool
}

func newMockService(name string) *mockService {
	return &mockService{
		name:         name,
		startErr:     nil,
		stopErr:      nil,
		startDelay:   0,
		stopDelay:    0,
		panicOnStart: false,
		started:      atomic.Bool{},
		stopped:      atomic.Bool{},
	}
}

func (m *mockService) Start(ctx context.Context) error {
	if m.panicOnStart {
		panic("mock panic on start")
	}

	if m.startDelay > 0 {
		select {
		case <-time.After(m.startDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.startErr != nil {
		return m.startErr
	}

	m.started.Store(true)

	return nil
}

func (m *mockService) Stop() error {
	if m.stopDelay > 0 {
		time.Sleep(m.stopDelay)
	}

	if m.stopErr != nil {
		return m.stopErr
	}

	m.stopped.Store(true)

	return nil
}

func (m *mockService) Name() string {
	return m.name
}

func (m *mockService) wasStarted() bool {
	return m.started.Load()
}

func (m *mockService) wasStopped() bool {
	return m.stopped.Load()
}

func TestNew_DefaultValues(t *testing.T) {
	t.Parallel()

	r := runner.New()
	require.NotNil(t, r)
}

func TestNew_WithCoreService(t *testing.T) {
	t.Parallel()

	svc := newMockService("core-svc")

	r := runner.New(runner.WithCoreService(svc))
	require.NotNil(t, r)
}

func TestNew_WithMultipleCoreServices(t *testing.T) {
	t.Parallel()

	svc1 := newMockService("core1")
	svc2 := newMockService("core2")

	r := runner.New(
		runner.WithCoreService(svc1),
		runner.WithCoreService(svc2),
	)
	require.NotNil(t, r)
}

func TestNew_WithInfrastructureService(t *testing.T) {
	t.Parallel()

	svc := newMockService("infra-svc")

	r := runner.New(runner.WithInfrastructureService(svc))
	require.NotNil(t, r)
}

func TestNew_WithMultipleInfrastructureServices(t *testing.T) {
	t.Parallel()

	svc1 := newMockService("infra1")
	svc2 := newMockService("infra2")

	r := runner.New(
		runner.WithInfrastructureService(svc1),
		runner.WithInfrastructureService(svc2),
	)
	require.NotNil(t, r)
}

func TestNew_WithShutdownTimeout(t *testing.T) {
	t.Parallel()

	r := runner.New(runner.WithShutdownTimeout(5 * time.Second))
	require.NotNil(t, r)
}

func TestNew_WithAllOptions(t *testing.T) {
	t.Parallel()

	coreSvc := newMockService("core")
	infraSvc := newMockService("infra")

	r := runner.New(
		runner.WithCoreService(coreSvc),
		runner.WithInfrastructureService(infraSvc),
		runner.WithShutdownTimeout(10*time.Second),
	)
	require.NotNil(t, r)
}

func TestRunner_ErrServicePanicIsDefined(t *testing.T) {
	t.Parallel()

	if runner.ErrServicePanic == nil {
		t.Error("ErrServicePanic should not be nil")
	}
}

func TestRunner_ErrServiceFailedIsDefined(t *testing.T) {
	t.Parallel()

	if runner.ErrServiceFailed == nil {
		t.Error("ErrServiceFailed should not be nil")
	}
}

func TestRunner_ErrShutdownTimeoutIsDefined(t *testing.T) {
	t.Parallel()

	if runner.ErrShutdownTimout == nil {
		t.Error("ErrShutdownTimout should not be nil")
	}
}

func TestRunner_ServiceInterfaceImplementation(t *testing.T) {
	t.Parallel()

	var _ runner.Service = newMockService("test")
}

func TestMockService_StartReturnsError(t *testing.T) {
	t.Parallel()

	svc := newMockService("error-svc")
	svc.startErr = errStart

	err := svc.Start(t.Context())
	if !errors.Is(err, errStart) {
		t.Errorf("expected errStart, got %v", err)
	}

	if svc.wasStarted() {
		t.Error("service should not be marked as started when error occurs")
	}
}

func TestMockService_StopReturnsError(t *testing.T) {
	t.Parallel()

	svc := newMockService("error-svc")
	svc.stopErr = errStop

	err := svc.Stop()
	if !errors.Is(err, errStop) {
		t.Errorf("expected errStop, got %v", err)
	}

	if svc.wasStopped() {
		t.Error("service should not be marked as stopped when error occurs")
	}
}

func TestMockService_StartWithDelay(t *testing.T) {
	t.Parallel()

	svc := newMockService("delayed-svc")
	svc.startDelay = 50 * time.Millisecond

	start := time.Now()

	err := svc.Start(t.Context())

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if elapsed < 50*time.Millisecond {
		t.Errorf("expected delay of at least 50ms, got %v", elapsed)
	}

	if !svc.wasStarted() {
		t.Error("service should be marked as started")
	}
}

func TestMockService_StartWithDelayRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	svc := newMockService("delayed-svc")
	svc.startDelay = 500 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()

	err := svc.Start(ctx)

	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected cancellation before full delay, took %v", elapsed)
	}

	if svc.wasStarted() {
		t.Error("service should not be marked as started when cancelled")
	}
}

func TestMockService_StopWithDelay(t *testing.T) {
	t.Parallel()

	svc := newMockService("delayed-svc")
	svc.stopDelay = 50 * time.Millisecond

	start := time.Now()

	err := svc.Stop()

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if elapsed < 50*time.Millisecond {
		t.Errorf("expected delay of at least 50ms, got %v", elapsed)
	}

	if !svc.wasStopped() {
		t.Error("service should be marked as stopped")
	}
}

func TestMockService_PanicOnStart(t *testing.T) {
	t.Parallel()

	svc := newMockService("panic-svc")
	svc.panicOnStart = true

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic but none occurred")
		}
	}()

	_ = svc.Start(t.Context())
}

func TestMockService_Name(t *testing.T) {
	t.Parallel()

	svc := newMockService("test-service")

	if svc.Name() != "test-service" {
		t.Errorf("expected name 'test-service', got '%s'", svc.Name())
	}
}

func TestMockService_SuccessfulStartAndStop(t *testing.T) {
	t.Parallel()

	svc := newMockService("normal-svc")

	if err := svc.Start(t.Context()); err != nil {
		t.Errorf("unexpected start error: %v", err)
	}

	if !svc.wasStarted() {
		t.Error("service should be marked as started")
	}

	if err := svc.Stop(); err != nil {
		t.Errorf("unexpected stop error: %v", err)
	}

	if !svc.wasStopped() {
		t.Error("service should be marked as stopped")
	}
}

func TestMockService_ConcurrentStopIsSafe(t *testing.T) {
	t.Parallel()

	svc := newMockService("concurrent-svc")
	svc.stopDelay = 10 * time.Millisecond

	done := make(chan struct{})

	for range 3 {
		go func() {
			_ = svc.Stop()
			done <- struct{}{}
		}()
	}

	for range 3 {
		<-done
	}
}
