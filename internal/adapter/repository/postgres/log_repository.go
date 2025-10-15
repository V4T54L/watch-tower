package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/lib/pq"
	"github.com/user/log-ingestor/internal/domain"
)

const logsTableName = "logs"

// LogRepository implements the sink part of the domain.LogRepository interface for PostgreSQL.
type LogRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewLogRepository creates a new PostgreSQL log repository.
func NewLogRepository(db *sql.DB, logger *slog.Logger) *LogRepository {
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

// The following methods are not implemented for the PostgreSQL sink repository.
var errNotImplemented = errors.New("method not implemented for this repository type")

func (r *LogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	return errNotImplemented
}

func (r *LogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	return nil, errNotImplemented
}

func (r *LogRepository) AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error {
	return errNotImplemented
}

