package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/V4T54L/watch-tower/pkg/util"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type authService struct {
	userRepo  domain.UserRepository
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthService(userRepo domain.UserRepository, jwtSecret string) AuthUseCase {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		jwtExpiry: 24 * time.Hour,
	}
}

func (s *authService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if !util.CheckPasswordHash(password, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}

	token, err := util.GenerateToken(user.ID, user.TenantID, user.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *authService) Signup(ctx context.Context, email, password string) (string, error) {
	return "", errors.New("not implemented")
}
