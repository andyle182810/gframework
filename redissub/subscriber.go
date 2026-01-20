package redissub

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	defaultShutdownTimeout = 5 * time.Second
	defaultExecTimeout     = 30 * time.Second
)

var (
	ErrNilRedisClient           = errors.New("subscriber: redis client cannot be nil")
	ErrEmptyConsumerGroup       = errors.New("subscriber: consumer group name cannot be empty")
	ErrEmptyTopicName           = errors.New("subscriber: topic name cannot be empty")
	ErrNilMessageHandler        = errors.New("subscriber: message handler cannot be nil")
	ErrMessageHandlerNotDefined = errors.New("subscriber: message handler is not defined")
	ErrMaxRetriesExceeded       = errors.New("subscriber: max retries exceeded")
	ErrExecTimeout              = errors.New("subscriber: message handler execution timed out")
	ErrAlreadyRunning           = errors.New("subscriber: already running")
)

type MessageHandler func(ctx context.Context, payload message.Payload) error

type Metrics interface {
	MessageReceived(topic string)
	MessageProcessed(topic string, duration time.Duration, err error)
	MessageAcked(topic string)
	MessageNacked(topic string)
	MessageSentToDLQ(topic string)
}

type RetryConfig struct {
	MaxRetries int           // Maximum number of retries (0 = no retries)
	RetryDelay time.Duration // Delay between retries
	DLQTopic   string        // Dead letter queue topic (empty = no DLQ)
}

type SubscriberConfig struct {
	BlockTime       time.Duration // How long to block waiting for messages
	ClaimInterval   time.Duration // Interval for claiming pending messages
	MaxIdleTime     time.Duration // Max idle time before message can be claimed
	ShutdownTimeout time.Duration // Maximum time to wait for graceful shutdown before force closing
	ExecTimeout     time.Duration // Maximum time allowed for message handler execution
	Metrics         Metrics
	Retry           *RetryConfig
}

type SubscriberOption func(*SubscriberConfig)

func WithBlockTime(d time.Duration) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.BlockTime = d
	}
}

func WithClaimInterval(d time.Duration) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.ClaimInterval = d
	}
}

func WithMaxIdleTime(d time.Duration) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.MaxIdleTime = d
	}
}

func WithMetrics(m Metrics) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.Metrics = m
	}
}

func WithRetry(maxRetries int, retryDelay time.Duration, dlqTopic string) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.Retry = &RetryConfig{
			MaxRetries: maxRetries,
			RetryDelay: retryDelay,
			DLQTopic:   dlqTopic,
		}
	}
}

func WithShutdownTimeout(d time.Duration) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.ShutdownTimeout = d
	}
}

func WithExecTimeout(d time.Duration) SubscriberOption {
	return func(c *SubscriberConfig) {
		c.ExecTimeout = d
	}
}

type Subscriber struct {
	*redisstream.Subscriber
	name           string
	topic          string
	consumerGroup  string
	shutdownSignal chan struct{}
	stoppedSignal  chan struct{}
	messageHandler MessageHandler
	config         SubscriberConfig
	healthy        atomic.Bool
	running        atomic.Bool
	redisClient    goredis.UniversalClient
}

