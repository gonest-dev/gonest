// gonest/exceptions/validation_exception.go
package exceptions

import (
	"github.com/gonest-dev/gonest/validator"
)

// ValidationException represents validation errors
type ValidationException struct {
	*HTTPException
	ValidationResult *validator.ValidationResult
}

// NewValidationException creates a validation exception
func NewValidationException(result *validator.ValidationResult) *ValidationException {
	return &ValidationException{
		HTTPException: &HTTPException{
			StatusCode: 400,
			Message:    "Validation failed",
			Details:    result.ToJSON(),
		},
		ValidationResult: result,
	}
}

// ToJSON converts validation exception to JSON
func (e *ValidationException) ToJSON() map[string]any {
	return map[string]any{
		"statusCode": e.StatusCode,
		"message":    e.Message,
		"errors":     e.ValidationResult.ToJSON()["errors"],
	}
}
