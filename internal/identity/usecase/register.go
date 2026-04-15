package usecase

import (
	"context"
	"errors"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/domain"
)

var _ RegisterUseCase = (*registerUseCase)(nil)

type registerUseCase struct {
	identityRepo IdentityRepository
}

func NewRegisterUseCase(identityRepo IdentityRepository) RegisterUseCase {
	return &registerUseCase{
		identityRepo: identityRepo,
	}
}

func (uc *registerUseCase) Execute(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	email, err := domain.NewEmail(input.Email)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidEmail) {
			return RegisterOutput{}, NewValidationError("invalid email")
		}

		return RegisterOutput{}, err
	}

	passwordHash, err := domain.HashPassword(input.Password)
	if err != nil {
		if errors.Is(err, domain.ErrWeakPassword) {
			return RegisterOutput{}, NewValidationError("password is weak")
		}

		return RegisterOutput{}, err
	}

	identity, err := domain.NewIdentity(email, passwordHash)
	if err != nil {
		return RegisterOutput{}, err
	}

	if err = uc.identityRepo.Save(ctx, identity); err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			return RegisterOutput{}, NewConflictError("email already exists")
		}

		return RegisterOutput{}, NewInternalError("cannot create account", err)
	}

	return toRegisterOutput(identity), nil
}
