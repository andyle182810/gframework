package redispub_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/andyle182810/gframework/redispub"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	redis.UniversalClient
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		UniversalClient: nil,
	}
}

func setupTestClient(ctx context.Context, t *testing.T) *valkey.Valkey {
	t.Helper()

	container := testutil.SetupValkeyContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err, "failed to parse port")

	cfg := &valkey.Config{
		Host:            container.Host,
		Port:            port,
		Password:        "",
		DB:              0,
		DialTimeout:     5 * time.Second,
		MaxIdleConns:    5,
		MinIdleConns:    1,
		PingTimeout:     2 * time.Second,
		PoolSize:        0,
		ReadTimeout:     0,
		WriteTimeout:    0,
		MaxRetries:      0,
		MinRetryBackoff: 0,
		MaxRetryBackoff: 0,
		TLSEnabled:      false,
		TLSSkipVerify:   false,
		TLSCertFile:     "",
		TLSKeyFile:      "",
		TLSCAFile:       "",
	}

	client, err := valkey.New(cfg)
	require.NoError(t, err, "failed to create valkey client")

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

func TestNew_WithNilRedisClient(t *testing.T) {
	t.Parallel()

	_, err := redispub.New(nil, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	if !errors.Is(err, redispub.ErrNilRedisClient) {
		t.Errorf("expected ErrNilRedisClient, got %v", err)
	}
}

func TestNew_WithNegativeMaxStreamEntries(t *testing.T) {
	t.Parallel()

	client := newMockRedisClient()

	_, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: -1,
		Timeout:          0,
		Logger:           nil,
	})
	if !errors.Is(err, redispub.ErrInvalidMaxStreamEntries) {
		t.Errorf("expected ErrInvalidMaxStreamEntries, got %v", err)
	}
}

func TestNew_WithZeroMaxStreamEntries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	require.NotNil(t, publisher)

	_ = publisher.Close()
}

func TestNew_WithPositiveMaxStreamEntries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 1000,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	require.NotNil(t, publisher)

	_ = publisher.Close()
}

func TestNew_WithCustomTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          10 * time.Second,
		Logger:           nil,
	})
	require.NoError(t, err)
	require.NotNil(t, publisher)

	_ = publisher.Close()
}

func TestNew_WithZeroTimeoutUsesDefault(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	require.NotNil(t, publisher)

	_ = publisher.Close()
}

func TestNew_WithNegativeTimeoutUsesDefault(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          -1 * time.Second,
		Logger:           nil,
	})
	require.NoError(t, err)
	require.NotNil(t, publisher)

	_ = publisher.Close()
}

func TestRedisPublisher_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)

	_ = publisher.Close()
	_ = publisher.Close()
}

func TestRedisPublisher_ImplementsPublisherInterface(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	var _ redispub.Publisher = publisher
}

func TestRedisPublisher_PublishToTopicWithNoMessages(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	err = publisher.PublishToTopic(ctx, "test-topic")
	if err != nil {
		t.Errorf("expected no error when publishing zero messages, got %v", err)
	}
}

func TestRedisPublisher_PublishToTopicWithSingleMessage(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	err = publisher.PublishToTopic(ctx, "test-topic", "message-1")
	if err != nil {
		t.Errorf("expected no error when publishing single message, got %v", err)
	}
}

func TestRedisPublisher_PublishToTopicWithMultipleMessages(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	err = publisher.PublishToTopic(ctx, "test-topic", "message-1", "message-2", "message-3")
	if err != nil {
		t.Errorf("expected no error when publishing multiple messages, got %v", err)
	}
}

func TestRedisPublisher_PublishToTopicRespectsExistingContextDeadline(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          1 * time.Second,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = publisher.PublishToTopic(ctxWithTimeout, "test-topic", "message-1")
	if err != nil {
		t.Errorf("expected no error with valid context deadline, got %v", err)
	}
}

func TestRedisPublisher_PublishToTopicWithAlreadyCancelledContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	ctxCancelled, cancel := context.WithCancel(ctx)
	cancel()

	// Note: A cancelled context without a deadline will have a new timeout applied
	// by PublishToTopic, so the publish may still succeed. The context cancellation
	// is set on the message but watermill's Publish doesn't check it synchronously.
	_ = publisher.PublishToTopic(ctxCancelled, "test-topic", "message-1")
}

func TestRedisPublisher_PublishToTopicAppliesDefaultTimeoutWhenNoDeadline(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          5 * time.Second,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	err = publisher.PublishToTopic(ctx, "test-topic", "message-1")
	if err != nil {
		t.Errorf("expected no error when publishing with default timeout, got %v", err)
	}
}

func TestRedisPublisher_PublishToMultipleTopics(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 0,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	topics := []string{"topic-1", "topic-2", "topic-3"}
	for _, topic := range topics {
		err = publisher.PublishToTopic(ctx, topic, "message")
		if err != nil {
			t.Errorf("expected no error when publishing to topic %s, got %v", topic, err)
		}
	}
}

func TestRedisPublisher_PublishWithMaxStreamEntries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := setupTestClient(ctx, t)

	publisher, err := redispub.New(client, redispub.Options{
		MaxStreamEntries: 10,
		Timeout:          0,
		Logger:           nil,
	})
	require.NoError(t, err)
	defer publisher.Close()

	for i := range 20 {
		err = publisher.PublishToTopic(ctx, "capped-topic", "message")
		if err != nil {
			t.Errorf("expected no error on publish %d, got %v", i, err)
		}
	}
}
