package pipes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gonest-dev/gonest/core"
)

// ParseIntPipe parses string to int
type ParseIntPipe struct{}

func NewParseIntPipe() *ParseIntPipe {
	return &ParseIntPipe{}
}

func (p *ParseIntPipe) Transform(value any, ctx *core.Context) (int, error) {
	str, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", value)
	}

	result, err := strconv.Atoi(str)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int: %w", err)
	}

	return result, nil
}

// ParseFloatPipe parses string to float64
type ParseFloatPipe struct{}

func NewParseFloatPipe() *ParseFloatPipe {
	return &ParseFloatPipe{}
}

func (p *ParseFloatPipe) Transform(value any, ctx *core.Context) (float64, error) {
	str, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", value)
	}

	result, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float: %w", err)
	}

	return result, nil
}

// ParseBoolPipe parses string to bool
type ParseBoolPipe struct{}

func NewParseBoolPipe() *ParseBoolPipe {
	return &ParseBoolPipe{}
}

func (p *ParseBoolPipe) Transform(value any, ctx *core.Context) (bool, error) {
	str, ok := value.(string)
	if !ok {
		return false, fmt.Errorf("expected string, got %T", value)
	}

	result, err := strconv.ParseBool(str)
	if err != nil {
		return false, fmt.Errorf("failed to parse bool: %w", err)
	}

	return result, nil
}

// ParseUUIDPipe validates UUID format
type ParseUUIDPipe struct{}

func NewParseUUIDPipe() *ParseUUIDPipe {
	return &ParseUUIDPipe{}
}

func (p *ParseUUIDPipe) Transform(value any, ctx *core.Context) (string, error) {
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", value)
	}

	// Simple UUID validation (v4 format)
	str = strings.ToLower(str)
	if len(str) != 36 {
		return "", fmt.Errorf("invalid UUID length")
	}

	// Check format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	parts := strings.Split(str, "-")
	if len(parts) != 5 {
		return "", fmt.Errorf("invalid UUID format")
	}

	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		return "", fmt.Errorf("invalid UUID format")
	}

	return str, nil
}

// ParseEnumPipe validates enum values
type ParseEnumPipe struct {
	allowedValues []string
}

func NewParseEnumPipe(allowedValues ...string) *ParseEnumPipe {
	return &ParseEnumPipe{
		allowedValues: allowedValues,
	}
}

func (p *ParseEnumPipe) Transform(value any, ctx *core.Context) (string, error) {
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", value)
	}

	for _, allowed := range p.allowedValues {
		if str == allowed {
			return str, nil
		}
	}

	return "", fmt.Errorf("invalid enum value '%s', allowed: %v", str, p.allowedValues)
}

// ParseArrayPipe parses comma-separated string to array
type ParseArrayPipe struct {
	separator string
}

func NewParseArrayPipe(separator ...string) *ParseArrayPipe {
	sep := ","
	if len(separator) > 0 {
		sep = separator[0]
	}

	return &ParseArrayPipe{
		separator: sep,
	}
}

func (p *ParseArrayPipe) Transform(value any, ctx *core.Context) ([]string, error) {
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", value)
	}

	if str == "" {
		return []string{}, nil
	}

	parts := strings.Split(str, p.separator)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, nil
}

// DefaultValuePipe provides default value if input is empty
type DefaultValuePipe struct {
	defaultValue any
}

func NewDefaultValuePipe(defaultValue any) *DefaultValuePipe {
	return &DefaultValuePipe{
		defaultValue: defaultValue,
	}
}

func (p *DefaultValuePipe) Transform(value any, ctx *core.Context) (any, error) {
	// Check if value is empty
	if value == nil {
		return p.defaultValue, nil
	}

	// Check for empty string
	if str, ok := value.(string); ok && str == "" {
		return p.defaultValue, nil
	}

	return value, nil
}
