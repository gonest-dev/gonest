// gonest/common/types.go
package common

import (
	"context"
	"net/http"
	"reflect"
)

// Module defines the interface that all modules must implement
type Module interface {
	Configure(builder *ModuleBuilder)
}

// Provider represents a dependency that can be injected
// Can be a simple type, or one of the provider descriptors below
type Provider any

// ProviderClass represents a class-based provider
// Example: { provide: UserService, useClass: UserServiceImpl }
type ProviderClass struct {
	Provide  reflect.Type
	UseClass any
}

// ProviderFactory represents a factory-based provider
// Example: { provide: Connection, useFactory: createConnection }
type ProviderFactory struct {
	Provide    reflect.Type
	UseFactory any // func(...deps) T
}

// ProviderValue represents a value-based provider
// Example: { provide: CONFIG_TOKEN, useValue: config }
type ProviderValue struct {
	Provide  reflect.Type
	UseValue any
}

// Controller represents a request handler with routes
type Controller interface {
	Routes() []RouteDefinition
}

// OnModuleInit lifecycle hook called when module is initialized
type OnModuleInit interface {
	OnModuleInit(ctx context.Context) error
}

// OnModuleDestroy lifecycle hook called when module is destroyed
type OnModuleDestroy interface {
	OnModuleDestroy(ctx context.Context) error
}

// OnApplicationBootstrap lifecycle hook called when application starts
type OnApplicationBootstrap interface {
	OnApplicationBootstrap(ctx context.Context) error
}

// OnApplicationShutdown lifecycle hook called before application stops
type OnApplicationShutdown interface {
	OnApplicationShutdown(ctx context.Context) error
}

// RouteDefinition defines a single route with its configuration
type RouteDefinition struct {
	Method      string
	Path        string
	Handler     HandlerFunc
	Middlewares []MiddlewareFunc
	Metadata    map[string]any // For swagger, guards, etc
}

// HandlerFunc is the signature for route handlers
type HandlerFunc func(*Context) error

// MiddlewareFunc is the signature for middleware functions
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// ApplicationOption configures the NestApplication
type ApplicationOption func(*NestApplication)

// Adapter abstracts different HTTP frameworks
type Adapter interface {
	// Name returns the platform name (e.g., "gin", "fiber", "echo")
	Name() string

	// RegisterRoute registers a route with the platform
	RegisterRoute(route RouteDefinition) error

	// Handler returns the http.Handler for the platform
	Handler() http.Handler

	// Use registers global middleware
	Use(middleware MiddlewareFunc)
}

// Helper function to get reflect.Type from provider
func getProviderType(provider any) reflect.Type {
	return reflect.TypeOf(provider)
}
