package usecase

type ErrorType string

const (
	ErrValidation   ErrorType = "validation"
	ErrBadRequest   ErrorType = "bad_request"
	ErrConflict     ErrorType = "conflict"
	ErrUnauthorized ErrorType = "unauthorized"
	ErrInternal     ErrorType = "internal"
)

type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewValidationError(msg string) *AppError {
	return &AppError{
		Type:    ErrValidation,
		Message: msg,
	}
}

func NewConflictError(msg string) *AppError {
	return &AppError{
		Type:    ErrConflict,
		Message: msg,
	}
}

func NewInternalError(msg string, err error) *AppError {
	return &AppError{
		Type:    ErrInternal,
		Message: msg,
		Err:     err,
	}
}
