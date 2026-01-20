package msghandler

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/rs/zerolog/log"
)

func (h *Handler) HandleOrder(_ context.Context, payload message.Payload) error {
	log.Info().
		Str("topic", "orders").
		Str("payload", string(payload)).
		Msg("Processing order message")

	processingTime := time.Duration(rand.IntN(500)+100) * time.Millisecond //nolint:gosec,mnd
	time.Sleep(processingTime)

	log.Info().
		Str("topic", "orders").
		Dur("processing_time", processingTime).
		Msg("Order message processed successfully")

	return nil
}
