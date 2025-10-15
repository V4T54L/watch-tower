package domain

import (
	"context"
)

// LogRepository defines the contract for storing, retrieving, and managing log events.
type LogRepository interface {
	// BufferLog adds a single log event to a durable buffer (e.g., Redis Stream, WAL).
	BufferLog(ctx context.Context, event LogEvent) error

	// ReadLogBatch reads a batch of log events from the buffer for a specific consumer group.
	ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]LogEvent, error)

	// WriteLogBatch writes a batch of log events to the final persistent storage (e.g., PostgreSQL).
	WriteLogBatch(ctx context.Context, events []LogEvent) error

	// AcknowledgeLogs marks log events as successfully processed in the buffer.
	AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error

	// MoveToDLQ moves a batch of events to a Dead-Letter Queue.
	MoveToDLQ(ctx context.Context, events []LogEvent) error
}

// APIKeyRepository defines the contract for validating API keys.
type APIKeyRepository interface {
	IsValid(ctx context.Context, key string) (bool, error)
}

// WALRepository defines the contract for a Write-Ahead Log.
type WALRepository interface {
	Write(ctx context.Context, event LogEvent) error
	Replay(ctx context.Context, handler func(event LogEvent) error) error
	Truncate(ctx context.Context) error
	Close() error
}

