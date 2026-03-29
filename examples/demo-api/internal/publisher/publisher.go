package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/andyle182810/gframework/redispub"
	"github.com/google/uuid"
)

type Event struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"eventType"`
}

type EventPublisher struct {
	publisher redispub.Publisher
	topic     string
	eventType string
}

func (p *EventPublisher) publish(ctx context.Context) error {
	event := &Event{
		ID:        uuid.New().String(),
		Topic:     p.topic,
		EventType: p.eventType,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal %s event: %w", p.eventType, err)
	}

	if err := p.publisher.PublishToTopic(ctx, p.topic, string(data)); err != nil {
		return fmt.Errorf("failed to publish %s event: %w", p.eventType, err)
	}

	return nil
}
