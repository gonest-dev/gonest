// gonest/validator/rules/boolean.go
package rules

import (
	"github.com/gonest-dev/gonest/packages/validator"
)

// IsTrue validates that boolean value is true
func IsTrue() validator.Validator[bool] {
	return func(value bool) *validator.FieldError {
		if !value {
			return validator.
				NewFieldError("", "is_true", "Value must be true")
		}
		return nil
	}
}

// IsFalse validates that boolean value is false
func IsFalse() validator.Validator[bool] {
	return func(value bool) *validator.FieldError {
		if value {
			return validator.
				NewFieldError("", "is_false", "Value must be false")
		}
		return nil
	}
}

// MustAccept validates that value is true (useful for terms acceptance)
func MustAccept() validator.Validator[bool] {
	return func(value bool) *validator.FieldError {
		if !value {
			return validator.
				NewFieldError("", "must_accept", "You must accept to continue")
		}
		return nil
	}
}

// MustDecline validates that value is false (useful for opt-outs)
func MustDecline() validator.Validator[bool] {
	return func(value bool) *validator.FieldError {
		if value {
			return validator.
				NewFieldError("", "must_decline", "You must decline to continue")
		}
		return nil
	}
}


