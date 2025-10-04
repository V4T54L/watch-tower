package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type postgresS3ChunkRepository struct {
	db *sql.DB
}

func NewPostgresS3ChunkRepository(db *sql.DB) domain.S3ChunkRepository {
	return &postgresS3ChunkRepository{db: db}
}

// Store inserts a new S3 chunk metadata record.
func (r *postgresS3ChunkRepository) Store(ctx context.Context, chunk *domain.S3ChunkMetadata) error {
	query := `
        INSERT INTO s3_chunks (id, tenant_id, s3_key, start_ts, end_ts, record_count, bloom_filter)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	// Replace ? with $1, $2, ... if using PostgreSQL with `lib/pq` or compatible driver.
	query = `
        INSERT INTO s3_chunks (id, tenant_id, s3_key, start_ts, end_ts, record_count, bloom_filter)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

	_, err := r.db.ExecContext(ctx, query,
		chunk.ID,
		chunk.TenantID,
		chunk.S3Key,
		chunk.StartTS,
		chunk.EndTS,
		chunk.RecordCount,
		chunk.BloomFilter,
	)

	return err
}

// FindChunksByTimeRange returns all S3 chunks overlapping the given time range.
func (r *postgresS3ChunkRepository) FindChunksByTimeRange(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]*domain.S3ChunkMetadata, error) {
	query := `
        SELECT id, tenant_id, s3_key, start_ts, end_ts, record_count, created_at
        FROM s3_chunks
        WHERE tenant_id = $1 AND start_ts <= $3 AND end_ts >= $2
        ORDER BY start_ts DESC
    `

	rows, err := r.db.QueryContext(ctx, query, tenantID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*domain.S3ChunkMetadata
	for rows.Next() {
		var chunk domain.S3ChunkMetadata
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TenantID,
			&chunk.S3Key,
			&chunk.StartTS,
			&chunk.EndTS,
			&chunk.RecordCount,
			&chunk.CreatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, &chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}
