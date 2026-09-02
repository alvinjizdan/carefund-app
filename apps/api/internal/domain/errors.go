package domain

import "errors"

var (
	ErrNotFound              = errors.New("resource not found")
	ErrDuplicate             = errors.New("resource already exists")
	ErrInvalidInput          = errors.New("invalid input")
	ErrInternalError         = errors.New("internal server error")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrProviderRejected      = errors.New("provider rejected refund request")
)

type ProviderRejectionError struct {
	Message string
}

func (e *ProviderRejectionError) Error() string {
	return e.Message
}
