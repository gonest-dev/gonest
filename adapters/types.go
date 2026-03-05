package adapters

import (
	"github.com/gonest-dev/gonest/core"
)

// PlatformAdapter defines the interface for platform adapters
type PlatformAdapter interface {
	// Name returns the adapter name
	Name() string

	// WrapHandler wraps a GoNest handler for the platform
	WrapHandler(handler core.HandlerFunc) any

	// WrapMiddleware wraps a GoNest middleware for the platform
	WrapMiddleware(middleware core.MiddlewareFunc) any

	// ExtractContext extracts GoNest context from platform context
	ExtractContext(platformCtx any) *core.Context

	// CreateContext creates a GoNest context from platform context
	CreateContext(platformCtx any) *core.Context
}

// AdapterConfig configures an adapter
type AdapterConfig struct {
	ErrorHandler func(error, any) error
	Logger       Logger
}

// Logger interface for adapters
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// DefaultLogger provides basic logging
type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, args ...any)  {}
func (l *DefaultLogger) Error(msg string, args ...any) {}
func (l *DefaultLogger) Debug(msg string, args ...any) {}
