package redissub

import (
	"context"
	"errors"
	"fmt"

	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	ErrNilRedisClient           = errors.New("redis client cannot be nil")
	ErrEmptyConsumerGroup       = errors.New("consumer group name cannot be empty")
	ErrEmptyTopicName           = errors.New("topic name cannot be empty")
	ErrNilMessageHandler        = errors.New("message handler cannot be nil")
	ErrMessageHandlerNotDefined = errors.New("message handler is not defined")
)

type MessageHandler func(ctx context.Context, payload message.Payload) error

type Subscriber struct {
	*redisstream.Subscriber
	topic          string
	consumerGroup  string
	shutdownSignal chan struct{} // Channel to signal shutdown
	messageHandler MessageHandler
}

func NewSubscriber(
	redisClient goredis.UniversalClient,
	consumerGroup,
	topic string,
	messageHandler MessageHandler,
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

	redisSubscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:                    redisClient,
			Unmarshaller:              redisstream.DefaultMarshallerUnmarshaller{},
			ConsumerGroup:             consumerGroup,
			Consumer:                  "",
			NackResendSleep:           0,
			BlockTime:                 0,
			ClaimInterval:             0,
			ClaimBatchSize:            0,
			MaxIdleTime:               0,
			CheckConsumersInterval:    0,
			ConsumerTimeout:           0,
			OldestId:                  "",
			FanOutOldestId:            "",
			ShouldClaimPendingMessage: nil,
			ShouldStopOnReadErrors:    nil,
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis subscriber: %w", err)
	}

	return &Subscriber{
		topic:          topic,
		consumerGroup:  consumerGroup,
		Subscriber:     redisSubscriber,
		messageHandler: messageHandler,
		shutdownSignal: make(chan struct{}), // Initialize the shutdown signal channel
	}, nil
}

func (s *Subscriber) Close() error {
	if s.shutdownSignal != nil {
		close(s.shutdownSignal) // Signal shutdown
	}

	return nil
}

func (s *Subscriber) ConsumerGroup() string {
	return s.consumerGroup
}

func (s *Subscriber) Topic() string {
	return s.topic
}

func (s *Subscriber) Start() {
	log.Info().
		Str("topic", s.Topic()).
		Msg("The subscription is being started")

	msgChan, err := s.Subscriber.Subscribe(context.Background(), s.Topic())
	if err != nil {
		log.Panic().
			Err(err).
			Str("topic", s.Topic()).
			Msg("The subscription to the topic has failed")

		return
	}

	for {
		select {
		case <-s.shutdownSignal:
			log.Info().
				Str("topic", s.Topic()).
				Msg("The subscription has been stopped")

			return
		case msg := <-msgChan:
			if msg == nil || msg.UUID == "" {
				log.Debug().
					Str("topic", s.Topic()).
					Msg("An empty message has been received")

				continue
			}

			if err := s.handleMessage(context.Background(), msg); err != nil {
				log.Error().
					Err(err).
					Str("topic", s.Topic()).
					Str("message_id", msg.UUID).
					Msg("The message processing has failed")
			}
		}
	}
}

// handleMessage processes a single message using the provided message handler.
func (s *Subscriber) handleMessage(ctx context.Context, msg *message.Message) error {
	if s.messageHandler == nil {
		return ErrMessageHandlerNotDefined
	}

	// Process the message payload
	if err := s.messageHandler(ctx, msg.Payload); err != nil {
		return fmt.Errorf("message handler failed: %w", err)
	}

	// Acknowledge the message
	if !msg.Ack() {
		log.Debug().
			Str("message_id", msg.UUID).
			Msg("The message has already been acknowledged")
	} else {
		log.Debug().
			Str("message_id", msg.UUID).
			Msg("The message has been acknowledged successfully")
	}

	return nil
}
