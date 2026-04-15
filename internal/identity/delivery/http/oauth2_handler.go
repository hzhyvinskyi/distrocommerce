package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/usecase"
)

type OAuth2Handler struct {
	registerUseCase usecase.RegisterUseCase
	validate        *validator.Validate
}

func NewOAuth2Handler(registerUseCase usecase.RegisterUseCase, validate *validator.Validate) *OAuth2Handler {
	return &OAuth2Handler{
		registerUseCase: registerUseCase,
		validate:        validate,
	}
}

func (h *OAuth2Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, errBadRequest, err.Error())
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	input := toRegisterInput(req)
	out, err := h.registerUseCase.Execute(r.Context(), input)
	if err != nil {
		mapUseCaseError(w, err)
		return
	}

	resp := toRegisterResponse(out)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}
