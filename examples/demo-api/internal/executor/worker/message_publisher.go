package worker

import (
	"context"
	"math/rand/v2"

	"github.com/andyle182810/gframework/examples/demo-api/internal/publisher"
	"github.com/rs/zerolog/log"
)

type MessagePublisher struct {
	orderPub        publisher.OrderPublisher
	notificationPub publisher.NotificationPublisher
	analyticsPub    publisher.AnalyticsPublisher
}

type MessagePublisherConfig struct {
	OrderPublisher        publisher.OrderPublisher
	NotificationPublisher publisher.NotificationPublisher
	AnalyticsPublisher    publisher.AnalyticsPublisher
}

func NewMessagePublisher(cfg *MessagePublisherConfig) *MessagePublisher {
	return &MessagePublisher{
		orderPub:        cfg.OrderPublisher,
		notificationPub: cfg.NotificationPublisher,
		analyticsPub:    cfg.AnalyticsPublisher,
	}
}

func (p *MessagePublisher) Execute(ctx context.Context) error {
	orderCount := rand.IntN(3) + 1 //nolint:gosec,mnd
	log.Info().
		Int("order_count", orderCount).
		Msg("Publishing order events")

	for range orderCount {
		if err := p.orderPub.PublishOrder(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to publish order event")

			return err
		}
	}

	notificationCount := rand.IntN(3) + 1 //nolint:gosec,mnd
	log.Info().
		Int("notification_count", notificationCount).
		Msg("Publishing notification events")

	for range notificationCount {
		if err := p.notificationPub.PublishNotification(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to publish notification event")

			return err
		}
	}

	analyticsCount := rand.IntN(3) + 1 //nolint:gosec,mnd
	log.Info().
		Int("analytics_count", analyticsCount).
		Msg("Publishing analytics events")

	for range analyticsCount {
		if err := p.analyticsPub.PublishAnalytics(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to publish analytics event")

			return err
		}
	}

	log.Info().Msg("Message publisher completed publishing events")

	return nil
}
