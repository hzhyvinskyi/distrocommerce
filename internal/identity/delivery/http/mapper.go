package http

import (
	"time"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/usecase"
)

func toRegisterInput(req registerRequest) usecase.RegisterInput {
	return usecase.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}
}

func toRegisterResponse(out usecase.RegisterOutput) registerResponse {
	return registerResponse{
		IdentityID:   out.IdentityID,
		Email:        out.Email,
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresIn:    out.ExpiresAt - time.Now().Unix(),
	}
}
