// Background processing with gframework: Redis Streams pub/sub with retries
// and a dead-letter queue, a periodic worker pool, and a distributed lock so
// only one instance runs the periodic job at a time.
//
// Requires Redis/Valkey (see docker-compose.yml):
//
//	docker compose up -d
//	go run .
//
// The publisher emits an event every 2 seconds; the subscriber processes it
// (and demonstrates retry + DLQ on failures); the worker pool runs a cleanup
// job every 10 seconds guarded by a distributed lock.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/andyle182810/gframework/distlock"
	"github.com/andyle182810/gframework/redispub"
	"github.com/andyle182810/gframework/redissub"
	"github.com/andyle182810/gframework/runner"
	"github.com/andyle182810/gframework/valkey"
	"github.com/andyle182810/gframework/workerpool"
	"github.com/rs/zerolog/log"
)

const (
	topic    = "orders"
	dlqTopic = "orders-dlq"
	group    = "order-processors"
)

type OrderEvent struct {
	OrderID string    `json:"order_id"`
	Amount  int       `json:"amount"`
	At      time.Time `json:"at"`
}

func main() {
	// Infrastructure: one Valkey/Redis client shared by every component.
	client, err := valkey.New(&valkey.Config{ //nolint:exhaustruct
		Host: "localhost",
		Port: 6379,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create valkey client")
		os.Exit(1)
	}

	publisher, err := redispub.New(client.Client, redispub.Options{}) //nolint:exhaustruct
	if err != nil {
		log.Error().Err(err).Msg("failed to create publisher")
		os.Exit(1)
	}

	// Subscriber with retries and a DLQ. After 3 failed attempts the message is
	// written to the DLQ stream and acknowledged; if the DLQ write itself fails,
	// the message stays pending and is redelivered — it is never silently lost.
	subscriber, err := redissub.NewSubscriber(
		client.Client,
		group,
		topic,
		handleOrder,
		redissub.WithRetry(3, time.Second, dlqTopic),
		redissub.WithExecTimeout(30*time.Second),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create subscriber")
		os.Exit(1)
	}

	// Periodic publisher: emits a demo event every 2 seconds.
	emitter := workerpool.New(
		&orderEmitter{publisher: publisher},
		workerpool.WithName("order-emitter"),
		workerpool.WithTickInterval(2*time.Second),
		workerpool.WithExecutionTimeout(5*time.Second),
	)

	// Periodic cleanup guarded by a distributed lock: when several instances of
	// this process run, only one executes the job per tick; the others skip.
	locker := distlock.New(client.Client)
	cleaner := workerpool.New(
		&cleanupJob{locker: locker},
		workerpool.WithName("cleanup"),
		workerpool.WithTickInterval(10*time.Second),
		workerpool.WithExecutionTimeout(time.Minute),
	)

	runner.New(
		runner.WithInfrastructureService(client),
		runner.WithCoreService(subscriber),
		runner.WithCoreService(emitter),
		runner.WithCoreService(cleaner),
	).Run()
}

func handleOrder(_ context.Context, payload message.Payload) error {
	var event OrderEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode order event: %w", err)
	}

	log.Info().
		Str("order_id", event.OrderID).
		Int("amount", event.Amount).
		Msg("order processed")

	return nil
}

type orderEmitter struct {
	publisher *redispub.RedisPublisher
	counter   int
}

func (e *orderEmitter) Execute(ctx context.Context) error {
	e.counter++

	event := OrderEvent{
		OrderID: fmt.Sprintf("order-%d", e.counter),
		Amount:  e.counter * 10,
		At:      time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode order event: %w", err)
	}

	return e.publisher.PublishToTopic(ctx, topic, string(payload))
}

type cleanupJob struct {
	locker *distlock.Locker
}

func (j *cleanupJob) Execute(ctx context.Context) error {
	// TryWithLock silently skips when another instance holds the lock.
	return j.locker.TryWithLock(ctx, "jobs:cleanup", time.Minute, func() error {
		log.Info().Msg("running cleanup (this instance holds the lock)")

		return nil
	})
}
