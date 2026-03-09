// gonest/common/application.go
package common

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gonest-dev/gonest/core/di"
)

// NestFactory creates NestJS-style applications
type NestFactory struct{}

// Create creates a new NestApplication with the root module
func (f NestFactory) Create(rootModule Module, opts ...ApplicationOption) *NestApplication {
	app := &NestApplication{
		metadata:        NewMetadataStorage(),
		compiler:        NewModuleCompiler(),
		lifecycle:       NewLifecycleManager(),
		container:       di.NewContainer(),
		injector:        nil, // Will be created after container setup
		platformAdapter: nil, // Will be set via option or default
		shutdownTimeout: 10 * time.Second,
	}

	// Create injector
	app.injector = di.NewInjector(app.container)

	// Apply options
	for _, opt := range opts {
		opt(app)
	}

	// Check for dev mode via environment variable
	if os.Getenv("GONEST_DEV_MODE") == "true" {
		app.EnableDevMode()
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
	container       *di.Container
	injector        *di.Injector
	platformAdapter PlatformAdapter
	shutdownTimeout time.Duration
	rootModule      *ModuleRef
	server          *http.Server
	isDevMode       bool
}

// bootstrapModule compiles and initializes the root module
func (app *NestApplication) bootstrapModule(module Module) error {
	ctx := context.Background()

	// Compile module tree (resolves dependencies)
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

	// Bootstrap all modules
	if err := app.bootstrapModulesRecursive(ctx, moduleRef); err != nil {
		return fmt.Errorf("failed to bootstrap modules: %w", err)
	}

	// Register all modules with lifecycle manager
	app.registerModulesRecursive(moduleRef)

	// Initialize all modules
	if err := app.lifecycle.CallOnModuleInit(ctx); err != nil {
		return fmt.Errorf("failed to initialize modules: %w", err)
	}

	// Call OnApplicationBootstrap
	if err := app.lifecycle.CallOnApplicationBootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap application: %w", err)
	}

	return nil
}

// bootstrapModulesRecursive bootstraps modules and their dependencies
func (app *NestApplication) bootstrapModulesRecursive(ctx context.Context, moduleRef *ModuleRef) error {
	// First, bootstrap imported modules (dependencies first)
	for _, importedModule := range moduleRef.imports {
		if err := app.bootstrapModulesRecursive(ctx, importedModule); err != nil {
			return err
		}
	}

	// Then bootstrap this module
	return app.bootstrapSingleModule(ctx, moduleRef)
}

// bootstrapSingleModule bootstraps a single module
func (app *NestApplication) bootstrapSingleModule(ctx context.Context, moduleRef *ModuleRef) error {
	// 1. Register and instantiate providers
	for _, providerDef := range moduleRef.metadata.providers {
		if err := app.registerProvider(ctx, moduleRef, providerDef); err != nil {
			return fmt.Errorf("failed to register provider in module %s: %w", moduleRef.name, err)
		}
	}

	// 2. Instantiate and register controllers
	for _, controllerType := range moduleRef.metadata.controllers {
		if err := app.registerController(ctx, moduleRef, controllerType); err != nil {
			return fmt.Errorf("failed to register controller in module %s: %w", moduleRef.name, err)
		}
	}

	// 3. Export providers
	for _, exportType := range moduleRef.metadata.exports {
		if err := app.exportProvider(moduleRef, exportType); err != nil {
			return fmt.Errorf("failed to export provider in module %s: %w", moduleRef.name, err)
		}
	}

	return nil
}

// registerProvider registers a provider in the DI container
func (app *NestApplication) registerProvider(ctx context.Context, moduleRef *ModuleRef, providerDef Provider) error {
	// Provider can be:
	// 1. A struct instance (use as class provider)
	// 2. A factory function
	// 3. A value

	switch p := providerDef.(type) {
	case ProviderClass:
		// Class provider: { provide: UserService, useClass: UserService }
		if err := app.container.RegisterType(p.UseClass, di.Singleton()); err != nil {
			return err
		}

		// Instantiate and store in module
		instance, err := app.container.Resolve(ctx, p.Provide)
		if err != nil {
			return err
		}
		moduleRef.RegisterProvider(p.Provide, instance)

	case ProviderFactory:
		// Factory provider: { provide: Token, useFactory: factoryFunc }
		if err := app.container.RegisterFactory(p.UseFactory, di.Singleton()); err != nil {
			return err
		}

		instance, err := app.container.Resolve(ctx, p.Provide)
		if err != nil {
			return err
		}
		moduleRef.RegisterProvider(p.Provide, instance)

	case ProviderValue:
		// Value provider: { provide: Token, useValue: value }
		if err := app.container.RegisterValue(p.UseValue, ""); err != nil {
			return err
		}
		moduleRef.RegisterProvider(p.Provide, p.UseValue)

	default:
		// Simple provider (just a struct type)
		if err := app.container.RegisterType(providerDef, di.Singleton()); err != nil {
			return err
		}

		t := getProviderType(providerDef)
		instance, err := app.container.Resolve(ctx, t)
		if err != nil {
			return err
		}
		moduleRef.RegisterProvider(t, instance)
	}

	return nil
}

