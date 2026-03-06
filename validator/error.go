// gonest/validator/error.go
package validator

import "fmt"

// FieldError represents a validation error for a specific field
type FieldError struct {
	field   string
	code    string
	message string
	params  map[string]any
}

// NewFieldError creates a new field error
func NewFieldError(field, code, message string) *FieldError {
	return &FieldError{
		field:   field,
		code:    code,
		message: message,
		params:  make(map[string]any),
	}
}

// Field returns the field name
func (e *FieldError) Field() string {
	return e.field
}

// Code returns the error code
func (e *FieldError) Code() string {
	return e.code
}

// Message returns the error message
func (e *FieldError) Message() string {
	return e.message
}

// Params returns the error parameters
func (e *FieldError) Params() map[string]any {
	return e.params
}

// WithParam adds a parameter to the error
func (e *FieldError) WithParam(key string, value any) *FieldError {
	e.params[key] = value
	return e
}

// WithParams adds multiple parameters to the error
func (e *FieldError) WithParams(params map[string]any) *FieldError {
	for k, v := range params {
		e.params[k] = v
	}
	return e
}

// Error implements the error interface
func (e *FieldError) Error() string {
	if e.field != "" {
		return fmt.Sprintf("%s: %s", e.field, e.message)
	}
	return e.message
}

// ErrorCode represents standard validation error codes
type ErrorCode string

const (
	// ErrorCodeREQUIRED - field is required
	ErrorCodeREQUIRED ErrorCode = "required"

	// ErrorCodeEMAIL - invalid email format
	ErrorCodeEMAIL ErrorCode = "email"

	// ErrorCodeMIN - value below minimum
	ErrorCodeMIN ErrorCode = "min"

	// ErrorCodeMAX - value above maximum
	ErrorCodeMAX ErrorCode = "max"

	// ErrorCodeMINLENGTH - string too short
	ErrorCodeMINLENGTH ErrorCode = "min_length"

	// ErrorCodeMAXLENGTH - string too long
	ErrorCodeMAXLENGTH ErrorCode = "max_length"

	// ErrorCodePATTERN - doesn't match pattern
	ErrorCodePATTERN ErrorCode = "pattern"

	// ErrorCodeURL - invalid URL format
	ErrorCodeURL ErrorCode = "url"

	// ErrorCodeUUID - invalid UUID format
	ErrorCodeUUID ErrorCode = "uuid"

	// ErrorCodeONEOF - value not in allowed list
	ErrorCodeONEOF ErrorCode = "one_of"

	// ErrorCodeCUSTOM - custom validation failed
	ErrorCodeCUSTOM ErrorCode = "custom"
)
