package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
)

// UserRole defines the permission level of a user.
type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleViewer  UserRole = "viewer"
	RoleSupport UserRole = "support"
)

// User represents a user account within a tenant.
type User struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Not exposed in API responses
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Store(ctx context.Context, u *User) error
}
