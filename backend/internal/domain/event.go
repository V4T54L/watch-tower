package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Log represents a single log entry.
type Log struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
}

// LogRepository defines the interface for log persistence and querying.
// This will handle both hot (Postgres) and cold (S3) storage access.
type LogRepository interface {
	// StoreBatch stores a batch of logs in the hot storage.
	StoreBatch(ctx context.Context, logs []*Log) error

	// Search performs a query against hot and/or cold storage.
	Search(ctx context.Context, tenantID uuid.UUID, query string, start, end time.Time, limit int, cursor string) ([]*Log, string, error)
}
