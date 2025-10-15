package redis

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/domain"
)

const (
	logStreamKey = "log_events"
)

// LogRepository implements the buffering parts of the domain.LogRepository interface using Redis Streams.
type LogRepository struct {
	client *redis.Client
	logger *slog.Logger
}

// NewLogRepository creates a new Redis-backed LogRepository.
func NewLogRepository(client *redis.Client, logger *slog.Logger) *LogRepository {
	return &LogRepository{
		client: client,
		logger: logger,
	}
}

// BufferLog adds a log event to the Redis Stream using XADD.
func (r *LogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		r.logger.Error("failed to marshal log event for redis", "error", err, "event_id", event.ID)
		return err
	}

	args := &redis.XAddArgs{
		Stream: logStreamKey,
		Values: map[string]interface{}{
			"event_id": event.ID,
			"payload":  string(eventBytes),
		},
	}

	_, err = r.client.XAdd(ctx, args).Result()
	if err != nil {
		r.logger.Error("failed to add event to redis stream", "error", err, "event_id", event.ID)
		return err
	}

	return nil
}

// ReadLogBatch is a placeholder for the consumer implementation.
func (r *LogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	return nil, errors.New("not implemented")
}

// WriteLogBatch is not implemented by the Redis repository; it's a sink responsibility.
func (r *LogRepository) WriteLogBatch(ctx context.Context, events []domain.LogEvent) error {
	return errors.New("not implemented: WriteLogBatch is a sink-specific method")
}

// AcknowledgeLogs is a placeholder for the consumer implementation.
func (r *LogRepository) AcknowledgeLogs(ctx context.Context, group string, eventIDs ...string) error {
	return errors.New("not implemented")
}

