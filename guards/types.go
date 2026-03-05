package guards

import (
	"github.com/gonest-dev/gonest/core"
)

// ExecutionContext provides context for guard execution
type ExecutionContext struct {
	Context  *core.Context
	Handler  core.HandlerFunc
	Metadata map[string]any
}

// Guard interface that all guards must implement
type Guard interface {
	// CanActivate determines if the request can proceed
	CanActivate(ctx *ExecutionContext) (bool, error)
}

// GuardFunc is a function type that implements Guard
type GuardFunc func(*ExecutionContext) (bool, error)

// CanActivate implements Guard interface for GuardFunc
func (f GuardFunc) CanActivate(ctx *ExecutionContext) (bool, error) {
	return f(ctx)
}

// GuardError represents a guard rejection error
type GuardError struct {
	Message    string
	StatusCode int
	Details    map[string]any
}

func (e *GuardError) Error() string {
	return e.Message
}

// NewGuardError creates a new guard error
func NewGuardError(message string, statusCode int) *GuardError {
	return &GuardError{
		Message:    message,
		StatusCode: statusCode,
		Details:    make(map[string]any),
	}
}

// WithDetail adds a detail to the error
func (e *GuardError) WithDetail(key string, value any) *GuardError {
	e.Details[key] = value
	return e
}

// ToJSON converts error to JSON response
func (e *GuardError) ToJSON() map[string]any {
	result := map[string]any{
		"statusCode": e.StatusCode,
		"message":    e.Message,
	}

	if len(e.Details) > 0 {
		result["details"] = e.Details
	}

	return result
}
