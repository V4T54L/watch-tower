package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Tenant represents an isolated customer account.
type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"-"` // Not exposed in API responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TenantRepository defines the interface for tenant persistence.
type TenantRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*Tenant, error)
	Store(ctx context.Context, t *Tenant) error
}
