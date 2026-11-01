package errx

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
	ErrInvalid         = errors.New("invalid")
	ErrNotImplemented  = errors.New("not implemented")
	ErrRateLimited     = errors.New("rate limited")
	ErrUpstream        = errors.New("upstream failure")
)

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

func New(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func Invalid(code, message string) error {
	return New(code, message, ErrInvalid)
}

func Conflict(code, message string) error {
	return New(code, message, ErrConflict)
}

func Forbidden(code, message string) error {
	return New(code, message, ErrForbidden)
}

func RateLimited(code, message string) error {
	return New(code, message, ErrRateLimited)
}

// Upstream marks a failure of an external dependency (e.g. an AI provider).
func Upstream(code, message string) error {
	return New(code, message, ErrUpstream)
}

func Unauthenticated(message string) error {
	return New("unauthenticated", message, ErrUnauthenticated)
}

func NotFound(code, message string) error {
	return New(code, message, ErrNotFound)
}
