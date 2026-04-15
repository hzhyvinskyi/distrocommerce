package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

func parseValidationError(err error) []fieldError {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return []fieldError{
			{
				Field:   "unknown",
				Code:    "invalid",
				Message: err.Error(),
			},
		}
	}

	out := make([]fieldError, 0, len(ve))

	for _, fe := range ve {
		out = append(out, fieldError{
			Field:   fe.Field(),
			Code:    fe.Tag(),
			Message: validationMessage(fe),
		})
	}

	return out
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email"
	case "min":
		return fe.Field() + " is too short"
	default:
		return fe.Field() + " is invalid"
	}
}

func writeValidationError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	json.NewEncoder(w).Encode(oauthError{ //nolint:errcheck
		Error:  string(errValidation),
		Errors: parseValidationError(err),
	})
}
