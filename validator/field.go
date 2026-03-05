package validator

import "context"

// FieldValidator provides type-safe validation for a field
type FieldValidator[T any] struct {
	name            string
	validators      []Validator[T]
	asyncValidators []ContextValidator[T]
	optional        bool
	customMessage   string
}

// Field creates a new field validator
func Field[T any](name string) *FieldValidator[T] {
	return &FieldValidator[T]{
		name:            name,
		validators:      make([]Validator[T], 0),
		asyncValidators: make([]ContextValidator[T], 0),
	}
}

// Optional marks the field as optional
func (fv *FieldValidator[T]) Optional() *FieldValidator[T] {
	fv.optional = true
	return fv
}

// Required marks the field as required (default)
func (fv *FieldValidator[T]) Required() *FieldValidator[T] {
	fv.optional = false
	return fv
}

// WithMessage sets a custom error message
func (fv *FieldValidator[T]) WithMessage(message string) *FieldValidator[T] {
	fv.customMessage = message
	return fv
}

// Is adds a validator to the chain
func (fv *FieldValidator[T]) Is(validator Validator[T]) *FieldValidator[T] {
	fv.validators = append(fv.validators, validator)
	return fv
}

// IsAsync adds an async validator to the chain
func (fv *FieldValidator[T]) IsAsync(validator ContextValidator[T]) *FieldValidator[T] {
	fv.asyncValidators = append(fv.asyncValidators, validator)
	return fv
}

// Must is an alias for Is (more readable in some contexts)
func (fv *FieldValidator[T]) Must(validator Validator[T]) *FieldValidator[T] {
	return fv.Is(validator)
}

// MustAsync is an alias for IsAsync
func (fv *FieldValidator[T]) MustAsync(validator ContextValidator[T]) *FieldValidator[T] {
	return fv.IsAsync(validator)
}

// Check validates the field value synchronously
func (fv *FieldValidator[T]) Check(value T) *FieldError {
	for _, validator := range fv.validators {
		if err := validator(value); err != nil {
			// Apply custom message if set
			if fv.customMessage != "" {
				err.message = fv.customMessage
			}
			// Set field name
			err.field = fv.name
			return err
		}
	}
	return nil
}

// CheckAsync validates the field value asynchronously
func (fv *FieldValidator[T]) CheckAsync(ctx context.Context, value T) *FieldError {
	// First run sync validators
	if err := fv.Check(value); err != nil {
		return err
	}

	// Then run async validators
	for _, validator := range fv.asyncValidators {
		if err := validator(ctx, value); err != nil {
			if fv.customMessage != "" {
				err.message = fv.customMessage
			}
			err.field = fv.name
			return err
		}
	}

	return nil
}

// Validate validates the field and returns a ValidationResult
func (fv *FieldValidator[T]) Validate(value T) *ValidationResult {
	result := NewValidationResult()

	if err := fv.Check(value); err != nil {
		result.AddError(err)
	}

	return result
}

// ValidateAsync validates the field asynchronously
func (fv *FieldValidator[T]) ValidateAsync(ctx context.Context, value T) *ValidationResult {
	result := NewValidationResult()

	if err := fv.CheckAsync(ctx, value); err != nil {
		result.AddError(err)
	}

	return result
}
