package domain

import (
	"context"
	"time"
)

// LogRepository defines the interface for log event persistence and buffering.
type LogRepository interface {
	BufferLog(ctx context.Context, event LogEvent) error
	ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]LogEvent, error)
	WriteLogBatch(ctx context.Context, events []LogEvent) error
	AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error
	MoveToDLQ(ctx context.Context, events []LogEvent) error
}

// APIKeyRepository defines the interface for validating API keys.
type APIKeyRepository interface {
	IsValid(ctx context.Context, key string) (bool, error)
}

// WALRepository defines the interface for a Write-Ahead Log.
type WALRepository interface {
	Write(ctx context.Context, event LogEvent) error
	Replay(ctx context.Context, handler func(event LogEvent) error) error
	Truncate(ctx context.Context) error
	Close() error
}

// StreamAdminRepository defines the interface for administrative operations on a stream.
type StreamAdminRepository interface {
	GetGroupInfo(ctx context.Context, stream string) ([]ConsumerGroupInfo, error)
	GetConsumerInfo(ctx context.Context, stream, group string) ([]ConsumerInfo, error)
	GetPendingSummary(ctx context.Context, stream, group string) (*PendingMessageSummary, error)
	GetPendingMessages(ctx context.Context, stream, group, consumer string, startID string, count int64) ([]PendingMessageDetail, error)
	ClaimMessages(ctx context.Context, stream, group, consumer string, minIdleTime time.Duration, messageIDs []string) ([]LogEvent, error)
	AcknowledgeMessages(ctx context.Context, stream, group string, messageIDs ...string) (int64, error)
	TrimStream(ctx context.Context, stream string, maxLen int64) (int64, error)
}
