package publisher

import (
	"context"

	"github.com/andyle182810/gframework/redispub"
)

type AnalyticsPublisher interface {
	PublishAnalytics(ctx context.Context) error
}

func NewAnalyticsPublisher(publisher redispub.Publisher, topic string) *EventPublisher {
	return &EventPublisher{publisher: publisher, topic: topic, eventType: "analytics_event"}
}

func (p *EventPublisher) PublishAnalytics(ctx context.Context) error {
	return p.publish(ctx)
}
