package util

import (
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims defines the custom claims for the JWT.
type Claims struct {
	UserID   uuid.UUID       `json:"user_id"`
	TenantID uuid.UUID       `json:"tenant_id"`
	Role     domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for a given user.
func GenerateToken(userID, tenantID uuid.UUID, role domain.UserRole, secretKey string, expiry time.Duration) (string, error) {
	expirationTime := time.Now().Add(expiry)
	claims := &Claims{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ValidateToken parses and validates a JWT string.
func ValidateToken(tokenString, secretKey string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
