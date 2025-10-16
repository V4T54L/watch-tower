package postgres

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/lib/pq"
)

// LogRepository implements the sink part of the domain.LogRepository interface for PostgreSQL.
type LogRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewLogRepository creates a new PostgreSQL log repository.
func NewLogRepository(db *sql.DB, logger *slog.Logger) domain.LogRepository {
	return &LogRepository{db: db, logger: logger}
}

// WriteLogBatch writes a batch of log events to PostgreSQL using the COPY protocol for high performance.
// It uses an ON CONFLICT clause to perform an upsert, ensuring idempotency based on event_id.
func (r *LogRepository) WriteLogBatch(ctx context.Context, events []domain.LogEvent) error {
	if len(events) == 0 {
		return nil
	}

	txn, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txn.Rollback() // Rollback is a no-op if Commit() is called

	// Use a temporary table to stage the data, then merge into the main table.
	// This is a common pattern for high-performance, idempotent bulk inserts.
	tempTableName := "logs_temp_import"
	_, err = txn.ExecContext(ctx, `CREATE TEMP TABLE `+tempTableName+` (LIKE logs INCLUDING DEFAULTS) ON COMMIT DROP;`)
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn(tempTableName, "event_id", "received_at", "event_time", "source", "level", "message", "metadata"))
	if err != nil {
		return err
	}

	for _, event := range events {
		_, err = stmt.ExecContext(ctx, event.ID, event.ReceivedAt, event.EventTime, event.Source, event.Level, event.Message, event.Metadata)
		if err != nil {
			// Close the statement to avoid connection issues
			_ = stmt.Close()
			return err
		}
	}

	if err := stmt.Close(); err != nil {
		return err
	}

	// Upsert from the temp table into the main table
	upsertQuery := `
		INSERT INTO logs (event_id, received_at, event_time, source, level, message, metadata)
		SELECT event_id, received_at, event_time, source, level, message, metadata FROM ` + tempTableName + `
		ON CONFLICT (event_id) DO UPDATE SET
			received_at = EXCLUDED.received_at,
			event_time = EXCLUDED.event_time,
			source = EXCLUDED.source,
			level = EXCLUDED.level,
			message = EXCLUDED.message,
			metadata = EXCLUDED.metadata;
	`
	_, err = txn.ExecContext(ctx, upsertQuery)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (r *LogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	query := `
		INSERT INTO log_buffer (id, received_at, event_time, source, level, message, metadata, consumer_group, acknowledged, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, false, 0)
		ON CONFLICT (id) DO NOTHING;
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.ReceivedAt, event.EventTime, event.Source, event.Level, event.Message, event.Metadata,
	)
	if err != nil {
		r.logger.Error("failed to buffer log", "error", err)
	}
	return err
}

func (r *LogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	query := `
		SELECT id, received_at, event_time, source, level, message, metadata
		FROM log_buffer
		WHERE acknowledged = false AND (consumer_group IS NULL OR consumer_group = $1)
		ORDER BY received_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED;
	`

	rows, err := r.db.QueryContext(ctx, query, group, count)
	if err != nil {
		r.logger.Error("failed to read log batch", "error", err)
		return nil, err
	}
	defer rows.Close()

	var events []domain.LogEvent
	for rows.Next() {
		var e domain.LogEvent
		if err := rows.Scan(&e.ID, &e.ReceivedAt, &e.EventTime, &e.Source, &e.Level, &e.Message, &e.Metadata); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Mark them as "in processing" by updating the consumer group
	// This assumes that only one consumer per group processes these.
	if len(events) > 0 {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			_, err := tx.ExecContext(ctx,
				`UPDATE log_buffer SET consumer_group = $1 WHERE id = $2`, group, e.ID)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return events, nil
}

func (r *LogRepository) AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	query := `
		UPDATE log_buffer
		SET acknowledged = true
		WHERE id = ANY($1) AND consumer_group = $2
	`
	_, err := r.db.ExecContext(ctx, query, pq.Array(messageIDs), group)
	if err != nil {
		r.logger.Error("failed to acknowledge logs", "error", err)
	}
	return err
}

func (r *LogRepository) MoveToDLQ(ctx context.Context, events []domain.LogEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	for _, e := range events {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO log_dlq (id, failed_at, reason, event_time, source, level, message, metadata)
			VALUES ($1, now(), $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO NOTHING
		`, e.ID, "manual DLQ move", e.EventTime, e.Source, e.Level, e.Message, e.Metadata)
		if err != nil {
			tx.Rollback()
			r.logger.Error("failed to move log to DLQ", "error", err)
			return err
		}
		// Optionally, delete from buffer
		_, _ = tx.ExecContext(ctx, `DELETE FROM log_buffer WHERE id = $1`, e.ID)
	}

	return tx.Commit()
}
