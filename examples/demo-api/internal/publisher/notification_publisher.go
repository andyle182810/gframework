package publisher

import (
	"context"

	"github.com/andyle182810/gframework/redispub"
)

type NotificationPublisher interface {
	PublishNotification(ctx context.Context) error
}

func NewNotificationPublisher(publisher redispub.Publisher, topic string) *EventPublisher {
	return &EventPublisher{publisher: publisher, topic: topic, eventType: "notification_sent"}
}

func (p *EventPublisher) PublishNotification(ctx context.Context) error {
	return p.publish(ctx)
}
