// gonest/platform/types.go
package platform

import (
	"net/http"

	"github.com/gonest-dev/gonest/core/common"
)

// Adapter defines the interface for platform-specific (e.g. Fiber, Gin) implementations
type Adapter interface {
	// Name returns the platform name
	Name() string

	// RegisterRoute registers a route with the platform
	RegisterRoute(route common.RouteDefinition) error

	// Handler returns the http.Handler for the platform
	Handler() http.Handler

	// Use registers global middleware
	Use(middleware common.MiddlewareFunc)
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

func (l *DefaultLogger) Info(_ string, _ ...any)  {}
func (l *DefaultLogger) Error(_ string, _ ...any) {}
func (l *DefaultLogger) Debug(_ string, _ ...any) {}
