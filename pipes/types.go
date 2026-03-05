package pipes

import (
	"github.com/leandroluk/gonest/core"
)

// PipeTransform is the interface that all pipes must implement
type PipeTransform[T any, R any] interface {
	Transform(value T, ctx *core.Context) (R, error)
}

// Pipe is a function type for transformation
type Pipe func(value any, ctx *core.Context) (any, error)

// ValidationPipeOptions configures validation behavior
type ValidationPipeOptions struct {
	// Transform - enable transformation
	Transform bool

	// Whitelist - strip properties that don't have decorators
	Whitelist bool

	// ForbidNonWhitelisted - throw error if non-whitelisted properties exist
	ForbidNonWhitelisted bool

	// SkipMissingProperties - skip validation of properties that don't exist
	SkipMissingProperties bool

	// DisableErrorMessages - disable detailed error messages
	DisableErrorMessages bool
}

// DefaultValidationPipeOptions returns default options
func DefaultValidationPipeOptions() *ValidationPipeOptions {
	return &ValidationPipeOptions{
		Transform:             true,
		Whitelist:             false,
		ForbidNonWhitelisted:  false,
		SkipMissingProperties: false,
		DisableErrorMessages:  false,
	}
}
