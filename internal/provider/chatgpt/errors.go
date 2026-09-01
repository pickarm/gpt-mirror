package chatgpt

import (
	"errors"
	"fmt"
	"time"
)

type ErrorKind string

const (
	ErrorKindAuth           ErrorKind = "auth"
	ErrorKindTransport      ErrorKind = "transport"
	ErrorKindRateLimit      ErrorKind = "rate_limit"
	ErrorKindProtocol       ErrorKind = "protocol"
	ErrorKindNotFound       ErrorKind = "not_found"
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	ErrorKindUnavailable    ErrorKind = "unavailable"
)

// Error is the normalized provider failure surfaced to application services.
// StatusCode is optional protocol metadata and must not be used to expose raw
// transport responses outside a concrete provider implementation.
type Error struct {
	Kind       ErrorKind
	Operation  string
	StatusCode int
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("chatgpt provider %s failed (%s): %v", e.Operation, e.Kind, e.Err)
	}
	return fmt.Sprintf("chatgpt provider %s failed (%s)", e.Operation, e.Kind)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsKind(err error, kind ErrorKind) bool {
	var providerErr *Error
	return errors.As(err, &providerErr) && providerErr.Kind == kind
}

func unavailableError(operation string) error {
	return &Error{
		Kind:      ErrorKindUnavailable,
		Operation: operation,
	}
}
