package domain

import "context"

// LogRepository defines the interface for buffering and sinking log events.
// This abstracts away the specific implementations (e.g., Redis Streams, PostgreSQL).
type LogRepository interface {
	// BufferLog adds a single log event to the durable buffer.
	BufferLog(ctx context.Context, event LogEvent) error

	// ReadLogBatch reads a batch of log events from the buffer for a specific consumer.
	ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]LogEvent, error)

	// WriteLogBatch writes a batch of log events to the final structured sink.
	WriteLogBatch(ctx context.Context, events []LogEvent) error

	// AcknowledgeLogs marks a set of log events as successfully processed in the buffer.
	AcknowledgeLogs(ctx context.Context, group string, eventIDs ...string) error
}

// APIKeyRepository defines the interface for validating API keys.
type APIKeyRepository interface {
	// IsValid checks if the provided API key is valid and active.
	// Implementations should handle caching to reduce database load.
	IsValid(ctx context.Context, key string) (bool, error)
}

// WALRepository defines the interface for the Write-Ahead Log failover mechanism.
type WALRepository interface {
	// Write appends a log event to the local WAL file.
	Write(ctx context.Context, event LogEvent) error

	// Replay reads events from the WAL and sends them to a handler function.
	// The handler is responsible for re-buffering the event (e.g., to Redis).
	Replay(ctx context.Context, handler func(event LogEvent) error) error

	// Truncate removes WAL segments that have been successfully replayed.
	Truncate(ctx context.Context) error
}

