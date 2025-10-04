package usecase

import (
	"context"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type alertService struct {
	repo domain.AlertRuleRepository
}

func NewAlertService(repo domain.AlertRuleRepository) AlertUseCase {
	return &alertService{repo: repo}
}

func (s *alertService) CreateRule(ctx context.Context, rule *domain.AlertRule) error {
	return s.repo.Store(ctx, rule)
}

func (s *alertService) GetRule(ctx context.Context, id, tenantID uuid.UUID) (*domain.AlertRule, error) {
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rule.TenantID != tenantID {
		return nil, domain.ErrNotFound // Or a specific permission error
	}
	return rule, nil
}

func (s *alertService) ListRules(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlertRule, error) {
	return s.repo.FindByTenantID(ctx, tenantID)
}

func (s *alertService) UpdateRule(ctx context.Context, rule *domain.AlertRule) error {
	// Additional check to ensure user can only update their own tenant's rule
	existing, err := s.GetRule(ctx, rule.ID, rule.TenantID)
	if err != nil {
		return err
	}
	// Copy over immutable fields if necessary
	rule.CreatedAt = existing.CreatedAt
	return s.repo.Update(ctx, rule)
}

func (s *alertService) DeleteRule(ctx context.Context, id, tenantID uuid.UUID) error {
	// Ensure the rule belongs to the tenant before deleting
	if _, err := s.GetRule(ctx, id, tenantID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
