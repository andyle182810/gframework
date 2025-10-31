package redissub

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/andyle182810/gframework/notifylog"
	goredis "github.com/redis/go-redis/v9"
)

var (
	ErrEmptyTopic         = errors.New("topic cannot be empty")
	ErrSubscriberCreation = errors.New("failed to create subscriber")
	ErrClosingSubcribers  = errors.New("errors while closing subscribers")
)

type MultiSubscriber struct {
	redisClient    goredis.UniversalClient
	consumerGroup  string
	subscribers    []*Subscriber
	logger         notifylog.NotifyLog
	waitGroup      sync.WaitGroup // WaitGroup to wait for all subscribers to stop
	subscribersMux sync.Mutex     // Mutex to protect the subscribers slice
}

func NewMultiSubscriber(redisClient goredis.UniversalClient, consumerGroup string) *MultiSubscriber {
	return &MultiSubscriber{
		redisClient:    redisClient,
		consumerGroup:  consumerGroup,
		subscribers:    make([]*Subscriber, 0),
		logger:         notifylog.New("multisub", notifylog.JSON),
		waitGroup:      sync.WaitGroup{},
		subscribersMux: sync.Mutex{},
	}
}

func (m *MultiSubscriber) Subscribe(topic string, messageHandler MessageHandler) error {
	if topic == "" {
		return ErrEmptyTopic
	}

	if messageHandler == nil {
		return ErrNilMessageHandler
	}

	subscriber, err := NewSubscriber(m.redisClient, m.consumerGroup, topic, messageHandler)
	if err != nil {
		return fmt.Errorf("%w for topic %s: %w", ErrSubscriberCreation, topic, err)
	}

	m.subscribersMux.Lock()
	m.subscribers = append(m.subscribers, subscriber)
	m.subscribersMux.Unlock()

	m.waitGroup.Add(1)

	go func() {
		defer m.waitGroup.Done()
		subscriber.Start()
	}()

	m.logger.Info().Str("topic", topic).Msg("Successfully subscribed to topic")

	return nil
}

func (m *MultiSubscriber) Close() error {
	m.logger.Info().Msg("Initiating shutdown of all subscribers")

	// Collect errors during closing
	var errorMessages []string

	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	for _, subscriber := range m.subscribers {
		if err := subscriber.Close(); err != nil {
			errorMessages = append(
				errorMessages,
				fmt.Sprintf("[%s/%s: %v]", subscriber.ConsumerGroup(), subscriber.Topic(), err),
			)

			m.logger.Error().Err(err).Str("topic", subscriber.Topic()).Msg("Failed to close subscriber")
		} else {
			m.logger.Info().Str("topic", subscriber.Topic()).Msg("Subscriber closed successfully")
		}
	}

	// Wait for all subscribers to stop
	m.waitGroup.Wait()

	if len(errorMessages) > 0 {
		return fmt.Errorf("%w: %s", ErrClosingSubcribers, strings.Join(errorMessages, ", "))
	}

	m.logger.Info().Msg("All subscribers shut down successfully")

	return nil
}
