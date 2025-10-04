package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type postgresTenantRepository struct {
	db *sql.DB
}

func NewPostgresTenantRepository(db *sql.DB) domain.TenantRepository {
	return &postgresTenantRepository{db: db}
}

func (r *postgresTenantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	query := `
        SELECT id, name, api_key, created_at, updated_at
        FROM tenants
        WHERE id = $1
    `

	var tenant domain.Tenant
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.APIKey,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("find by ID: %w", err)
	}

	return &tenant, nil
}

func (r *postgresTenantRepository) FindByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	query := `
        SELECT id, name, api_key, created_at, updated_at
        FROM tenants
        WHERE api_key = $1
    `

	var tenant domain.Tenant
	err := r.db.QueryRowContext(ctx, query, apiKey).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.APIKey,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("find by API key: %w", err)
	}

	return &tenant, nil
}

func (r *postgresTenantRepository) Store(ctx context.Context, t *domain.Tenant) error {
	query := `
        INSERT INTO tenants (id, name, api_key, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5)
    `

	_, err := r.db.ExecContext(ctx, query,
		t.ID,
		t.Name,
		t.APIKey,
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("store tenant: %w", err)
	}

	return nil
}
