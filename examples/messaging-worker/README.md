# messaging-worker

Background processing with gframework:

- **`redispub` / `redissub`** — Redis Streams pub/sub with consumer groups,
  per-message retries, execution timeouts, and a dead-letter queue
- **`workerpool`** — tick-driven periodic jobs (an event emitter and a cleanup task)
- **`distlock`** — a distributed lock so the cleanup job runs on only one
  instance at a time (`TryWithLock` silently skips when another instance holds it)
- **`runner`** — infrastructure (Valkey) starts before, and stops after, the
  core services

## Run

```bash
docker compose up -d
go run .
```

You'll see an order event published every 2 seconds and processed by the
subscriber, plus a cleanup job every 10 seconds. Start a second instance to see
the distributed lock in action: only one process logs "running cleanup".

## DLQ semantics

After `MaxRetries` failed attempts a message is written to the `orders-dlq`
stream and acknowledged. If the DLQ write fails, the message remains pending in
the consumer group and is redelivered later — messages are never silently dropped
when a DLQ is configured. Inspect the DLQ with:

```bash
redis-cli XRANGE orders-dlq - +
```