// registerController instantiates and registers a controller
func (app *NestApplication) registerController(ctx context.Context, moduleRef *ModuleRef, controllerType any) error {
	// Controllers are instantiated with DI
	var instance any
	var err error

	// Try to resolve from container (if already registered)
	t := getProviderType(controllerType)
	instance, err = app.container.Resolve(ctx, t)
	if err != nil {
		// Not registered, create new instance and inject dependencies
		instance, err = app.injector.AutoWire(ctx, controllerType)
		if err != nil {
			return fmt.Errorf("failed to autowire controller: %w", err)
		}
	}

	// Register controller in module
	if err := moduleRef.RegisterController(instance); err != nil {
		return err
	}

	return nil
}

// exportProvider marks a provider as exported
func (app *NestApplication) exportProvider(moduleRef *ModuleRef, exportType any) error {
	t := getProviderType(exportType)

	// Get provider instance from module
	instance, ok := moduleRef.GetProvider(t)
	if !ok {
		return fmt.Errorf("cannot export provider that is not registered: %s", t)
	}

	// Mark as exported
	moduleRef.ExportProvider(t, instance)
	return nil
}

// registerModulesRecursive registers modules and their imports with lifecycle manager
func (app *NestApplication) registerModulesRecursive(moduleRef *ModuleRef) {
	app.lifecycle.RegisterModule(moduleRef)

	for _, importedModule := range moduleRef.imports {
		app.registerModulesRecursive(importedModule)
	}
}

// UsePlatform sets the platform adapter
func (app *NestApplication) UsePlatform(adapter PlatformAdapter) {
	app.platformAdapter = adapter
}

// Listen starts the HTTP server using the platform adapter
func (app *NestApplication) Listen(addr string) error {
	if app.platformAdapter == nil {
		return fmt.Errorf("no platform adapter set. Use app.UsePlatform() or provide a platform in options")
	}

	// Register all routes from modules with the platform
	modules := app.compiler.GetAll()
	for _, moduleRef := range modules {
		for _, ctrlRef := range moduleRef.GetControllers() {
			for _, route := range ctrlRef.routes {
				_ = app.platformAdapter.RegisterRoute(route)
			}
		}
	}

	// Start platform server
	app.server = &http.Server{
		Addr:    addr,
		Handler: app.platformAdapter.Handler(),
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Application is running on http://%s (platform: %s)", addr, app.platformAdapter.Name())
		if app.isDevMode {
			log.Printf("🛠️  Development mode enabled (fast shutdown active)")
		}
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	if app.isDevMode {
		log.Println("🔄 Reload signal received...")
	} else {
		log.Println("Shutting down server...")
	}

	return app.Close()
}

// EnableDevMode enables development mode with fast shutdown and specialized logging
func (app *NestApplication) EnableDevMode() {
	app.isDevMode = true
	app.shutdownTimeout = 500 * time.Millisecond
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
	if app.server != nil {
		if err := app.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
	}

	log.Println("✅ Server gracefully stopped")
	return nil
}

// GetContainer returns the DI container
func (app *NestApplication) GetContainer() *di.Container {
	return app.container
}

// GetMetadata returns the metadata storage
func (app *NestApplication) GetMetadata() *MetadataStorage {
	return app.metadata
}

// Application option functions

// WithPlatform sets the platform adapter
func WithPlatform(adapter PlatformAdapter) ApplicationOption {
	return func(app *NestApplication) {
		app.platformAdapter = adapter
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
		if app.server == nil {
			app.server = &http.Server{}
		}
		app.server.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the HTTP server write timeout
func WithWriteTimeout(timeout time.Duration) ApplicationOption {
	return func(app *NestApplication) {
		if app.server == nil {
			app.server = &http.Server{}
		}
		app.server.WriteTimeout = timeout
	}
}


