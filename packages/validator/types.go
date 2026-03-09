// gonest/validator/types.go
package validator

import "context"

// Validator is a function that validates a value of type T
type Validator[T any] func(T) *FieldError

// ContextValidator validates with context (for async validations)
type ContextValidator[T any] func(context.Context, T) *FieldError

// ValidationResult contains the result of a validation
type ValidationResult struct {
	valid  bool
	errors []*FieldError
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		valid:  true,
		errors: make([]*FieldError, 0),
	}
}

// Valid returns whether the validation passed
func (vr *ValidationResult) Valid() bool {
	return vr.valid
}

// Invalid returns whether the validation failed
func (vr *ValidationResult) Invalid() bool {
	return !vr.valid
}

// Errors returns all validation errors
func (vr *ValidationResult) Errors() []*FieldError {
	return vr.errors
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(err *FieldError) {
	if err != nil {
		vr.valid = false
		vr.errors = append(vr.errors, err)
	}
}

// Merge merges another validation result into this one
func (vr *ValidationResult) Merge(other *ValidationResult) {
	if other != nil && !other.valid {
		vr.valid = false
		vr.errors = append(vr.errors, other.errors...)
	}
}

// Error implements the error interface
func (vr *ValidationResult) Error() string {
	if vr.valid {
		return ""
	}

	if len(vr.errors) == 0 {
		return "validation failed"
	}

	return vr.errors[0].Error()
}

// First returns the first error (useful for simple display)
func (vr *ValidationResult) First() *FieldError {
	if len(vr.errors) > 0 {
		return vr.errors[0]
	}
	return nil
}

// HasField checks if there's an error for a specific field
func (vr *ValidationResult) HasField(field string) bool {
	for _, err := range vr.errors {
		if err.Field() == field {
			return true
		}
	}
	return false
}

// GetFieldErrors returns all errors for a specific field
func (vr *ValidationResult) GetFieldErrors(field string) []*FieldError {
	var fieldErrors []*FieldError
	for _, err := range vr.errors {
		if err.Field() == field {
			fieldErrors = append(fieldErrors, err)
		}
	}
	return fieldErrors
}

// ToJSON converts the validation result to JSON-friendly format
func (vr *ValidationResult) ToJSON() map[string]any {
	if vr.valid {
		return map[string]any{
			"valid": true,
		}
	}

	errorsByField := make(map[string][]map[string]any)

	for _, err := range vr.errors {
		field := err.Field()
		errorsByField[field] = append(errorsByField[field], map[string]any{
			"code":    err.Code(),
			"message": err.Message(),
			"params":  err.Params(),
		})
	}

	return map[string]any{
		"valid":  false,
		"errors": errorsByField,
	}
}

// Count returns the number of errors
func (vr *ValidationResult) Count() int {
	return len(vr.errors)
}


