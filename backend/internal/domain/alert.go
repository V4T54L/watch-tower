package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AlertRule represents a rule that defines conditions for triggering an alert.
type AlertRule struct {
	ID                  uuid.UUID `json:"id"`
	TenantID            uuid.UUID `json:"tenant_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Query               string    `json:"query"`
	Threshold           int       `json:"threshold"`
	IntervalSeconds     int       `json:"interval_seconds"`
	NotificationChannel string    `json:"notification_channel"`
	IsEnabled           bool      `json:"is_enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AlertRuleRepository defines the methods for interacting with alert rule persistence.
type AlertRuleRepository interface {
	Store(ctx context.Context, rule *AlertRule) error
	FindByID(ctx context.Context, id uuid.UUID) (*AlertRule, error)
	FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*AlertRule, error)
	FindAllEnabled(ctx context.Context) ([]*AlertRule, error)
	Update(ctx context.Context, rule *AlertRule) error
	Delete(ctx context.Context, id uuid.UUID) error
}
