// gonest/validator/rules/string.go
package rules

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/gonest-dev/gonest/packages/validator"
)

// MinLength validates minimum string length
func MinLength(minLength int) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if len(value) < minLength {
			return validator.
				NewFieldError("", string(validator.ErrorCodeMINLENGTH), "String is too short").
				WithParam("min", minLength).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// MaxLength validates maximum string length
func MaxLength(maxLength int) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if len(value) > maxLength {
			return validator.
				NewFieldError("", string(validator.ErrorCodeMAXLENGTH), "String is too long").
				WithParam("max", maxLength).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// Length validates exact string length
func Length(length int) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if len(value) != length {
			return validator.
				NewFieldError("", "length", "String must be exactly the specified length").
				WithParam("expected", length).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// Email validates email format
func Email() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if value == "" {
			return nil // Use Required() for empty check
		}

		_, err := mail.ParseAddress(value)
		if err != nil {
			return validator.
				NewFieldError("", string(validator.ErrorCodeEMAIL), "Invalid email format")
		}
		return nil
	}
}

// URL validates URL format
func URL() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		_, err := url.ParseRequestURI(value)
		if err != nil {
			return validator.
				NewFieldError("", string(validator.ErrorCodeURL), "Invalid URL format")
		}
		return nil
	}
}

// Pattern validates against a regex pattern
func Pattern(pattern string) validator.Validator[string] {
	re := regexp.MustCompile(pattern)

	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !re.MatchString(value) {
			return validator.
				NewFieldError("", string(validator.ErrorCodePATTERN), "Value doesn't match the required pattern").
				WithParam("pattern", pattern)
		}
		return nil
	}
}

// UUID validates UUID format (v4)
func UUID() validator.Validator[string] {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !uuidRegex.MatchString(strings.ToLower(value)) {
			return validator.
				NewFieldError("", string(validator.ErrorCodeUUID), "Invalid UUID format")
		}
		return nil
	}
}

// AlphaNumeric validates that string contains only letters and numbers
func AlphaNumeric() validator.Validator[string] {
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !re.MatchString(value) {
			return validator.
				NewFieldError("", "alpha_numeric", "Value must contain only letters and numbers")
		}
		return nil
	}
}

// Alpha validates that string contains only letters
func Alpha() validator.Validator[string] {
	re := regexp.MustCompile(`^[a-zA-Z]+$`)

	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !re.MatchString(value) {
			return validator.
				NewFieldError("", "alpha", "Value must contain only letters")
		}
		return nil
	}
}

// Numeric validates that string contains only numbers
func Numeric() validator.Validator[string] {
	re := regexp.MustCompile(`^[0-9]+$`)

	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !re.MatchString(value) {
			return validator.
				NewFieldError("", "numeric", "Value must contain only numbers")
		}
		return nil
	}
}

// Contains validates that string contains a substring
func Contains(substr string) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !strings.Contains(value, substr) {
			return validator.
				NewFieldError("", "contains", "Value must contain the specified substring").
				WithParam("substring", substr)
		}
		return nil
	}
}

// StartsWith validates that string starts with a prefix
func StartsWith(prefix string) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !strings.HasPrefix(value, prefix) {
			return validator.
				NewFieldError("", "starts_with", "Value must start with the specified prefix").
				WithParam("prefix", prefix)
		}
		return nil
	}
}

// EndsWith validates that string ends with a suffix
func EndsWith(suffix string) validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if value == "" {
			return nil
		}

		if !strings.HasSuffix(value, suffix) {
			return validator.
				NewFieldError("", "ends_with", "Value must end with the specified suffix").
				WithParam("suffix", suffix)
		}
		return nil
	}
}

// HasUpperCase validates that string contains at least one uppercase letter
func HasUpperCase() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		for _, c := range value {
			if c >= 'A' && c <= 'Z' {
				return nil
			}
		}
		return validator.
			NewFieldError("", "has_uppercase", "Value must contain at least one uppercase letter")
	}
}

// HasLowerCase validates that string contains at least one lowercase letter
func HasLowerCase() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		for _, c := range value {
			if c >= 'a' && c <= 'z' {
				return nil
			}
		}
		return validator.
			NewFieldError("", "has_lowercase", "Value must contain at least one lowercase letter")
	}
}

// HasDigit validates that string contains at least one digit
func HasDigit() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		for _, c := range value {
			if c >= '0' && c <= '9' {
				return nil
			}
		}
		return validator.
			NewFieldError("", "has_digit", "Value must contain at least one digit")
	}
}

// HasSpecialChar validates that string contains at least one special character
func HasSpecialChar() validator.Validator[string] {
	specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"

	return func(value string) *validator.FieldError {
		for _, c := range value {
			if strings.ContainsRune(specialChars, c) {
				return nil
			}
		}
		return validator.
			NewFieldError("", "has_special_char", "Value must contain at least one special character").
			WithParam("allowed", specialChars)
	}
}

// StrongPassword validates a strong password (8+ chars, upper, lower, digit, special)
func StrongPassword() validator.Validator[string] {
	return func(value string) *validator.FieldError {
		if len(value) < 8 {
			return validator.
				NewFieldError("", "strong_password", "Password must be at least 8 characters long")
		}

		hasUpper := false
		hasLower := false
		hasDigit := false
		hasSpecial := false
		specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"

		for _, c := range value {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
			}
			if c >= 'a' && c <= 'z' {
				hasLower = true
			}
			if c >= '0' && c <= '9' {
				hasDigit = true
			}
			if strings.ContainsRune(specialChars, c) {
				hasSpecial = true
			}
		}

		if !hasUpper {
			return validator.
				NewFieldError("", "strong_password", "Password must contain at least one uppercase letter")
		}
		if !hasLower {
			return validator.
				NewFieldError("", "strong_password", "Password must contain at least one lowercase letter")
		}
		if !hasDigit {
			return validator.
				NewFieldError("", "strong_password", "Password must contain at least one digit")
		}
		if !hasSpecial {
			return validator.
				NewFieldError("", "strong_password", "Password must contain at least one special character")
		}

		return nil
	}
}


