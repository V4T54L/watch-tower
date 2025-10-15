package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/domain"
)

const (
	logStreamKey = "log_events"
)

var errNotImplemented = errors.New("method not implemented for this repository type")

// LogRepository implements the domain.LogRepository interface using Redis Streams.
// It also includes a Write-Ahead Log (WAL) for failover.
type LogRepository struct {
	client       *redis.Client
	logger       *slog.Logger
	wal          domain.WALRepository
	dlqStreamKey string
	isAvailable  atomic.Bool
}

// NewLogRepository creates a new Redis-backed LogRepository.
// The WAL is optional; pass nil if not needed (e.g., for consumers).
func NewLogRepository(client *redis.Client, logger *slog.Logger, group, consumer, dlqStreamKey string, wal domain.WALRepository) (*LogRepository, error) {
	repo := &LogRepository{
		client:       client,
		logger:       logger.With("component", "redis_repository"),
		wal:          wal,
		dlqStreamKey: dlqStreamKey,
	}
	repo.isAvailable.Store(true) // Assume available initially

	if err := repo.setupConsumerGroup(context.Background(), group); err != nil {
		repo.isAvailable.Store(false)
		repo.logger.Error("Failed to setup consumer group, Redis may be unavailable on startup", "error", err)
	}

	return repo, nil
}

// StartHealthCheck starts a background goroutine to monitor Redis connectivity and trigger WAL replay.
func (r *LogRepository) StartHealthCheck(ctx context.Context, interval time.Duration) {
	if r.wal == nil {
		r.logger.Info("WAL is not configured, skipping health check/replayer")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.logger.Info("Starting Redis health check and WAL replayer")

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping Redis health check")
			return
		case <-ticker.C:
			err := r.client.Ping(ctx).Err()
			if err != nil {
				if r.isAvailable.CompareAndSwap(true, false) {
					r.logger.Error("Redis connection lost", "error", err)
				}
			} else {
				if r.isAvailable.CompareAndSwap(false, true) {
					r.logger.Info("Redis connection recovered")
					if err := r.ReplayWAL(ctx); err != nil {
						r.logger.Error("Failed to replay WAL after Redis recovery", "error", err)
						r.isAvailable.Store(false)
					}
				}
			}
		}
	}
}

// ReplayWAL replays events from the WAL to Redis and truncates the WAL on success.
func (r *LogRepository) ReplayWAL(ctx context.Context) error {
	r.logger.Info("Attempting to replay WAL to Redis")
	replayHandler := func(event domain.LogEvent) error {
		return r.bufferLogToRedis(ctx, event)
	}

	if err := r.wal.Replay(ctx, replayHandler); err != nil {
		return fmt.Errorf("WAL replay failed: %w", err)
	}

	if err := r.wal.Truncate(ctx); err != nil {
		return fmt.Errorf("failed to truncate WAL after successful replay: %w", err)
	}

	r.logger.Info("WAL replay to Redis completed successfully")
	return nil
}

func (r *LogRepository) setupConsumerGroup(ctx context.Context, group string) error {
	err := r.client.XGroupCreateMkStream(ctx, logStreamKey, group, "0").Err()
	if err != nil && !isRedisBusyGroupError(err) {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

// BufferLog adds a log event to the Redis Stream, falling back to the WAL if Redis is unavailable.
func (r *LogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	if !r.isAvailable.Load() {
		if r.wal == nil {
			return errors.New("redis is unavailable and WAL is not configured")
		}
		r.logger.Warn("Redis is unavailable, writing to WAL", "event_id", event.ID)
		return r.wal.Write(ctx, event)
	}

	err := r.bufferLogToRedis(ctx, event)
	if err != nil {
		if isNetworkError(err) {
			if r.isAvailable.CompareAndSwap(true, false) {
				r.logger.Error("Redis connection lost during write", "error", err)
			}
			if r.wal == nil {
				return fmt.Errorf("redis became unavailable and WAL is not configured: %w", err)
			}
			r.logger.Warn("Redis became unavailable, writing to WAL", "event_id", event.ID)
			return r.wal.Write(ctx, event)
		}
		return err
	}
	return nil
}

func (r *LogRepository) bufferLogToRedis(ctx context.Context, event domain.LogEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: logStreamKey,
		Values: map[string]interface{}{"payload": payload},
	}

	if err := r.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("failed to XADD to redis stream: %w", err)
	}
	return nil
}

// ReadLogBatch reads a batch of log events from the Redis Stream for a consumer group.
func (r *LogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	args := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{logStreamKey, ">"},
		Count:    int64(count),
		Block:    2 * time.Second,
	}

	streams, err := r.client.XReadGroup(ctx, args).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to XREADGROUP from redis: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	messages := streams[0].Messages
	events := make([]domain.LogEvent, 0, len(messages))
	for _, msg := range messages {
		payload, ok := msg.Values["payload"].(string)
		if !ok {
			r.logger.Warn("Invalid message format in stream, skipping", "message_id", msg.ID)
			continue
		}

		var event domain.LogEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			r.logger.Warn("Failed to unmarshal log event from stream, skipping", "message_id", msg.ID, "error", err)
			continue
		}
		event.StreamMessageID = msg.ID
		events = append(events, event)
	}

	return events, nil
}

// AcknowledgeLogs acknowledges processed messages in the Redis Stream.
func (r *LogRepository) AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if err := r.client.XAck(ctx, logStreamKey, group, messageIDs...).Err(); err != nil {
		return fmt.Errorf("failed to XACK messages in redis: %w", err)
	}
	return nil
}

// MoveToDLQ moves a batch of events to the Dead-Letter Queue stream.
func (r *LogRepository) MoveToDLQ(ctx context.Context, events []domain.LogEvent) error {
	if len(events) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			r.logger.Error("Failed to marshal event for DLQ", "event_id", event.ID, "error", err)
			continue
		}
		args := &redis.XAddArgs{
			Stream: r.dlqStreamKey,
			Values: map[string]interface{}{
				"payload":         payload,
				"original_stream": logStreamKey,
				"original_msg_id": event.StreamMessageID,
				"failed_at":       time.Now().UTC().Format(time.RFC3339),
			},
		}
		pipe.XAdd(ctx, args)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute DLQ pipeline: %w", err)
	}
	r.logger.Warn("Moved events to DLQ", "count", len(events))
	return nil
}

// WriteLogBatch is not implemented for this repository.
func (r *LogRepository) WriteLogBatch(ctx context.Context, events []domain.LogEvent) error {
	return errNotImplemented
}

func isRedisBusyGroupError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

func isNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, redis.ErrClosed) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