func NewSubscriber(
	redisClient goredis.UniversalClient,
	consumerGroup,
	topic string,
	messageHandler MessageHandler,
	opts ...SubscriberOption,
) (*Subscriber, error) {
	if redisClient == nil {
		return nil, ErrNilRedisClient
	}

	if consumerGroup == "" {
		return nil, ErrEmptyConsumerGroup
	}

	if topic == "" {
		return nil, ErrEmptyTopicName
	}

	if messageHandler == nil {
		return nil, ErrNilMessageHandler
	}

	config := defaultSubscriberConfig()

	for _, opt := range opts {
		opt(&config)
	}

	//nolint:exhaustruct
	redisSubscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:        redisClient,
			Unmarshaller:  redisstream.DefaultMarshallerUnmarshaller{},
			ConsumerGroup: consumerGroup,
			BlockTime:     config.BlockTime,
			ClaimInterval: config.ClaimInterval,
			MaxIdleTime:   config.MaxIdleTime,
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis subscriber: %w", err)
	}

	//nolint:exhaustruct
	sub := &Subscriber{
		name:           fmt.Sprintf("redissub-%s-%s", consumerGroup, topic),
		topic:          topic,
		consumerGroup:  consumerGroup,
		Subscriber:     redisSubscriber,
		messageHandler: messageHandler,
		shutdownSignal: make(chan struct{}),
		stoppedSignal:  make(chan struct{}),
		config:         config,
		redisClient:    redisClient,
	}
	sub.healthy.Store(false)

	return sub, nil
}

func defaultSubscriberConfig() SubscriberConfig {
	return SubscriberConfig{
		BlockTime:       0,
		ClaimInterval:   0,
		MaxIdleTime:     0,
		ShutdownTimeout: defaultShutdownTimeout,
		ExecTimeout:     defaultExecTimeout,
		Metrics:         nil,
		Retry:           nil,
	}
}

func (s *Subscriber) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return nil
	}

	log.Info().
		Str("topic", s.topic).
		Str("consumer_group", s.consumerGroup).
		Msg("Stopping subscriber")

	s.healthy.Store(false)

	if s.shutdownSignal != nil {
		close(s.shutdownSignal)
	}

	select {
	case <-s.stoppedSignal:
		// Good, subscriber stopped cleanly
	case <-time.After(s.config.ShutdownTimeout):
		log.Error().
			Str("topic", s.topic).
			Str("consumer_group", s.consumerGroup).
			Dur("timeout", s.config.ShutdownTimeout).
			Msg("Timeout waiting for subscriber to stop")
	}

	log.Info().
		Str("topic", s.topic).
		Str("consumer_group", s.consumerGroup).
		Msg("Subscriber stopped")

	return nil
}

func (s *Subscriber) Name() string {
	return s.name
}

func (s *Subscriber) IsHealthy() bool {
	return s.healthy.Load()
}

func (s *Subscriber) ConsumerGroup() string {
	return s.consumerGroup
}

func (s *Subscriber) Topic() string {
	return s.topic
}

func (s *Subscriber) Start(ctx context.Context) error { //nolint:cyclop
	if !s.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}

	defer close(s.stoppedSignal)

	log.Info().
		Str("topic", s.Topic()).
		Msg("The subscription is being started")

	msgChan, err := s.Subscriber.Subscribe(ctx, s.Topic())
	if err != nil {
		return fmt.Errorf("subscription to topic %s failed: %w", s.Topic(), err)
	}

	s.healthy.Store(true)

	for {
		select {
		case <-ctx.Done():
			s.healthy.Store(false)
			log.Info().
				Str("topic", s.Topic()).
				Msg("The subscription has been stopped due to context cancellation")

			return ctx.Err()
		case <-s.shutdownSignal:
			s.healthy.Store(false)
			log.Info().
				Str("topic", s.Topic()).
				Msg("The subscription has been stopped")

			return nil
		case msg := <-msgChan:
			if msg == nil || msg.UUID == "" {
				log.Debug().
					Str("topic", s.Topic()).
					Msg("An empty message has been received")

				continue
			}

			if s.config.Metrics != nil {
				s.config.Metrics.MessageReceived(s.Topic())
			}

			if err := s.handleMessage(ctx, msg); err != nil {
				log.Error().
					Err(err).
					Str("topic", s.Topic()).
					Str("message_id", msg.UUID).
					Msg("The message processing has failed")
			}
		}
	}
}

