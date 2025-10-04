package usecase

import (
	"context"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

// AuthUseCase defines the contract for authentication services.
type AuthUseCase interface {
	Login(ctx context.Context, email, password string) (string, error)
	Signup(ctx context.Context, email, password string) (string, error)
}

// IngestionUseCase defines the contract for log ingestion services.
type IngestionUseCase interface {
	IngestBatch(ctx context.Context, tenantID uuid.UUID, logs []IngestRequest) (accepted, rejected int, err error)
}

// SearchUseCase defines the contract for log search services.
type SearchUseCase interface {
	Search(ctx context.Context, params SearchParams) (*SearchResult, error)
}

// TailUseCase defines the contract for real-time log tailing services.
type TailUseCase interface {
	Tail(ctx context.Context, tenantID uuid.UUID, filters map[string]string) (<-chan *domain.Log, error)
}

// AlertUseCase defines the contract for managing alert rules.
type AlertUseCase interface {
	CreateRule(ctx context.Context, rule *domain.AlertRule) error
	GetRule(ctx context.Context, id, tenantID uuid.UUID) (*domain.AlertRule, error)
	ListRules(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlertRule, error)
	UpdateRule(ctx context.Context, rule *domain.AlertRule) error
	DeleteRule(ctx context.Context, id, tenantID uuid.UUID) error
}

// TailingPublisher defines the contract for publishing logs to a real-time channel.
type TailingPublisher interface {
	Publish(ctx context.Context, tenantID uuid.UUID, log *domain.Log) error
}

// TailingSubscriber defines the contract for subscribing to a real-time log channel.
type TailingSubscriber interface {
	Subscribe(ctx context.Context, tenantID uuid.UUID) (<-chan *domain.Log, error)
}

// IngestRequest represents a single log entry for ingestion.
type IngestRequest struct {
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Service   string    `json:"service"`
}

// SearchParams defines the parameters for a log search.
type SearchParams struct {
	TenantID uuid.UUID
	Query    string
	Start    time.Time
	End      time.Time
	Limit    int
	Cursor   string
}

// SearchResult holds the results of a log search.
type SearchResult struct {
	Logs       []*domain.Log `json:"logs"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
