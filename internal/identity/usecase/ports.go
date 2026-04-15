package usecase

import (
	"context"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/domain"
)

type IdentityRepository interface {
	Save(ctx context.Context, identity *domain.Identity) error
}
