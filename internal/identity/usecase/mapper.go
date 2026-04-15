package usecase

import "github.com/hzhyvinskyi/distrocommerce/internal/identity/domain"

func toRegisterOutput(identity *domain.Identity) RegisterOutput {
	return RegisterOutput{
		IdentityID: identity.ID().String(),
		Email:      identity.Email().String(),
	}
}
