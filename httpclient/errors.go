package httpclient

import (
	"errors"
	"fmt"
)

var (
	ErrRequestFailed    = errors.New("httpclient: request failed")
	ErrServiceError     = errors.New("httpclient: service error")
	ErrDecodeResponse   = errors.New("httpclient: failed to decode response")
	ErrCreateRequest    = errors.New("httpclient: failed to create request")
	ErrEncodeBody       = errors.New("httpclient: failed to encode request body")
	ErrAuthFailed       = errors.New("httpclient: authentication failed")
	ErrResponseTooLarge = errors.New("httpclient: response body too large")
)

type ServiceError struct {
	StatusCode int
	Message    string
	Internal   string
	RequestID  string
}

func (e *ServiceError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	return fmt.Sprintf("httpclient: service returned status %d", e.StatusCode)
}

func (e *ServiceError) Is(target error) bool {
	return errors.Is(target, ErrServiceError)
}

func (e *ServiceError) Unwrap() error {
	return ErrServiceError
}

func NewServiceError(statusCode int, message, internal, requestID string) *ServiceError {
	return &ServiceError{
		StatusCode: statusCode,
		Message:    message,
		Internal:   internal,
		RequestID:  requestID,
	}
}

func IsServiceError(err error) (*ServiceError, bool) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		return svcErr, true
	}

	return nil, false
}
