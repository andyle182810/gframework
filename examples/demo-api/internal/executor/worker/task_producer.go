package worker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/andyle182810/gframework/taskqueue"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type TaskProducer struct {
	taskQueue *taskqueue.Queue
}

func NewTaskProducer(taskQueue *taskqueue.Queue) *TaskProducer {
	return &TaskProducer{taskQueue: taskQueue}
}

func (p *TaskProducer) Execute(ctx context.Context) error {
	taskCount := rand.IntN(5) + 1 //nolint:gosec,mnd
	tasks := make([]taskqueue.Task, taskCount)

	for i := range taskCount {
		tasks[i] = taskqueue.Task{
			ID:      uuid.New().String(),
			Payload: fmt.Appendf(nil, `{"index":%d,"timestamp":"%s"}`, i, time.Now().Format(time.RFC3339)),
		}
	}

	taskIDs := make([]string, taskCount)
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	log.Info().
		Int("task_count", taskCount).
		Strs("task_ids", taskIDs).
		Msg("Task producer pushing tasks to queue")

	if err := p.taskQueue.Push(ctx, tasks...); err != nil {
		return fmt.Errorf("failed to push tasks: %w", err)
	}

	log.Info().Msg("Task producer done")

	return nil
}
