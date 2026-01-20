package msghandler

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/rs/zerolog/log"
)

func (h *Handler) HandleAnalytics(_ context.Context, payload message.Payload) error {
	log.Info().
		Str("topic", "analytics").
		Str("payload", string(payload)).
		Msg("Processing analytics message")

	processingTime := time.Duration(rand.IntN(200)+50) * time.Millisecond //nolint:gosec,mnd
	time.Sleep(processingTime)

	log.Info().
		Str("topic", "analytics").
		Dur("processing_time", processingTime).
		Msg("Analytics message processed successfully")

	return nil
}
