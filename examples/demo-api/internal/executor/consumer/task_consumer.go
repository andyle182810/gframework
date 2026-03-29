package consumer

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/andyle182810/gframework/taskqueue"
	"github.com/rs/zerolog/log"
)

type TaskConsumer struct{}

func New() *TaskConsumer {
	return &TaskConsumer{}
}

func (c *TaskConsumer) Execute(ctx context.Context, taskID string, payload taskqueue.Payload) error {
	sleepDuration := time.Duration(rand.IntN(3)+1) * time.Second //nolint:gosec,mnd

	log.Info().
		Str("task_id", taskID).
		Str("payload", string(payload)).
		Dur("sleep_duration", sleepDuration).
		Msg("Processing task...")

	select {
	case <-time.After(sleepDuration):
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Info().Str("task_id", taskID).Msg("Task completed")

	return nil
}
