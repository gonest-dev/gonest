package core

import "context"

// Module defines the interface that all modules must implement
type Module interface {
	Configure(builder *ModuleBuilder)
}

// Provider represents a dependency that can be injected
type Provider any

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
}

// HandlerFunc is the signature for route handlers
type HandlerFunc func(*Context) error

// MiddlewareFunc is the signature for middleware functions
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// ApplicationOption configures the NestApplication
type ApplicationOption func(*NestApplication)
