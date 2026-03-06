// gonest/di/container.go
package di

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Container is the dependency injection container
type Container struct {
	providers  map[Token]Provider
	singletons map[Token]any
	mu         sync.RWMutex

	// Parent container for hierarchical DI
	parent *Container

	// Request-scoped instances (per HTTP request)
	requestInstances map[Token]any
	requestMu        sync.RWMutex
}

// NewContainer creates a new DI container
func NewContainer() *Container {
	return &Container{
		providers:        make(map[Token]Provider),
		singletons:       make(map[Token]any),
		requestInstances: make(map[Token]any),
	}
}

// NewChildContainer creates a child container (for scoping)
func NewChildContainer(parent *Container) *Container {
	return &Container{
		providers:        make(map[Token]Provider),
		singletons:       make(map[Token]any),
		requestInstances: make(map[Token]any),
		parent:           parent,
	}
}

// Register registers a provider in the container
func (c *Container) Register(provider Provider, name string) error {
	token := Token{
		Type: provider.Type(),
		Name: name,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[token]; exists {
		return fmt.Errorf("provider already registered: %s", token.Type)
	}

	c.providers[token] = provider
	return nil
}

// RegisterType registers a type with automatic constructor resolution
func (c *Container) RegisterType(instance any, opts ...ProviderOption) error {
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Create constructor function
	constructor := func() any {
		return reflect.New(t).Interface()
	}

	provider, err := NewClassProvider(constructor, opts...)
	if err != nil {
		return err
	}

	return c.Register(provider, "")
}

// RegisterValue registers a pre-existing value
func (c *Container) RegisterValue(value any, name string) error {
	provider := NewValueProvider(value)
	return c.Register(provider, name)
}

// RegisterFactory registers a factory function
func (c *Container) RegisterFactory(factory any, opts ...ProviderOption) error {
	provider, err := NewFactoryProvider(factory, opts...)
	if err != nil {
		return err
	}
	return c.Register(provider, "")
}

// RegisterAsync registers an async provider
func (c *Container) RegisterAsync(factory any, opts ...ProviderOption) error {
	provider, err := NewAsyncProvider(factory, opts...)
	if err != nil {
		return err
	}
	return c.Register(provider, "")
}

// Resolve resolves a dependency by type
func (c *Container) Resolve(ctx context.Context, t reflect.Type) (any, error) {
	return c.ResolveNamed(ctx, t, "")
}

// ResolveNamed resolves a named dependency
func (c *Container) ResolveNamed(ctx context.Context, t reflect.Type, name string) (any, error) {
	token := Token{Type: t, Name: name}

	// Try to find provider
	provider, err := c.findProvider(token)
	if err != nil {
		return nil, err
	}

	// Handle different scopes
	switch provider.Scope() {
	case ScopeSINGLETON:
		return c.resolveSingleton(ctx, token, provider)
	case ScopeREQUEST:
		return c.resolveRequest(ctx, token, provider)
	case ScopeTRANSIENT:
		return c.resolveTransient(ctx, provider)
	default:
		return nil, fmt.Errorf("unknown scope: %s", provider.Scope())
	}
}

// findProvider finds a provider in this container or parent
func (c *Container) findProvider(token Token) (Provider, error) {
	c.mu.RLock()
	provider, exists := c.providers[token]
	c.mu.RUnlock()

	if exists {
		return provider, nil
	}

	// Try parent container
	if c.parent != nil {
		return c.parent.findProvider(token)
	}

	return nil, fmt.Errorf("provider not found: %s", token.Type)
}

// resolveSingleton resolves a singleton instance
func (c *Container) resolveSingleton(ctx context.Context, token Token, provider Provider) (any, error) {
	// Check if already instantiated
	c.mu.RLock()
	instance, exists := c.singletons[token]
	c.mu.RUnlock()

	if exists {
		return instance, nil
	}

	// Create new instance
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if instance, exists := c.singletons[token]; exists {
		return instance, nil
	}

	instance, err := provider.Provide(ctx, c)
	if err != nil {
		return nil, err
	}

	c.singletons[token] = instance
	return instance, nil
}

// resolveRequest resolves a request-scoped instance
func (c *Container) resolveRequest(ctx context.Context, token Token, provider Provider) (any, error) {
	c.requestMu.RLock()
	instance, exists := c.requestInstances[token]
	c.requestMu.RUnlock()

	if exists {
		return instance, nil
	}

	c.requestMu.Lock()
	defer c.requestMu.Unlock()

	// Double-check
	if instance, exists := c.requestInstances[token]; exists {
		return instance, nil
	}

	instance, err := provider.Provide(ctx, c)
	if err != nil {
		return nil, err
	}

	c.requestInstances[token] = instance
	return instance, nil
}

// resolveTransient always creates a new instance
func (c *Container) resolveTransient(ctx context.Context, provider Provider) (any, error) {
	return provider.Provide(ctx, c)
}

// ClearRequestScope clears all request-scoped instances
func (c *Container) ClearRequestScope() {
	c.requestMu.Lock()
	defer c.requestMu.Unlock()
	c.requestInstances = make(map[Token]any)
}

// Has checks if a provider is registered
func (c *Container) Has(t reflect.Type, name string) bool {
	token := Token{Type: t, Name: name}

	c.mu.RLock()
	_, exists := c.providers[token]
	c.mu.RUnlock()

	if exists {
		return true
	}

	if c.parent != nil {
		return c.parent.Has(t, name)
	}

	return false
}

// GetProvider returns the provider for a type
func (c *Container) GetProvider(t reflect.Type, name string) (Provider, bool) {
	token := Token{Type: t, Name: name}

	c.mu.RLock()
	provider, exists := c.providers[token]
	c.mu.RUnlock()

	if exists {
		return provider, true
	}

	if c.parent != nil {
		return c.parent.GetProvider(t, name)
	}

	return nil, false
}

// Clear clears all singletons and request instances
func (c *Container) Clear() {
	c.mu.Lock()
	c.singletons = make(map[Token]any)
	c.mu.Unlock()

	c.ClearRequestScope()
}