func (s *Subscriber) handleMessage(ctx context.Context, msg *message.Message) error {
	if s.messageHandler == nil {
		return ErrMessageHandlerNotDefined
	}

	start := time.Now()
	processingErr := s.processWithRetry(ctx, msg)
	duration := time.Since(start)

	if s.config.Metrics != nil {
		s.config.Metrics.MessageProcessed(s.Topic(), duration, processingErr)
	}

	if processingErr != nil {
		s.handleFailedMessage(ctx, msg, processingErr)

		return fmt.Errorf("%w: %w", ErrMaxRetriesExceeded, processingErr)
	}

	s.acknowledgeMessage(msg)

	return nil
}

func (s *Subscriber) processWithRetry(ctx context.Context, msg *message.Message) error {
	maxAttempts := 1
	if s.config.Retry != nil && s.config.Retry.MaxRetries > 0 {
		maxAttempts = s.config.Retry.MaxRetries + 1
	}

	var processingErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		processingErr = s.executeWithTimeout(ctx, msg)
		if processingErr == nil {
			return nil
		}

		if attempt < maxAttempts {
			log.Warn().
				Err(processingErr).
				Str("message_id", msg.UUID).
				Int("attempt", attempt).
				Int("max_attempts", maxAttempts).
				Msg("Message processing failed, retrying")

			if err := s.waitForRetry(ctx); err != nil {
				return err
			}
		}
	}

	return processingErr
}

func (s *Subscriber) executeWithTimeout(ctx context.Context, msg *message.Message) error {
	if s.config.ExecTimeout <= 0 {
		return s.messageHandler(ctx, msg.Payload)
	}

	execCtx, cancel := context.WithTimeout(ctx, s.config.ExecTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- s.messageHandler(execCtx, msg.Payload)
	}()

	select {
	case err := <-done:
		return err
	case <-execCtx.Done():
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: exceeded %v", ErrExecTimeout, s.config.ExecTimeout)
		}

		return execCtx.Err()
	}
}

func (s *Subscriber) waitForRetry(ctx context.Context) error {
	if s.config.Retry == nil || s.config.Retry.RetryDelay <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.config.Retry.RetryDelay):
		return nil
	}
}

func (s *Subscriber) handleFailedMessage(ctx context.Context, msg *message.Message, processingErr error) {
	if s.config.Retry != nil && s.config.Retry.DLQTopic != "" {
		if dlqErr := s.sendToDLQ(ctx, msg, processingErr); dlqErr != nil {
			log.Error().
				Err(dlqErr).
				Str("message_id", msg.UUID).
				Msg("Failed to send message to DLQ")
		} else if s.config.Metrics != nil {
			s.config.Metrics.MessageSentToDLQ(s.Topic())
		}
	}

	msg.Nack()

	if s.config.Metrics != nil {
		s.config.Metrics.MessageNacked(s.Topic())
	}
}

func (s *Subscriber) acknowledgeMessage(msg *message.Message) {
	if !msg.Ack() {
		log.Debug().
			Str("message_id", msg.UUID).
			Msg("The message has already been acknowledged")

		return
	}

	log.Debug().
		Str("message_id", msg.UUID).
		Msg("The message has been acknowledged successfully")

	if s.config.Metrics != nil {
		s.config.Metrics.MessageAcked(s.Topic())
	}
}

func (s *Subscriber) sendToDLQ(ctx context.Context, msg *message.Message, processingErr error) error {
	if s.config.Retry == nil || s.config.Retry.DLQTopic == "" {
		return nil
	}

	//nolint:exhaustruct
	return s.redisClient.XAdd(ctx, &goredis.XAddArgs{
		Stream: s.config.Retry.DLQTopic,
		Values: map[string]any{
			"uuid":           msg.UUID,
			"payload":        string(msg.Payload),
			"original_topic": s.topic,
			"consumer_group": s.consumerGroup,
			"error":          processingErr.Error(),
			"failed_at":      time.Now().UTC().Format(time.RFC3339),
		},
	}).Err()
}
