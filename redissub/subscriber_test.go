//nolint:exhaustruct,paralleltest,tparallel
package redissub_test

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/andyle182810/gframework/redispub"
	"github.com/andyle182810/gframework/redissub"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

var (
	errTemporary  = errors.New("temporary error")
	errAlwaysFail = errors.New("always fail")
)

func setupTestClient(t *testing.T) *valkey.Valkey {
	t.Helper()

	container := testutil.SetupValkeyContainer(t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	valkeyClient, err := valkey.New(&valkey.Config{
		Host: container.Host,
		Port: port,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = valkeyClient.Close()
	})

	return valkeyClient
}

func setupTestPublisher(t *testing.T, client *valkey.Valkey) *redispub.RedisPublisher {
	t.Helper()

	publisher, err := redispub.New(client.Client, redispub.Options{})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = publisher.Close()
	})

	return publisher
}

func publishTestMessage(t *testing.T, publisher *redispub.RedisPublisher, topic, payload string) {
	t.Helper()

	ctx := t.Context()

	err := publisher.PublishToTopic(ctx, topic, payload)
	require.NoError(t, err)
}

func TestNewSubscriber(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	tests := []struct {
		name          string
		client        func() goredis.UniversalClient
		consumerGroup string
		topic         string
		handler       redissub.MessageHandler
		expectError   error
	}{
		{
			name:          "valid configuration",
			client:        func() goredis.UniversalClient { return valkeyClient.Client },
			consumerGroup: "test-group",
			topic:         "test-topic",
			handler:       handler,
			expectError:   nil,
		},
		{
			name:          "nil client",
			client:        func() goredis.UniversalClient { return nil },
			consumerGroup: "test-group",
			topic:         "test-topic",
			handler:       handler,
			expectError:   redissub.ErrNilRedisClient,
		},
		{
			name:          "empty consumer group",
			client:        func() goredis.UniversalClient { return valkeyClient.Client },
			consumerGroup: "",
			topic:         "test-topic",
			handler:       handler,
			expectError:   redissub.ErrEmptyConsumerGroup,
		},
		{
			name:          "empty topic",
			client:        func() goredis.UniversalClient { return valkeyClient.Client },
			consumerGroup: "test-group",
			topic:         "",
			handler:       handler,
			expectError:   redissub.ErrEmptyTopicName,
		},
		{
			name:          "nil handler",
			client:        func() goredis.UniversalClient { return valkeyClient.Client },
			consumerGroup: "test-group",
			topic:         "test-topic",
			handler:       nil,
			expectError:   redissub.ErrNilMessageHandler,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			subscriber, err := redissub.NewSubscriber(
				testCase.client(),
				testCase.consumerGroup,
				testCase.topic,
				testCase.handler,
			)

			if testCase.expectError != nil {
				require.ErrorIs(t, err, testCase.expectError)
				require.Nil(t, subscriber)
			} else {
				require.NoError(t, err)
				require.NotNil(t, subscriber)
			}
		})
	}
}

func TestSubscriberName(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"my-group",
		"my-topic",
		handler,
	)
	require.NoError(t, err)

	require.Equal(t, "redissub-my-group-my-topic", subscriber.Name())
}

func TestSubscriberTopic(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic",
		handler,
	)
	require.NoError(t, err)

	require.Equal(t, "test-topic", subscriber.Topic())
}

func TestSubscriberConsumerGroup(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic",
		handler,
	)
	require.NoError(t, err)

	require.Equal(t, "test-group", subscriber.ConsumerGroup())
}

func TestSubscriberIsHealthy(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-health",
		handler,
	)
	require.NoError(t, err)

	require.False(t, subscriber.IsHealthy(), "subscriber should not be healthy before start")
}

func TestSubscriberStart(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-startstop",
		handler,
	)
	require.NoError(t, err)

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})
	stopped := make(chan error)

	go func() {
		close(started)

		stopped <- subscriber.Start(startCtx)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	require.True(t, subscriber.IsHealthy(), "subscriber should be healthy after start")

	cancel()

	stoppedErr := <-stopped
	require.ErrorIs(t, stoppedErr, context.Canceled)

	require.False(t, subscriber.IsHealthy(), "subscriber should not be healthy after stop")
}

func TestSubscriberStop(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-stop",
		handler,
	)
	require.NoError(t, err)

	started := make(chan struct{})
	stopped := make(chan error, 1)

	go func() {
		close(started)

		stopped <- subscriber.Start(t.Context())
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	require.True(t, subscriber.IsHealthy(), "subscriber should be healthy after start")

	err = subscriber.Stop()
	require.NoError(t, err, "Stop() should not return an error")

	select {
	case stoppedErr := <-stopped:
		require.NoError(t, stoppedErr, "subscriber should stop without error (via shutdown signal)")
	case <-time.After(1 * time.Second):
		t.Fatal("subscriber didn't stop within timeout")
	}

	require.False(t, subscriber.IsHealthy(), "subscriber should not be healthy after stop")
}

