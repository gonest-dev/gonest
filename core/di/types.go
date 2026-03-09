// gonest/di/types.go
package di

import (
	"context"
	"reflect"
)

// Scope defines the lifetime of a provider
type Scope string

const (
	// ScopeSINGLETON - single instance shared across the entire application
	ScopeSINGLETON Scope = "singleton"

	// ScopeTRANSIENT - new instance created every time it's requested
	ScopeTRANSIENT Scope = "transient"

	// ScopeREQUEST - single instance per HTTP request
	ScopeREQUEST Scope = "request"
)

// Provider represents a dependency that can be injected
type Provider interface {
	// Provide returns the instance to be injected
	Provide(ctx context.Context, container *Container) (any, error)

	// Scope returns the provider's scope
	Scope() Scope

	// Type returns the type this provider produces
	Type() reflect.Type
}

// Token uniquely identifies a provider
type Token struct {
	Type reflect.Type
	Name string // Optional name for multiple providers of same type
}

// ProviderOptions configures a provider
type ProviderOptions struct {
	Scope Scope
	Name  string
}

// ProviderOption is a functional option for configuring providers
type ProviderOption func(*ProviderOptions)

// WithScope sets the provider scope
func WithScope(scope Scope) ProviderOption {
	return func(o *ProviderOptions) {
		o.Scope = scope
	}
}

// WithName sets a named provider (for multiple providers of same type)
func WithName(name string) ProviderOption {
	return func(o *ProviderOptions) {
		o.Name = name
	}
}

// Singleton is a convenience option for singleton scope
func Singleton() ProviderOption {
	return WithScope(ScopeSINGLETON)
}

// Transient is a convenience option for transient scope
func Transient() ProviderOption {
	return WithScope(ScopeTRANSIENT)
}

// Request is a convenience option for request scope
func Request() ProviderOption {
	return WithScope(ScopeREQUEST)
}


