package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type postgresAlertRuleRepository struct {
	db *sql.DB
}

func NewPostgresAlertRuleRepository(db *sql.DB) domain.AlertRuleRepository {
	return &postgresAlertRuleRepository{db: db}
}

func (r *postgresAlertRuleRepository) Store(ctx context.Context, rule *domain.AlertRule) error {
	query := `
        INSERT INTO alert_rules (id, tenant_id, name, description, query, threshold, interval_seconds, notification_channel, is_enabled, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, rule.ID, rule.TenantID, rule.Name, rule.Description, rule.Query, rule.Threshold, rule.IntervalSeconds, rule.NotificationChannel, rule.IsEnabled, now, now)
	return err
}

func (r *postgresAlertRuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.AlertRule, error) {
	query := `
        SELECT id, tenant_id, name, description, query, threshold, interval_seconds, notification_channel, is_enabled, created_at, updated_at
        FROM alert_rules WHERE id = $1
    `
	var rule domain.AlertRule
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.TenantID, &rule.Name, &rule.Description, &rule.Query, &rule.Threshold,
		&rule.IntervalSeconds, &rule.NotificationChannel, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		// if errors.Is(err, pgx.ErrNoRows) {
		// 	return nil, domain.ErrNotFound
		// }
		return nil, err
	}
	return &rule, nil
}

func (r *postgresAlertRuleRepository) FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlertRule, error) {
	query := `
        SELECT id, tenant_id, name, description, query, threshold, interval_seconds, notification_channel, is_enabled, created_at, updated_at
        FROM alert_rules WHERE tenant_id = $1 ORDER BY created_at DESC
    `
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.AlertRule
	for rows.Next() {
		var rule domain.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.TenantID, &rule.Name, &rule.Description, &rule.Query, &rule.Threshold,
			&rule.IntervalSeconds, &rule.NotificationChannel, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *postgresAlertRuleRepository) FindAllEnabled(ctx context.Context) ([]*domain.AlertRule, error) {
	query := `
        SELECT id, tenant_id, name, description, query, threshold, interval_seconds, notification_channel, is_enabled, created_at, updated_at
        FROM alert_rules WHERE is_enabled = TRUE
    `
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.AlertRule
	for rows.Next() {
		var rule domain.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.TenantID, &rule.Name, &rule.Description, &rule.Query, &rule.Threshold,
			&rule.IntervalSeconds, &rule.NotificationChannel, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *postgresAlertRuleRepository) Update(ctx context.Context, rule *domain.AlertRule) error {
	query := `
        UPDATE alert_rules
        SET name = $2, description = $3, query = $4, threshold = $5, interval_seconds = $6, notification_channel = $7, is_enabled = $8, updated_at = $9
        WHERE id = $1
    `
	_, err := r.db.ExecContext(ctx, query, rule.ID, rule.Name, rule.Description, rule.Query, rule.Threshold, rule.IntervalSeconds, rule.NotificationChannel, rule.IsEnabled, time.Now())
	return err
}

func (r *postgresAlertRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM alert_rules WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
