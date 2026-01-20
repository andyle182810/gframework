package redissub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	ErrEmptyTopic                    = errors.New("multi_subscriber: topic cannot be empty")
	ErrSubscriberCreation            = errors.New("multi_subscriber: failed to create subscriber")
	ErrClosingSubscribers            = errors.New("multi_subscriber: errors while closing subscribers")
	ErrNoSubscribers                 = errors.New("multi_subscriber: no subscribers registered")
	ErrMultiSubscriberAlreadyRunning = errors.New("multi_subscriber: already running")
)

type MultiSubscriber struct {
	name           string
	redisClient    goredis.UniversalClient
	consumerGroup  string
	subscribers    []*Subscriber
	waitGroup      sync.WaitGroup
	subscribersMux sync.Mutex
	healthy        atomic.Bool
	running        atomic.Bool
	opts           []SubscriberOption
	shutdownSignal chan struct{}
	stoppedSignal  chan struct{}
}

func NewMultiSubscriber(
	name string,
	redisClient goredis.UniversalClient,
	consumerGroup string,
	opts ...SubscriberOption,
) *MultiSubscriber {
	//nolint:exhaustruct
	multiSub := &MultiSubscriber{
		name:           name,
		redisClient:    redisClient,
		consumerGroup:  consumerGroup,
		subscribers:    make([]*Subscriber, 0),
		opts:           opts,
		shutdownSignal: make(chan struct{}),
		stoppedSignal:  make(chan struct{}),
	}
	multiSub.healthy.Store(false)

	return multiSub
}

func (m *MultiSubscriber) Subscribe(topic string, messageHandler MessageHandler) error {
	if topic == "" {
		return ErrEmptyTopic
	}

	if messageHandler == nil {
		return ErrNilMessageHandler
	}

	subscriber, err := NewSubscriber(m.redisClient, m.consumerGroup, topic, messageHandler, m.opts...)
	if err != nil {
		return fmt.Errorf("%w for topic %s: %w", ErrSubscriberCreation, topic, err)
	}

	m.subscribersMux.Lock()
	m.subscribers = append(m.subscribers, subscriber)
	m.subscribersMux.Unlock()

	log.Info().
		Str("topic", topic).
		Msg("The subscription to the topic has been registered")

	return nil
}

func (m *MultiSubscriber) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return ErrMultiSubscriberAlreadyRunning
	}

	defer close(m.stoppedSignal)

	m.subscribersMux.Lock()
	subscribers := make([]*Subscriber, len(m.subscribers))
	copy(subscribers, m.subscribers)
	m.subscribersMux.Unlock()

	if len(subscribers) == 0 {
		log.Warn().Msg("No subscribers registered, waiting for stop signal")

		return nil
	}

	log.Info().
		Int("count", len(subscribers)).
		Msg("Starting all subscribers")

	m.healthy.Store(true)

	for _, subscriber := range subscribers {
		m.waitGroup.Add(1)

		go func(sub *Subscriber) {
			defer m.waitGroup.Done()

			if err := sub.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error().
					Err(err).
					Str("topic", sub.Topic()).
					Msg("Subscriber failed")
			}
		}(subscriber)
	}

	for {
		select {
		case <-ctx.Done():
			m.healthy.Store(false)
			log.Info().
				Str("service_name", m.Name()).
				Int("subscriber_count", len(subscribers)).
				Msg("Multi-subscriber stopped: context cancelled")

			return ctx.Err()
		case <-m.shutdownSignal:
			m.healthy.Store(false)
			log.Info().
				Str("service_name", m.Name()).
				Int("subscriber_count", len(subscribers)).
				Msg("Multi-subscriber stopped: graceful shutdown initiated")

			return nil
		}
	}
}

func (m *MultiSubscriber) Stop() error {
	if !m.running.CompareAndSwap(true, false) {
		return nil
	}

	log.Info().
		Msg("The shutdown of all subscribers is being initiated")

	m.healthy.Store(false)

	if m.shutdownSignal != nil {
		close(m.shutdownSignal)
	}

	var errorMessages []string

	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	for _, subscriber := range m.subscribers {
		if err := subscriber.Stop(); err != nil {
			errorMessages = append(
				errorMessages,
				fmt.Sprintf("[%s/%s: %v]", subscriber.ConsumerGroup(), subscriber.Topic(), err),
			)

			log.Error().
				Err(err).
				Str("topic", subscriber.Topic()).
				Str("name", subscriber.Name()).
				Msg("The subscriber failed to stop")
		} else {
			log.Info().
				Str("topic", subscriber.Topic()).
				Str("name", subscriber.Name()).
				Msg("The subscriber has been stopped successfully")
		}
	}

	m.waitGroup.Wait() // Wait for all goroutines to finish

	if len(errorMessages) > 0 {
		return fmt.Errorf("%w: %s", ErrClosingSubscribers, strings.Join(errorMessages, ", "))
	}

	shutdownStart := time.Now()
	select {
	case <-m.stoppedSignal:
		log.Info().
			Str("service_name", m.Name()).
			Dur("shutdown_duration", time.Since(shutdownStart)).
			Msg("Multi-subscriber stopped cleanly")
	case <-time.After(defaultShutdownTimeout):
		log.Error().
			Str("service_name", m.Name()).
			Dur("timeout", defaultShutdownTimeout).
			Msg("Timeout waiting for multi-subscriber to stop gracefully")
	}

	log.Info().
		Str("service_name", m.Name()).
		Msg("All subscribers have been shut down successfully")

	return nil
}

func (m *MultiSubscriber) Name() string {
	return m.name
}

func (m *MultiSubscriber) IsHealthy() bool {
	if !m.healthy.Load() {
		return false
	}

	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	for _, sub := range m.subscribers {
		if !sub.IsHealthy() {
			return false
		}
	}

	return true
}

func (m *MultiSubscriber) SubscriberCount() int {
	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	return len(m.subscribers)
}
