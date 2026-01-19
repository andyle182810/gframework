package redispub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultPublishTimeout = 5 * time.Second
)

var (
	ErrPublisherInitialization = errors.New("publisher: failed to initialize redis stream publisher")
	ErrPublishFailed           = errors.New("publisher: failed to publish messages")
	ErrNilRedisClient          = errors.New("publisher: redis client is required")
	ErrInvalidMaxStreamEntries = errors.New("publisher: maxStreamEntries cannot be negative")
)

type Publisher interface {
	PublishToTopic(ctx context.Context, topic string, messageContents ...string) error
	Close() error
}

type Options struct {
	MaxStreamEntries int64 ``
	Timeout          time.Duration
	Logger           watermill.LoggerAdapter
}

type RedisPublisher struct {
	publisher *redisstream.Publisher
	timeout   time.Duration
}

var _ Publisher = (*RedisPublisher)(nil)

func New(redisClient goredis.UniversalClient, opts Options) (*RedisPublisher, error) {
	if redisClient == nil {
		return nil, ErrNilRedisClient
	}

	if opts.MaxStreamEntries < 0 {
		return nil, ErrInvalidMaxStreamEntries
	}

	publisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:        redisClient,
			Marshaller:    redisstream.DefaultMarshallerUnmarshaller{},
			Maxlens:       map[string]int64{},
			DefaultMaxlen: opts.MaxStreamEntries,
		},
		opts.Logger,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPublisherInitialization, err)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultPublishTimeout
	}

	return &RedisPublisher{
		publisher: publisher,
		timeout:   timeout,
	}, nil
}

func (p *RedisPublisher) PublishToTopic(ctx context.Context, topic string, messageContents ...string) error {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	messages := make([]*message.Message, 0, len(messageContents))

	for _, content := range messageContents {
		msg := message.NewMessage(watermill.NewUUID(), []byte(content))
		msg.SetContext(ctx)
		messages = append(messages, msg)
	}

	if err := p.publisher.Publish(topic, messages...); err != nil {
		return fmt.Errorf("%w to topic %s: %w", ErrPublishFailed, topic, err)
	}

	return nil
}

func (p *RedisPublisher) Close() error {
	return p.publisher.Close()
}
