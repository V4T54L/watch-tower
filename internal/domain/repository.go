package domain

import (
	"context"
)

// LogRepository defines the interface for storing and retrieving log events.
// It acts as a port for both the durable buffer (e.g., Redis) and the final sink (e.g., PostgreSQL).
type LogRepository interface {
	BufferLog(ctx context.Context, event LogEvent) error
	ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]LogEvent, error)
	WriteLogBatch(ctx context.Context, events []LogEvent) error
	AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error
}

// APIKeyRepository defines the interface for validating API keys.
// Implementations are expected to cache results for performance.
type APIKeyRepository interface {
	IsValid(ctx context.Context, key string) (bool, error)
}

// WALRepository defines the interface for a Write-Ahead Log for failover buffering.
type WALRepository interface {
	Write(ctx context.Context, event LogEvent) error
	Replay(ctx context.Context, handler func(event LogEvent) error) error
	Truncate(ctx context.Context) error
}
