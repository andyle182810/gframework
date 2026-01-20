package msghandler

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/rs/zerolog/log"
)

func (h *Handler) HandleNotification(_ context.Context, payload message.Payload) error {
	log.Info().
		Str("topic", "notifications").
		Str("payload", string(payload)).
		Msg("Processing notification message")

	processingTime := time.Duration(rand.IntN(300)+50) * time.Millisecond //nolint:gosec,mnd
	time.Sleep(processingTime)

	log.Info().
		Str("topic", "notifications").
		Dur("processing_time", processingTime).
		Msg("Notification message processed successfully")

	return nil
}
