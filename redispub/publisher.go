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
	ErrPublisherInitialization = errors.New("failed to initialize Redis stream publisher")
	ErrPublishFailed           = errors.New("failed to publish messages")
	ErrStreamTrimFailed        = errors.New("failed to trim stream")
)

type Options struct {
	MaxStreamEntries int64
}

type RedisPublisher struct {
	redisStreamPublisher *redisstream.Publisher
	redisClient          goredis.UniversalClient
	maxStreamEntries     int64
}

func New(redisClient goredis.UniversalClient, opts Options) (*RedisPublisher, error) {
	redisStreamPublisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:        redisClient,
			Marshaller:    redisstream.DefaultMarshallerUnmarshaller{},
			Maxlens:       map[string]int64{},
			DefaultMaxlen: 0,
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPublisherInitialization, err)
	}

	return &RedisPublisher{
		redisStreamPublisher: redisStreamPublisher,
		redisClient:          redisClient,
		maxStreamEntries:     opts.MaxStreamEntries,
	}, nil
}

func (p *RedisPublisher) PublishToTopic(topic string, messageContents ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultPublishTimeout)
	defer cancel()

	messages := make([]*message.Message, 0, len(messageContents))

	for _, content := range messageContents {
		msg := message.NewMessage(watermill.NewUUID(), []byte(content))
		messages = append(messages, msg)
	}

	if err := p.redisStreamPublisher.Publish(topic, messages...); err != nil {
		return fmt.Errorf("%w to topic %s: %w", ErrPublishFailed, topic, err)
	}

	if p.maxStreamEntries > 0 {
		if err := p.redisClient.XTrimMaxLen(ctx, topic, p.maxStreamEntries).Err(); err != nil {
			return fmt.Errorf("%w for topic %s: %w", ErrStreamTrimFailed, topic, err)
		}
	}

	return nil
}
