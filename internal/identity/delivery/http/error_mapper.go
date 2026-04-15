package http

import (
	"errors"
	"net/http"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/usecase"
)

func mapUseCaseError(w http.ResponseWriter, err error) {
	if appErr, ok := errors.AsType[*usecase.AppError](err); ok {
		switch appErr.Type {
		case usecase.ErrValidation:
			fallthrough
		case usecase.ErrBadRequest:
			writeOAuthError(w, http.StatusBadRequest, "bad_request", appErr.Message)

		case usecase.ErrConflict:
			writeOAuthError(w, http.StatusConflict, "conflict", appErr.Message)

		case usecase.ErrUnauthorized:
			writeOAuthError(w, http.StatusUnauthorized, "unauthorized", appErr.Message)

		default:
			writeOAuthError(w, http.StatusInternalServerError, "internal_error", "unexpected error")
		}

		return
	}

	writeOAuthError(w, http.StatusInternalServerError, "internal_error", "unexpected error")
}
