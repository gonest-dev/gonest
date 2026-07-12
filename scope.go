package gonest

import "github.com/gonest-dev/gonest/internal/scope"

// Scope defines the lifetime of a provider instance within the DI container.
type Scope = scope.Scope

const (
	// ScopeSingleton means a single shared instance is created and reused
	// for the lifetime of the application.
	ScopeSingleton = scope.Singleton
	// ScopeTransient means a new instance is created every time the
	// provider is resolved.
	ScopeTransient = scope.Transient
	// ScopeRequest means a single instance is created per incoming request
	// and shared across resolutions within that request.
	ScopeRequest = scope.Request
)
