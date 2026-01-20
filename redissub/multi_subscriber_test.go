//nolint:exhaustruct,paralleltest,tparallel
package redissub_test

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/andyle182810/gframework/redispub"
	"github.com/andyle182810/gframework/redissub"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/stretchr/testify/require"
)

func setupMultiTestClient(t *testing.T) *valkey.Valkey {
	t.Helper()

	ctx := t.Context()
	container := testutil.SetupValkeyContainer(ctx, t)

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

func setupMultiTestPublisher(t *testing.T, client *valkey.Valkey) *redispub.RedisPublisher {
	t.Helper()

	publisher, err := redispub.New(client.Client, redispub.Options{})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = publisher.Close()
	})

	return publisher
}

func publishMultiTestMessage(
	t *testing.T,
	publisher *redispub.RedisPublisher,
	topic, payload string,
) {
	t.Helper()

	ctx := t.Context()

	err := publisher.PublishToTopic(ctx, topic, payload)
	require.NoError(t, err)
}

func TestNewMultiSubscriber(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	require.NotNil(t, multiSub)
	require.Equal(t, "test-multi-sub", multiSub.Name())
	require.Equal(t, 0, multiSub.SubscriberCount())
}

func TestMultiSubscriberSubscribe(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	tests := []struct {
		name        string
		topic       string
		handler     redissub.MessageHandler
		expectError error
	}{
		{
			name:        "valid subscription",
			topic:       "test-topic",
			handler:     handler,
			expectError: nil,
		},
		{
			name:        "empty topic",
			topic:       "",
			handler:     handler,
			expectError: redissub.ErrEmptyTopic,
		},
		{
			name:        "nil handler",
			topic:       "test-topic",
			handler:     nil,
			expectError: redissub.ErrNilMessageHandler,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			multiSub := redissub.NewMultiSubscriber(
				"test-multi-sub",
				valkeyClient.Client,
				"test-group",
			)

			err := multiSub.Subscribe(testCase.topic, testCase.handler)

			if testCase.expectError != nil {
				require.ErrorIs(t, err, testCase.expectError)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, multiSub.SubscriberCount())
			}
		})
	}
}

func TestMultiSubscriberName(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	multiSub := redissub.NewMultiSubscriber(
		"my-multi-subscriber",
		valkeyClient.Client,
		"test-group",
	)

	require.Equal(t, "my-multi-subscriber", multiSub.Name())
}

func TestMultiSubscriberSubscriberCount(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	require.Equal(t, 0, multiSub.SubscriberCount())

	err := multiSub.Subscribe("topic-1", handler)
	require.NoError(t, err)
	require.Equal(t, 1, multiSub.SubscriberCount())

	err = multiSub.Subscribe("topic-2", handler)
	require.NoError(t, err)
	require.Equal(t, 2, multiSub.SubscriberCount())

	err = multiSub.Subscribe("topic-3", handler)
	require.NoError(t, err)
	require.Equal(t, 3, multiSub.SubscriberCount())
}

func TestMultiSubscriberIsHealthy(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	require.False(t, multiSub.IsHealthy(), "multi subscriber should not be healthy before start")

	err := multiSub.Subscribe("test-topic-health", handler)
	require.NoError(t, err)

	require.False(t, multiSub.IsHealthy(), "multi subscriber should not be healthy before start")
}

func TestMultiSubscriberStart(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	err := multiSub.Subscribe("test-topic-startstop", handler)
	require.NoError(t, err)

	started := make(chan struct{})
	stopped := make(chan error)

	go func() {
		close(started)
		stopped <- multiSub.Start(t.Context())
	}()

	<-started
	time.Sleep(200 * time.Millisecond)

	require.True(t, multiSub.IsHealthy(), "multi subscriber should be healthy after start")

	err = <-stopped
	require.NoError(t, err)

	err = multiSub.Stop()
	require.NoError(t, err)

	require.False(t, multiSub.IsHealthy(), "multi subscriber should not be healthy after stop")
}

func TestMultiSubscriberStartWithNoSubscribers(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	startCtx, cancel := context.WithCancel(t.Context())

	started := make(chan struct{})
	stopped := make(chan error)

	go func() {
		close(started)
		stopped <- multiSub.Start(startCtx)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	cancel()

	err := <-stopped
	require.NoError(t, err)
}

func TestMultiSubscriberStop(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	err := multiSub.Subscribe("test-topic-stop", handler)
	require.NoError(t, err)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = multiSub.Start(t.Context())
	}()

	<-started
	time.Sleep(200 * time.Millisecond)

	err = multiSub.Stop()
	require.NoError(t, err)

	require.False(t, multiSub.IsHealthy())
}

