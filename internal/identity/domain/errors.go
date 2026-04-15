package domain

import "errors"

var (
	ErrIdentityNotFound   = errors.New("identity not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrWeakPassword       = errors.New("password must be at least 8 characters and contain a letter and a digit")
)
