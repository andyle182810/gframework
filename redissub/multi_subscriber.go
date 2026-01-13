package redissub

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
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
	waitGroup      sync.WaitGroup // WaitGroup to wait for all subscribers to stop
	subscribersMux sync.Mutex     // Mutex to protect the subscribers slice
}

func NewMultiSubscriber(redisClient goredis.UniversalClient, consumerGroup string) *MultiSubscriber {
	return &MultiSubscriber{
		redisClient:    redisClient,
		consumerGroup:  consumerGroup,
		subscribers:    make([]*Subscriber, 0),
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

	log.Info().
		Str("topic", topic).
		Msg("The subscription to the topic has been completed successfully.")

	return nil
}

func (m *MultiSubscriber) Close() error {
	log.Info().
		Msg("The shutdown of all subscribers is being initiated.")

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

			log.Error().
				Err(err).
				Str("topic", subscriber.Topic()).
				Msg("The subscriber failed to close.")
		} else {
			log.Info().
				Str("topic", subscriber.Topic()).
				Msg("The subscriber has been closed successfully.")
		}
	}

	// Wait for all subscribers to stop
	m.waitGroup.Wait()

	if len(errorMessages) > 0 {
		return fmt.Errorf("%w: %s", ErrClosingSubcribers, strings.Join(errorMessages, ", "))
	}

	log.Info().
		Msg("All subscribers have been shut down successfully.")

	return nil
}
