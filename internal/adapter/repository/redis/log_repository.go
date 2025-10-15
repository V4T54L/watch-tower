package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/domain"
)

const (
	logStreamKey = "log_events"
)

// LogRepository implements the buffering part of the domain.LogRepository interface using Redis Streams.
type LogRepository struct {
	client *redis.Client
	logger *slog.Logger
}

// NewLogRepository creates a new Redis log repository.
// It also ensures the consumer group for the log stream exists.
func NewLogRepository(client *redis.Client, logger *slog.Logger, group, consumer string) (*LogRepository, error) {
	repo := &LogRepository{
		client: client,
		logger: logger,
	}
	if err := repo.setupConsumerGroup(context.Background(), group); err != nil {
		return nil, fmt.Errorf("failed to setup consumer group: %w", err)
	}
	return repo, nil
}

// setupConsumerGroup creates the Redis stream and consumer group if they don't already exist.
func (r *LogRepository) setupConsumerGroup(ctx context.Context, group string) error {
	// XGROUP CREATE log_events mygroup 0 MKSTREAM
	// The MKSTREAM option creates the stream if it doesn't exist.
	// We ignore the "BUSYGROUP Consumer Group name already exists" error.
	err := r.client.XGroupCreateMkStream(ctx, logStreamKey, group, "0").Err()
	if err != nil && !errors.Is(err, redis.Nil) && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	r.logger.Info("ensured redis consumer group exists", "stream", logStreamKey, "group", group)
	return nil
}

// BufferLog adds a log event to the Redis stream.
func (r *LogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: logStreamKey,
		Values: map[string]interface{}{
			"event_id": event.ID,
			"payload":  payload,
		},
	}

	_, err = r.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to add event to redis stream: %w", err)
	}
	return nil
}

// ReadLogBatch reads a batch of log events from the stream for a specific consumer group.
// It blocks for a short duration if no new messages are available.
func (r *LogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	// XREADGROUP GROUP mygroup myconsumer COUNT 10 BLOCK 2000 STREAMS log_events >
	// ">" means read new messages that have not been delivered to any other consumer.
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{logStreamKey, ">"},
		Count:    int64(count),
		Block:    2 * time.Second,
	}).Result()

	if err != nil {
		// redis.Nil indicates a timeout, which is normal. Return an empty slice.
		if errors.Is(err, redis.Nil) {
			return []domain.LogEvent{}, nil
		}
		return nil, fmt.Errorf("failed to read from redis stream group: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return []domain.LogEvent{}, nil
	}

	messages := streams[0].Messages
	events := make([]domain.LogEvent, 0, len(messages))
	for _, msg := range messages {
		payload, ok := msg.Values["payload"].(string)
		if !ok {
			r.logger.Warn("invalid message format in stream, payload not a string", "message_id", msg.ID)
			continue
		}

		var event domain.LogEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			r.logger.Warn("failed to unmarshal log event from stream", "message_id", msg.ID, "error", err)
			continue
		}
		event.StreamMessageID = msg.ID
		events = append(events, event)
	}

	return events, nil
}

// AcknowledgeLogs marks a set of log events as successfully processed in the stream.
func (r *LogRepository) AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	// XACK log_events mygroup <id1> <id2> ...
	_, err := r.client.XAck(ctx, logStreamKey, group, messageIDs...).Result()
	if err != nil {
		return fmt.Errorf("failed to acknowledge messages in redis stream: %w", err)
	}
	return nil
}

var errNotImplemented = errors.New("method not implemented for this repository type")

// WriteLogBatch is not implemented for the Redis buffer repository.
func (r *LogRepository) WriteLogBatch(ctx context.Context, events []domain.LogEvent) error {
	return errNotImplemented
}

