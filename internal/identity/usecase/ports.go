package usecase

import (
	"context"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/domain"
)

type IdentityRepository interface {
	Save(ctx context.Context, identity *domain.Identity) error
}

type TokenManager interface {
	GenerateTokenPair(identityID, email string, roles []string) (accessToken, refreshToken string, expiresAt int64, err error)
	ValidateRefreshToken(token string) (identityID string, err error)
}
