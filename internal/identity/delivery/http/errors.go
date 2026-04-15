package http

import (
	"encoding/json"
	"net/http"
)

type ErrorType string

const (
	errValidation ErrorType = "validation_error"
	errBadRequest ErrorType = "bad_request"
)

type fieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type oauthError struct {
	Error   string       `json:"error"`
	Message string       `json:"message,omitempty"`
	Errors  []fieldError `json:"errors,omitempty"`
}

func writeOAuthError(w http.ResponseWriter, status int, errType ErrorType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	json.NewEncoder(w).Encode(oauthError{ //nolint:errcheck
		Error:   string(errType),
		Message: msg,
	})
}
