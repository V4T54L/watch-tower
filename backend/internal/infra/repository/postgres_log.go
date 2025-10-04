package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type postgresLogRepository struct {
	db *sql.DB
}

func NewPostgresLogRepository(db *sql.DB) domain.LogRepository {
	return &postgresLogRepository{db: db}
}

// StoreBatch inserts a batch of logs into the database using a single transaction.
func (r *postgresLogRepository) StoreBatch(ctx context.Context, logs []*domain.Log) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO logs (id, tenant_id, timestamp, severity, service, message)
        VALUES ($1, $2, $3, $4, $5, $6)
    `)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		_, err := stmt.ExecContext(ctx,
			log.ID,
			log.TenantID,
			log.Timestamp,
			log.Severity,
			log.Service,
			log.Message,
		)
		if err != nil {
			return fmt.Errorf("executing insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Search retrieves logs matching the given tenant, query, time range, and pagination.
func (r *postgresLogRepository) Search(
	ctx context.Context,
	tenantID uuid.UUID,
	query string,
	start, end time.Time,
	limit int,
	cursor string,
) ([]*domain.Log, string, error) {

	args := []interface{}{tenantID, start, end, fmt.Sprintf("%%%s%%", query)}
	argIdx := 5

	sqlQuery := `
        SELECT id, tenant_id, timestamp, severity, service, message
        FROM logs
        WHERE tenant_id = $1 AND timestamp BETWEEN $2 AND $3 AND message ILIKE $4
    `

	if cursor != "" {
		cursorTime, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		sqlQuery += fmt.Sprintf(" AND (timestamp, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, cursorTime, cursorID)
		argIdx += 2
	}

	sqlQuery += fmt.Sprintf(" ORDER BY timestamp DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query logs: %w", err)
	}
	defer rows.Close()

	var logs []*domain.Log
	for rows.Next() {
		var log domain.Log
		if err := rows.Scan(
			&log.ID,
			&log.TenantID,
			&log.Timestamp,
			&log.Severity,
			&log.Service,
			&log.Message,
		); err != nil {
			return nil, "", fmt.Errorf("scan log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(logs) > limit {
		nextCursor = encodeCursor(logs[limit-1].Timestamp, logs[limit-1].ID)
		logs = logs[:limit]
	}

	return logs, nextCursor, nil
}

// encodeCursor generates a base64-encoded string from a timestamp and UUID.
func encodeCursor(t time.Time, id uuid.UUID) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s,%s", t.Format(time.RFC3339Nano), id.String())))
}

// decodeCursor decodes a base64-encoded cursor string into a timestamp and UUID.
func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}

	parts := strings.Split(string(decoded), ",")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}

	return t, id, nil
}