func TestSubscriberProcessMessage(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	var processedCount atomic.Int32

	handler := func(_ context.Context, payload message.Payload) error {
		processedCount.Add(1)
		t.Logf("Processed message: %s", string(payload))

		return nil
	}

	topic := "test-topic-process-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "message-1")
	publishTestMessage(t, publisher, topic, "message-2")
	publishTestMessage(t, publisher, topic, "message-3")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return processedCount.Load() >= 3
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	require.GreaterOrEqual(t, processedCount.Load(), int32(3))
}

func TestSubscriberWithOptions(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-options",
		handler,
		redissub.WithBlockTime(5*time.Second),
		redissub.WithClaimInterval(10*time.Second),
		redissub.WithMaxIdleTime(30*time.Second),
	)
	require.NoError(t, err)
	require.NotNil(t, subscriber)
}

func TestSubscriberWithRetry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	var attemptCount atomic.Int32

	handler := func(_ context.Context, _ message.Payload) error {
		attempt := attemptCount.Add(1)
		if attempt < 3 {
			return errTemporary
		}

		return nil
	}

	topic := "test-topic-retry-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithRetry(3, 100*time.Millisecond, ""),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "retry-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return attemptCount.Load() >= 3
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	require.GreaterOrEqual(t, attemptCount.Load(), int32(3))
}

func TestSubscriberWithMetrics(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	metrics := &mockMetrics{}

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	topic := "test-topic-metrics-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithMetrics(metrics),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "metrics-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return metrics.receivedCount.Load() >= 1
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	require.GreaterOrEqual(t, metrics.receivedCount.Load(), int32(1))
	require.GreaterOrEqual(t, metrics.processedCount.Load(), int32(1))
	require.GreaterOrEqual(t, metrics.ackedCount.Load(), int32(1))
}

func TestSubscriberWithDLQ(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	handler := func(_ context.Context, _ message.Payload) error {
		return errAlwaysFail
	}

	topic := "test-topic-dlq-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	dlqTopic := topic + "-dlq"

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithRetry(2, 50*time.Millisecond, dlqTopic),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "dlq-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		length, dlqErr := valkeyClient.Client.XLen(ctx, dlqTopic).Result()

		return dlqErr == nil && length >= 1
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	length, err := valkeyClient.Client.XLen(ctx, dlqTopic).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, length, int64(1))
}

type mockMetrics struct {
	receivedCount  atomic.Int32
	processedCount atomic.Int32
	ackedCount     atomic.Int32
	nackedCount    atomic.Int32
	dlqCount       atomic.Int32
}

func (m *mockMetrics) MessageReceived(_ string) {
	m.receivedCount.Add(1)
}

func (m *mockMetrics) MessageProcessed(_ string, _ time.Duration, _ error) {
	m.processedCount.Add(1)
}

func (m *mockMetrics) MessageAcked(_ string) {
	m.ackedCount.Add(1)
}

func (m *mockMetrics) MessageNacked(_ string) {
	m.nackedCount.Add(1)
}

func (m *mockMetrics) MessageSentToDLQ(_ string) {
	m.dlqCount.Add(1)
}

func TestSubscriberStopWhenNotRunning(t *testing.T) {
	t.Parallel()

	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-stop-not-running",
		handler,
	)
	require.NoError(t, err)

	err = subscriber.Stop()
	require.NoError(t, err, "Stop on non-running subscriber should return nil")

	err = subscriber.Stop()
	require.NoError(t, err, "Multiple Stop calls should be safe")
}

func TestSubscriberStartAlreadyRunning(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		"test-topic-already-running",
		handler,
	)
	require.NoError(t, err)

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})
	stopped := make(chan error, 1)

	go func() {
		close(started)

		stopped <- subscriber.Start(startCtx)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	require.True(t, subscriber.IsHealthy(), "subscriber should be healthy after start")

	err = subscriber.Start(ctx)
	require.ErrorIs(t, err, redissub.ErrAlreadyRunning, "second Start call should return ErrAlreadyRunning")

	cancel()

	<-stopped
}

func TestSubscriberWithExecTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	var handlerStarted atomic.Bool

	var handlerCompleted atomic.Bool

	handler := func(ctx context.Context, _ message.Payload) error {
		handlerStarted.Store(true)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			handlerCompleted.Store(true)

			return nil
		}
	}

	topic := "test-topic-timeout-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithExecTimeout(200*time.Millisecond),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "timeout-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, handlerStarted.Load, 5*time.Second, 50*time.Millisecond, "handler should have started")

	time.Sleep(500 * time.Millisecond)

	require.False(t, handlerCompleted.Load(), "handler should not have completed due to timeout")

	cancel()
}

func TestSubscriberWithExecTimeoutAndRetry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	var attemptCount atomic.Int32

	handler := func(ctx context.Context, _ message.Payload) error {
		attempt := attemptCount.Add(1)
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		}

		return nil
	}

	topic := "test-topic-timeout-retry-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithExecTimeout(100*time.Millisecond),
		redissub.WithRetry(3, 50*time.Millisecond, ""),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "timeout-retry-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return attemptCount.Load() >= 3
	}, 10*time.Second, 50*time.Millisecond, "should have retried at least 3 times")

	cancel()

	require.GreaterOrEqual(t, attemptCount.Load(), int32(3))
}

func TestSubscriberWithExecTimeoutSuccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	var processedCount atomic.Int32

	handler := func(_ context.Context, _ message.Payload) error {
		time.Sleep(50 * time.Millisecond)
		processedCount.Add(1)

		return nil
	}

	topic := "test-topic-timeout-success-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithExecTimeout(1*time.Second),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "success-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return processedCount.Load() >= 1
	}, 5*time.Second, 50*time.Millisecond, "message should have been processed successfully")

	cancel()

	require.GreaterOrEqual(t, processedCount.Load(), int32(1))
}

func TestSubscriberWithExecTimeoutToDLQ(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	handler := func(ctx context.Context, _ message.Payload) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}

	topic := "test-topic-timeout-dlq-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	dlqTopic := topic + "-dlq"

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithExecTimeout(100*time.Millisecond),
		redissub.WithRetry(2, 50*time.Millisecond, dlqTopic),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "timeout-dlq-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		length, dlqErr := valkeyClient.Client.XLen(ctx, dlqTopic).Result()

		return dlqErr == nil && length >= 1
	}, 10*time.Second, 100*time.Millisecond, "message should have been sent to DLQ after timeout")

	cancel()

	length, err := valkeyClient.Client.XLen(ctx, dlqTopic).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, length, int64(1))
}

func TestSubscriberNoInfiniteRedeliveryAfterRetryExhaustion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	const maxRetries = 2
	expectedAttempts := maxRetries + 1

	var attemptCount atomic.Int32

	handler := func(_ context.Context, _ message.Payload) error {
		attemptCount.Add(1)

		return errAlwaysFail
	}

	topic := "test-topic-no-infinite-loop-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithRetry(maxRetries, 10*time.Millisecond, ""),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "test-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return attemptCount.Load() >= int32(expectedAttempts) //nolint:gosec
	}, 5*time.Second, 50*time.Millisecond, "should have attempted processing %d times", expectedAttempts)

	time.Sleep(500 * time.Millisecond)

	finalAttempts := attemptCount.Load()
	require.Equal(t, int32(expectedAttempts), finalAttempts, //nolint:gosec
		"message should only be processed %d times (initial + %d retries), got %d - indicates infinite redelivery bug",
		expectedAttempts, maxRetries, finalAttempts)

	cancel()
}

func TestSubscriberNoInfiniteRedeliveryWithDLQ(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	const maxRetries = 2
	expectedAttempts := maxRetries + 1

	var attemptCount atomic.Int32

	handler := func(_ context.Context, _ message.Payload) error {
		attemptCount.Add(1)

		return errAlwaysFail
	}

	topic := "test-topic-dlq-no-loop-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	dlqTopic := topic + "-dlq"

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithRetry(maxRetries, 10*time.Millisecond, dlqTopic),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "dlq-test-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		length, dlqErr := valkeyClient.Client.XLen(ctx, dlqTopic).Result()

		return dlqErr == nil && length >= 1
	}, 5*time.Second, 50*time.Millisecond, "message should have been sent to DLQ")

	time.Sleep(500 * time.Millisecond)

	finalAttempts := attemptCount.Load()
	require.Equal(t, int32(expectedAttempts), finalAttempts,
		"message should only be processed %d times before going to DLQ, got %d - indicates infinite redelivery bug",
		expectedAttempts, finalAttempts)

	length, err := valkeyClient.Client.XLen(ctx, dlqTopic).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), length, "should have exactly 1 message in DLQ")

	cancel()
}

func TestSubscriberMetricsAfterRetryExhaustion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupTestClient(t)
	publisher := setupTestPublisher(t, valkeyClient)

	const maxRetries = 2

	metrics := &mockMetrics{}

	handler := func(_ context.Context, _ message.Payload) error {
		return errAlwaysFail
	}

	topic := "test-topic-metrics-retry-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	dlqTopic := topic + "-dlq"

	subscriber, err := redissub.NewSubscriber(
		valkeyClient.Client,
		"test-group",
		topic,
		handler,
		redissub.WithRetry(maxRetries, 10*time.Millisecond, dlqTopic),
		redissub.WithMetrics(metrics),
	)
	require.NoError(t, err)

	publishTestMessage(t, publisher, topic, "metrics-retry-message")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = subscriber.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return metrics.dlqCount.Load() >= 1
	}, 5*time.Second, 50*time.Millisecond, "message should have been sent to DLQ")

	time.Sleep(300 * time.Millisecond)

	cancel()

	require.Equal(t, int32(1), metrics.receivedCount.Load(), "should receive message exactly once")
	require.Equal(t, int32(1), metrics.processedCount.Load(), "should record processed exactly once")
	require.Equal(t, int32(1), metrics.dlqCount.Load(), "should send to DLQ exactly once")
	require.Equal(t, int32(1), metrics.nackedCount.Load(), "should record nacked exactly once (for failed processing)")
	require.Equal(t, int32(0), metrics.ackedCount.Load(), "should not record acked (message failed)")
}
