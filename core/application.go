// gonest/core/application.go
package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// NestFactory creates NestJS-style applications
type NestFactory struct{}

// Create creates a new NestApplication with the root module
func (f NestFactory) Create(rootModule Module, opts ...ApplicationOption) *NestApplication {
	app := &NestApplication{
		metadata:        NewMetadataStorage(),
		compiler:        NewModuleCompiler(),
		lifecycle:       NewLifecycleManager(),
		router:          NewRouter(),
		server:          &http.Server{},
		shutdownTimeout: 10 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(app)
	}

	// Bootstrap root module
	if err := app.bootstrapModule(rootModule); err != nil {
		log.Fatalf("Failed to bootstrap application: %v", err)
	}

	return app
}

// NestApplication is the main application instance
type NestApplication struct {
	metadata        *MetadataStorage
	compiler        *ModuleCompiler
	lifecycle       *LifecycleManager
	router          *Router
	server          *http.Server
	shutdownTimeout time.Duration
	rootModule      *ModuleRef
}

// bootstrapModule compiles and initializes the root module
func (app *NestApplication) bootstrapModule(module Module) error {
	// Compile module tree
	moduleRef, err := app.compiler.Compile(module)
	if err != nil {
		return fmt.Errorf("failed to compile module: %w", err)
	}

	app.rootModule = moduleRef

	// Detect circular dependencies
	detector := newCircularDependencyDetector()
	if err := detector.detect(module, moduleRef.metadata); err != nil {
		return err
	}

	// Register all modules with lifecycle manager
	app.registerModulesRecursive(moduleRef)

	// Initialize all modules
	ctx := context.Background()
	if err := app.lifecycle.CallOnModuleInit(ctx); err != nil {
		return fmt.Errorf("failed to initialize modules: %w", err)
	}

	// Bootstrap controllers and routes
	if err := app.bootstrapControllers(); err != nil {
		return fmt.Errorf("failed to bootstrap controllers: %w", err)
	}

	// Call OnApplicationBootstrap
	if err := app.lifecycle.CallOnApplicationBootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap application: %w", err)
	}

	return nil
}

// registerModulesRecursive registers modules and their imports with lifecycle manager
func (app *NestApplication) registerModulesRecursive(moduleRef *ModuleRef) {
	app.lifecycle.RegisterModule(moduleRef)

	for _, importedModule := range moduleRef.imports {
		app.registerModulesRecursive(importedModule)
	}
}

// bootstrapControllers initializes and registers all controllers
func (app *NestApplication) bootstrapControllers() error {
	modules := app.compiler.GetAll()

	for _, moduleRef := range modules {
		for _, ctrlRef := range moduleRef.GetControllers() {
			// Register routes
			for _, route := range ctrlRef.routes {
				app.router.Register(route)
			}
		}
	}

	return nil
}

// Listen starts the HTTP server
func (app *NestApplication) Listen(addr string) error {
	app.server.Addr = addr
	app.server.Handler = app.router

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Application is running on http://%s", addr)
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down server...")

	return app.Close()
}

// Close gracefully shuts down the application
func (app *NestApplication) Close() error {
	// Create shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
	defer cancel()

	// Call OnApplicationShutdown hooks
	if err := app.lifecycle.CallOnApplicationShutdown(ctx); err != nil {
		log.Printf("Warning: OnApplicationShutdown error: %v", err)
	}

	// Call OnModuleDestroy hooks
	if err := app.lifecycle.CallOnModuleDestroy(ctx); err != nil {
		log.Printf("Warning: OnModuleDestroy error: %v", err)
	}

	// Shutdown HTTP server
	if err := app.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	log.Println("✅ Server gracefully stopped")
	return nil
}

// GetMetadata returns the metadata storage
func (app *NestApplication) GetMetadata() *MetadataStorage {
	return app.metadata
}

// GetRouter returns the application router
func (app *NestApplication) GetRouter() *Router {
	return app.router
}

// Application option functions

// WithPort sets the server port
func WithPort(_ int) ApplicationOption {
	return func(_ *NestApplication) {
		// Port will be used in Listen() method
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout
func WithShutdownTimeout(timeout time.Duration) ApplicationOption {
	return func(app *NestApplication) {
		app.shutdownTimeout = timeout
	}
}

// WithReadTimeout sets the HTTP server read timeout
func WithReadTimeout(timeout time.Duration) ApplicationOption {
	return func(app *NestApplication) {
		app.server.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the HTTP server write timeout
func WithWriteTimeout(timeout time.Duration) ApplicationOption {
	return func(app *NestApplication) {
		app.server.WriteTimeout = timeout
	}
}

// WithIdleTimeout sets the HTTP server idle timeout
func WithIdleTimeout(timeout time.Duration) ApplicationOption {
	return func(app *NestApplication) {
		app.server.IdleTimeout = timeout
	}
}