func TestMultiSubscriberProcessMessages(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupMultiTestClient(t)
	publisher := setupMultiTestPublisher(t, valkeyClient)

	var topic1Count atomic.Int32

	var topic2Count atomic.Int32

	topic1 := "test-topic-multi-1-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	topic2 := "test-topic-multi-2-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	handler1 := func(_ context.Context, _ message.Payload) error {
		topic1Count.Add(1)

		return nil
	}

	handler2 := func(_ context.Context, _ message.Payload) error {
		topic2Count.Add(1)

		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	err := multiSub.Subscribe(topic1, handler1)
	require.NoError(t, err)

	err = multiSub.Subscribe(topic2, handler2)
	require.NoError(t, err)

	publishMultiTestMessage(t, publisher, topic1, "message-1")
	publishMultiTestMessage(t, publisher, topic1, "message-2")
	publishMultiTestMessage(t, publisher, topic2, "message-3")

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = multiSub.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return topic1Count.Load() >= 2 && topic2Count.Load() >= 1
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	require.GreaterOrEqual(t, topic1Count.Load(), int32(2))
	require.GreaterOrEqual(t, topic2Count.Load(), int32(1))
}

func TestMultiSubscriberWithOptions(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
		redissub.WithBlockTime(5*time.Second),
		redissub.WithClaimInterval(10*time.Second),
		redissub.WithMaxIdleTime(30*time.Second),
	)

	err := multiSub.Subscribe("test-topic-options", handler)
	require.NoError(t, err)
	require.NotNil(t, multiSub)
}

func TestMultiSubscriberConcurrentProcessing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupMultiTestClient(t)
	publisher := setupMultiTestPublisher(t, valkeyClient)

	var totalProcessed atomic.Int32

	topics := make([]string, 3)
	for i := range 3 {
		topics[i] = "test-topic-concurrent-" + strconv.Itoa(i) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	handler := func(_ context.Context, _ message.Payload) error {
		totalProcessed.Add(1)

		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	for _, topic := range topics {
		err := multiSub.Subscribe(topic, handler)
		require.NoError(t, err)
	}

	for _, topic := range topics {
		for i := range 5 {
			publishMultiTestMessage(t, publisher, topic, "message-"+strconv.Itoa(i))
		}
	}

	startCtx, cancel := context.WithCancel(ctx)

	started := make(chan struct{})

	go func() {
		close(started)

		_ = multiSub.Start(startCtx)
	}()

	<-started

	require.Eventually(t, func() bool {
		return totalProcessed.Load() >= 15
	}, 30*time.Second, 100*time.Millisecond)

	cancel()

	require.GreaterOrEqual(t, totalProcessed.Load(), int32(15))
}

func TestMultiSubscriberStopWhenNotRunning(t *testing.T) {
	t.Parallel()

	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	err := multiSub.Subscribe("test-topic-stop-not-running", handler)
	require.NoError(t, err)

	err = multiSub.Stop()
	require.NoError(t, err, "Stop on non-running multi subscriber should return nil")

	err = multiSub.Stop()
	require.NoError(t, err, "Multiple Stop calls should be safe")
}

func TestMultiSubscriberStartAlreadyRunning(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	valkeyClient := setupMultiTestClient(t)

	handler := func(_ context.Context, _ message.Payload) error {
		return nil
	}

	multiSub := redissub.NewMultiSubscriber(
		"test-multi-sub",
		valkeyClient.Client,
		"test-group",
	)

	err := multiSub.Subscribe("test-topic-already-running", handler)
	require.NoError(t, err)

	started := make(chan struct{})
	stopped := make(chan error, 1)

	go func() {
		close(started)

		stopped <- multiSub.Start(t.Context())
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	require.True(t, multiSub.IsHealthy(), "multi subscriber should be healthy after start")

	err = multiSub.Start(ctx)
	require.ErrorIs(
		t,
		err,
		redissub.ErrMultiSubscriberAlreadyRunning,
		"second Start call should return ErrMultiSubscriberAlreadyRunning",
	)

	<-stopped
}
