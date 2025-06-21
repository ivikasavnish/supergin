package supergin

import "fmt"

// ErrorCode represents different types of SuperGin errors
type ErrorCode string

const (
	ErrRouteNotFound      ErrorCode = "ROUTE_NOT_FOUND"
	ErrValidationFailed   ErrorCode = "VALIDATION_FAILED"
	ErrDIServiceNotFound  ErrorCode = "DI_SERVICE_NOT_FOUND"
	ErrCircularDependency ErrorCode = "CIRCULAR_DEPENDENCY"
	ErrInvalidFactory     ErrorCode = "INVALID_FACTORY"
	ErrContextRequired    ErrorCode = "CONTEXT_REQUIRED"
)

// SuperGinError represents an error within the SuperGin framework
type SuperGinError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface
func (e *SuperGinError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *SuperGinError) Unwrap() error {
	return e.Cause
}

// NewSuperGinError creates a new SuperGin error
func NewSuperGinError(code ErrorCode, message string, args ...interface{}) *SuperGinError {
	return &SuperGinError{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
	}
}

// NewSuperGinErrorWithCause creates a new SuperGin error with a cause
func NewSuperGinErrorWithCause(code ErrorCode, cause error, message string, args ...interface{}) *SuperGinError {
	return &SuperGinError{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
		Cause:   cause,
	}
}

// IsErrorCode checks if an error is a SuperGin error with specific code
func IsErrorCode(err error, code ErrorCode) bool {
	if sgErr, ok := err.(*SuperGinError); ok {
		return sgErr.Code == code
	}
	return false
}
