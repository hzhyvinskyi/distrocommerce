package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/domain"
	"github.com/hzhyvinskyi/distrocommerce/internal/identity/usecase"
)

var _ usecase.IdentityRepository = (*IdentityRepository)(nil)

const uniqueViolationCode = "23505"

type IdentityRepository struct {
	db *pgxpool.Pool
}

func NewIdentityRepository(db *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{
		db: db,
	}
}

func (r *IdentityRepository) Save(ctx context.Context, identity *domain.Identity) error {
	roles := rolesToStrings(identity.Roles())

	_, err := r.db.Exec(ctx, `
		INSERT INTO identities (id, email, password_hash, roles, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		identity.ID().String(), identity.Email().String(), identity.PasswordHash().String(), roles,
		identity.IsActive(), identity.CreatedAt(), identity.UpdatedAt(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailAlreadyExists
		}

		return fmt.Errorf("insert identity: %w", err)
	}

	return nil
}

func rolesToStrings(roles []domain.Role) []string {
	s := make([]string, len(roles))
	for i, r := range roles {
		s[i] = string(r)
	}
	return s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
