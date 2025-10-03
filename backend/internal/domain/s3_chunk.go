package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// S3ChunkMetadata represents the metadata for a compressed log chunk stored in S3.
type S3ChunkMetadata struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	S3Key       string    `json:"s3_key"`
	StartTS     time.Time `json:"start_ts"`
	EndTS       time.Time `json:"end_ts"`
	RecordCount int64     `json:"record_count"`
	BloomFilter []byte    `json:"-"` // Not exposed in JSON
	CreatedAt   time.Time `json:"created_at"`
}

// S3ChunkRepository defines the interface for interacting with S3 chunk metadata storage.
type S3ChunkRepository interface {
	Store(ctx context.Context, chunk *S3ChunkMetadata) error
	FindChunksByTimeRange(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]*S3ChunkMetadata, error)
}
