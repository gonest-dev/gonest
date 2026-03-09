// gonest/exceptions/types.go
package exceptions

import (
	"fmt"
	"net/http"

	"github.com/gonest-dev/gonest/core/common"
)

// HTTPException represents an HTTP error with status code
type HTTPException struct {
	StatusCode int
	Message    string
	Details    map[string]any
	Cause      error
}

// Error implements error interface
func (e *HTTPException) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// WithDetail adds a detail to the exception
func (e *HTTPException) WithDetail(key string, value any) *HTTPException {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// ToJSON converts exception to JSON response
func (e *HTTPException) ToJSON() map[string]any {
	result := map[string]any{
		"statusCode": e.StatusCode,
		"message":    e.Message,
	}

	if len(e.Details) > 0 {
		result["details"] = e.Details
	}

	if e.Cause != nil {
		result["cause"] = e.Cause.Error()
	}

	return result
}

// ExceptionFilter interface for handling exceptions
type ExceptionFilter interface {
	Catch(err error, ctx *common.Context) error
}

// ExceptionFilterFunc is a function type that implements ExceptionFilter
type ExceptionFilterFunc func(error, *common.Context) error

// Catch implements ExceptionFilter interface
func (f ExceptionFilterFunc) Catch(err error, ctx *common.Context) error {
	return f(err, ctx)
}

// Common HTTP Exceptions (4xx Client Errors)

// BadRequestException creates a 400 error
func BadRequestException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusBadRequest,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// UnauthorizedException creates a 401 error
func UnauthorizedException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusUnauthorized,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// PaymentRequiredException creates a 402 error
func PaymentRequiredException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusPaymentRequired,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// ForbiddenException creates a 403 error
func ForbiddenException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusForbidden,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// NotFoundException creates a 404 error
func NotFoundException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusNotFound,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// MethodNotAllowedException creates a 405 error
func MethodNotAllowedException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusMethodNotAllowed,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// NotAcceptableException creates a 406 error
func NotAcceptableException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusNotAcceptable,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// ProxyAuthRequiredException creates a 407 error
func ProxyAuthRequiredException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusProxyAuthRequired,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// RequestTimeoutException creates a 408 error
func RequestTimeoutException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusRequestTimeout,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// ConflictException creates a 409 error
func ConflictException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusConflict,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// GoneException creates a 410 error
func GoneException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusGone,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// LengthRequiredException creates a 411 error
func LengthRequiredException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusLengthRequired,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// PreconditionFailedException creates a 412 error
func PreconditionFailedException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusPreconditionFailed,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// PayloadTooLargeException creates a 413 error
func PayloadTooLargeException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusRequestEntityTooLarge,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// URITooLongException creates a 414 error
func URITooLongException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusRequestURITooLong,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// UnsupportedMediaTypeException creates a 415 error
func UnsupportedMediaTypeException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusUnsupportedMediaType,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// RangeNotSatisfiableException creates a 416 error
func RangeNotSatisfiableException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusRequestedRangeNotSatisfiable,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// ExpectationFailedException creates a 417 error
func ExpectationFailedException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusExpectationFailed,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// TeapotException creates a 418 error
func TeapotException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusTeapot,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// MisdirectedRequestException creates a 421 error
func MisdirectedRequestException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusMisdirectedRequest,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// UnprocessableEntityException creates a 422 error
func UnprocessableEntityException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusUnprocessableEntity,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// LockedException creates a 423 error
func LockedException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusLocked,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// FailedDependencyException creates a 424 error
func FailedDependencyException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusFailedDependency,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// TooEarlyException creates a 425 error
func TooEarlyException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusTooEarly,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// UpgradeRequiredException creates a 426 error
func UpgradeRequiredException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusUpgradeRequired,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// PreconditionRequiredException creates a 428 error
func PreconditionRequiredException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusPreconditionRequired,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// TooManyRequestsException creates a 429 error
func TooManyRequestsException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusTooManyRequests,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// RequestHeaderFieldsTooLargeException creates a 431 error
func RequestHeaderFieldsTooLargeException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusRequestHeaderFieldsTooLarge,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// UnavailableForLegalReasonsException creates a 451 error
func UnavailableForLegalReasonsException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusUnavailableForLegalReasons,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// Common HTTP Exceptions (5xx Server Errors)

// InternalServerErrorException creates a 500 error
func InternalServerErrorException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusInternalServerError,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// ServiceUnavailableException creates a 503 error
func ServiceUnavailableException(message string) *HTTPException {
	return &HTTPException{
		StatusCode: http.StatusServiceUnavailable,
		Message:    message,
		Details:    make(map[string]any),
	}
}

// NewHTTPException creates a custom HTTP exception
func NewHTTPException(statusCode int, message string) *HTTPException {
	return &HTTPException{
		StatusCode: statusCode,
		Message:    message,
		Details:    make(map[string]any),
	}
}


