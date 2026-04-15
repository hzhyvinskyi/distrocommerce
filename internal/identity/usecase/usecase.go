package usecase

import "context"

type RegisterUseCase interface {
	Execute(ctx context.Context, input RegisterInput) (RegisterOutput, error)
}
