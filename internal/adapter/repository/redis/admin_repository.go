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

// AdminRepository implements the domain.StreamAdminRepository interface for Redis.
type AdminRepository struct {
	client *redis.Client
	logger *slog.Logger
}

// NewAdminRepository creates a new Redis admin repository.
func NewAdminRepository(client *redis.Client, logger *slog.Logger) *AdminRepository {
	return &AdminRepository{
		client: client,
		logger: logger,
	}
}

// GetGroupInfo retrieves information about all consumer groups for a given stream.
func (r *AdminRepository) GetGroupInfo(ctx context.Context, stream string) ([]domain.ConsumerGroupInfo, error) {
	groups, err := r.client.XInfoGroups(ctx, stream).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get group info for stream %s: %w", stream, err)
	}

	result := make([]domain.ConsumerGroupInfo, len(groups))
	for i, g := range groups {
		result[i] = domain.ConsumerGroupInfo{
			Name:            g.Name,
			Consumers:       g.Consumers,
			Pending:         g.Pending,
			LastDeliveredID: g.LastDeliveredID,
		}
	}
	return result, nil
}

// GetConsumerInfo retrieves information about consumers in a specific group.
func (r *AdminRepository) GetConsumerInfo(ctx context.Context, stream, group string) ([]domain.ConsumerInfo, error) {
	consumers, err := r.client.XInfoConsumers(ctx, stream, group).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer info for stream %s, group %s: %w", stream, group, err)
	}

	result := make([]domain.ConsumerInfo, len(consumers))
	for i, c := range consumers {
		result[i] = domain.ConsumerInfo{
			Name:    c.Name,
			Pending: c.Pending,
			Idle:    time.Duration(c.Idle) * time.Millisecond,
		}
	}
	return result, nil
}

// GetPendingSummary retrieves a summary of pending messages for a group.
func (r *AdminRepository) GetPendingSummary(ctx context.Context, stream, group string) (*domain.PendingMessageSummary, error) {
	pending, err := r.client.XPending(ctx, stream, group).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending summary for stream %s, group %s: %w", stream, group, err)
	}

	summary := &domain.PendingMessageSummary{
		Total:          pending.Count,
		FirstMessageID: pending.Lower,
		LastMessageID:  pending.Upper,
		ConsumerTotals: pending.Consumers,
	}
	return summary, nil
}

// GetPendingMessages retrieves detailed information about pending messages.
func (r *AdminRepository) GetPendingMessages(ctx context.Context, stream, group, consumer string, startID string, count int64) ([]domain.PendingMessageDetail, error) {
	args := &redis.XPendingExtArgs{
		Stream:   stream,
		Group:    group,
		Start:    startID,
		End:      "+",
		Count:    count,
		Consumer: consumer,
	}

	messages, err := r.client.XPendingExt(ctx, args).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	result := make([]domain.PendingMessageDetail, len(messages))
	for i, m := range messages {
		result[i] = domain.PendingMessageDetail{
			ID:         m.ID,
			Consumer:   m.Consumer,
			IdleTime:   m.Idle,
			RetryCount: m.RetryCount,
		}
	}
	return result, nil
}

// ClaimMessages claims pending messages for a new consumer.
func (r *AdminRepository) ClaimMessages(ctx context.Context, stream, group, consumer string, minIdleTime time.Duration, messageIDs []string) ([]domain.LogEvent, error) {
	args := &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdleTime,
		Messages: messageIDs,
	}

	claimedMessages, err := r.client.XClaim(ctx, args).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to claim messages: %w", err)
	}

	events := make([]domain.LogEvent, 0, len(claimedMessages))
	for _, msg := range claimedMessages {
		var event domain.LogEvent
		// Assuming the message value is a JSON-encoded LogEvent
		if data, ok := msg.Values["data"].(string); ok {
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				event.StreamMessageID = msg.ID
				events = append(events, event)
			} else {
				r.logger.Warn("failed to unmarshal claimed message into LogEvent", "messageID", msg.ID, "error", err)
			}
		}
	}
	return events, nil
}

// AcknowledgeMessages acknowledges messages in a stream.
func (r *AdminRepository) AcknowledgeMessages(ctx context.Context, stream, group string, messageIDs ...string) (int64, error) {
	if len(messageIDs) == 0 {
		return 0, errors.New("at least one message ID is required")
	}
	return r.client.XAck(ctx, stream, group, messageIDs...).Result()
}

// TrimStream trims a stream to a maximum length.
func (r *AdminRepository) TrimStream(ctx context.Context, stream string, maxLen int64) (int64, error) {
	return r.client.XTrimMaxLen(ctx, stream, maxLen).Result()
}

