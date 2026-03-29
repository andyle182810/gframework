package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/andyle182810/gframework/taskqueue"
	"github.com/rs/zerolog/log"
)

type TaskRecovery struct {
	taskQueue *taskqueue.Queue
}

func NewTaskRecovery(taskQueue *taskqueue.Queue) *TaskRecovery {
	return &TaskRecovery{taskQueue: taskQueue}
}

// Execute recovers tasks that have been in the processing set for more than 2 minutes.
// This catches tasks from crashed workers or ungraceful shutdowns.
func (r *TaskRecovery) Execute(ctx context.Context) error {
	recovered, err := r.taskQueue.RecoverStale(ctx, 2*time.Minute) //nolint:mnd
	if err != nil {
		log.Error().Err(err).Msg("Failed to recover stale tasks")

		return fmt.Errorf("failed to recover stale tasks: %w", err)
	}

	if recovered > 0 {
		log.Warn().
			Int("recovered_count", recovered).
			Msg("Recovered stale tasks from processing set")
	} else {
		log.Debug().Msg("No stale tasks to recover")
	}

	return nil
}
