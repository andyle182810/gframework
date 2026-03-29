package publisher

import (
	"context"

	"github.com/andyle182810/gframework/redispub"
)

type OrderPublisher interface {
	PublishOrder(ctx context.Context) error
}

func NewOrderPublisher(publisher redispub.Publisher, topic string) *EventPublisher {
	return &EventPublisher{publisher: publisher, topic: topic, eventType: "order_created"}
}

func (p *EventPublisher) PublishOrder(ctx context.Context) error {
	return p.publish(ctx)
}
