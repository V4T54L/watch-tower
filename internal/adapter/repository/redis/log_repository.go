package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/V4T54L/watch-tower/internal/adapter/metrics"
	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/redis/go-redis/v9"
)

const logStreamKey = "log_events"

var errNotImplemented = errors.New("method not implemented for this repository type")
var ErrRedisNotAvailable = errors.New("redis not available")

// LogRepository implements domain.LogRepository for Redis Streams with WAL failover.
type LogRepository struct {
	client       *redis.Client
	logger       *slog.Logger
	wal          domain.WALRepository
	dlqStreamKey string
	isAvailable  atomic.Bool
	metrics      *metrics.IngestMetrics
}

// NewLogRepository creates a new Redis LogRepository.
// The WAL is optional; pass nil if not needed (e.g., for consumers).
func NewLogRepository(client *redis.Client, logger *slog.Logger, group, consumer, dlqStreamKey string, wal domain.WALRepository, m *metrics.IngestMetrics) (*LogRepository, error) {
	repo := &LogRepository{
		client:       client,
		logger:       logger.With("component", "redis_repository"),
		wal:          wal,
		dlqStreamKey: dlqStreamKey,
		metrics:      m,
	}
	repo.isAvailable.Store(true) // Assume available initially

	if err := repo.setupConsumerGroup(context.Background(), group); err != nil && !isRedisBusyGroupError(err) {
		repo.isAvailable.Store(false)
		repo.logger.Warn("Could not setup Redis consumer group on init, Redis might be down", "error", err)
		// Don't return error, allow startup with WAL
	}

	return repo, nil
}

// StartHealthCheck starts a background goroutine to monitor Redis connectivity and trigger WAL replay.
func (r *LogRepository) StartHealthCheck(ctx context.Context, interval time.Duration) {
	if r.wal == nil {
		r.logger.Info("WAL is not configured, skipping health check routine.")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.logger.Info("Starting Redis health check and WAL replayer")

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping Redis health check routine")
			return
		case <-ticker.C:
			wasAvailable := r.isAvailable.Load()
			err := r.client.Ping(ctx).Err()
			isCurrentlyAvailable := err == nil

			if isCurrentlyAvailable && !wasAvailable {
				r.logger.Info("Redis connection recovered. Starting WAL replay.")
				if r.metrics != nil {
					r.metrics.WALActive.Set(0)
				}
				if err := r.ReplayWAL(ctx); err != nil {
					r.logger.Error("Failed to replay WAL after Redis recovery", "error", err)
				} else {
					r.isAvailable.Store(true)
				}
			} else if !isCurrentlyAvailable && wasAvailable {
				r.logger.Warn("Redis connection lost. Activating WAL.", "error", err)
				r.isAvailable.Store(false)
				if r.metrics != nil {
					r.metrics.WALActive.Set(1)
				}
			}
		}
	}
}

// ReplayWAL replays events from the WAL to Redis and truncates the WAL on success.
func (r *LogRepository) ReplayWAL(ctx context.Context) error {
	r.logger.Info("Starting WAL replay to Redis")
	var replayedCount int
	replayHandler := func(event domain.LogEvent) error {
		if err := r.bufferLogToRedis(ctx, event); err != nil {
			// If we can't write to Redis during replay, something is very wrong.
			r.logger.Error("Failed to buffer event from WAL to Redis", "event_id", event.ID, "error", err)
			return err
		}
		replayedCount++
		return nil
	}

	if err := r.wal.Replay(ctx, replayHandler); err != nil {
		return fmt.Errorf("WAL replay failed: %w", err)
	}

	r.logger.Info("WAL replay finished", "replayed_count", replayedCount)
	if err := r.wal.Truncate(ctx); err != nil {
		r.logger.Error("Failed to truncate WAL after successful replay", "error", err)
		return fmt.Errorf("failed to truncate WAL after successful replay: %w", err)
	}

	r.logger.Info("WAL truncated successfully")
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
		if r.metrics != nil {
			r.metrics.WALActive.Set(1)
		}
		return r.wal.Write(ctx, event)
	}

	err := r.bufferLogToRedis(ctx, event)
	if err != nil {
		if isNetworkError(err) {
			if r.isAvailable.CompareAndSwap(true, false) {
				r.logger.Error("Redis connection lost during write", "error", err)
				if r.metrics != nil {
					r.metrics.WALActive.Set(1)
				}
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
				"payload":           payload,
				"original_event_id": event.ID,
				"original_stream":   logStreamKey,
				// "failed_at":       time.Now().UTC().Format(time.RFC3339), // Removed as per attempted content
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
	return err != nil && strings.HasPrefix(err.Error(), "BUSYGROUP")
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "i/o timeout") ||
		strings.Contains(err.Error(), "connection refused") ||
		errors.Is(err, redis.ErrClosed) // Keep original redis.ErrClosed
}
