# Chaos & Resilience Test Checklist

This document outlines a series of chaos engineering experiments to validate the resilience and durability of the log ingestion service. These tests should be performed in a staging environment that closely mirrors production.

## 1. Redis Outage (Buffer Unavailability)

**Goal:** Verify that the Write-Ahead Log (WAL) fallback mechanism works correctly and no data is lost during a transient Redis failure.

**Procedure:**
1.  Start a sustained load test against the `/ingest` endpoint.
2.  While the load is active, stop the Redis container: `docker-compose stop redis`.
3.  **Observe:**
    - `[ ]` The ingestor service logs should indicate Redis is unavailable and it's falling back to the WAL.
    - `[ ]` The `/ingest` endpoint should remain responsive, returning `202 Accepted`.
    - `[ ]` The `wal_active` Prometheus metric should switch to `1`.
    - `[ ]` WAL segment files should be created and grow in the configured WAL directory.
4.  After a few minutes, restart the Redis container: `docker-compose start redis`.
5.  **Observe:**
    - `[ ]` The ingestor service logs should indicate Redis is available again and that it's starting to replay events from the WAL.
    - `[ ]` The `wal_active` Prometheus metric should return to `0`.
    - `[ ]` After replay is complete, the WAL directory should be truncated (emptied).
    - `[ ]` Query PostgreSQL to confirm that events ingested during the outage have been successfully processed and stored. There should be no data loss.

## 2. PostgreSQL Outage (Sink Unavailability)

**Goal:** Verify that the consumer workers handle sink failures gracefully, retry, and eventually move messages to the Dead-Letter Queue (DLQ) without losing data from the buffer.

**Procedure:**
1.  Start a sustained load test.
2.  While the load is active, stop the PostgreSQL container: `docker-compose stop postgres`.
3.  **Observe:**
    - `[ ]` The consumer worker logs should show errors when trying to write to PostgreSQL, followed by retry attempts with backoff.
    - `[ ]` Use the admin API or `redis-cli` to check pending messages in the Redis Stream (`XPENDING log_events log-processors`). The number of pending messages should grow significantly.
    - `[ ]` After the configured number of retries, the consumer logs should indicate that failed batches are being moved to the DLQ stream (`log_events_dlq`).
    - `[ ]` The main `log_events` stream should continue to be acknowledged, preventing it from growing indefinitely with unprocessable messages.
4.  Restart the PostgreSQL container: `docker-compose start postgres`.
5.  **Observe:**
    - `[ ]` New messages should start processing and sinking to PostgreSQL successfully.
    - `[ ]` Messages in the DLQ remain there. An operator would need to manually inspect and replay them. This confirms the DLQ mechanism is working as designed.

## 3. Consumer Worker Crash

**Goal:** Verify that the Redis consumer group correctly reassigns pending messages from a crashed consumer to a healthy one, ensuring at-least-once processing.

**Procedure:**
1.  Ensure at least two consumer replicas are running (`docker-compose up --scale consumer=2`).
2.  Start a sustained load test.
3.  Identify one of the consumer containers and stop it: `docker kill <consumer_container_id>`.
4.  **Observe:**
    - `[ ]` Use the admin API or `redis-cli` to inspect pending messages. Messages that were being processed by the killed consumer will become idle.
    - `[ ]` After a short period (the idle timeout), the healthy consumer should claim the pending messages from the dead consumer (`XCLAIM`). This can be observed in the healthy consumer's logs or by monitoring `XPENDING`.
    - `[ ]` Query PostgreSQL to confirm that all data is eventually processed and stored. There should be no data loss.

## 4. Network Partition / Latency

**Goal:** Simulate network degradation to observe service behavior under non-ideal network conditions.

**Procedure:**
1.  Use a tool like `tc` (traffic control) on Linux or a Docker network with latency settings to introduce high latency between:
    - The ingestor and Redis.
    - The consumer and Redis.
    - The consumer and PostgreSQL.
2.  **Observe:**
    - `[ ]` **Ingestor <-> Redis Latency:** Ingestion latency (`p99`) should increase. If latency is high enough to cause timeouts, the WAL fallback should engage.
    - `[ ]` **Consumer <-> PostgreSQL Latency:** Consumer batch processing times should increase. This may lead to a backlog in the Redis Stream, causing `XPENDING` to grow. The system should remain stable but process logs more slowly.

