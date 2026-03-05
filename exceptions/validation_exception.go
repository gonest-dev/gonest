package exceptions

import (
	"github.com/leandroluk/gonest/validator"
)

// ValidationException represents validation errors
type ValidationException struct {
	*HttpException
	ValidationResult *validator.ValidationResult
}

// NewValidationException creates a validation exception
func NewValidationException(result *validator.ValidationResult) *ValidationException {
	return &ValidationException{
		HttpException: &HttpException{
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
