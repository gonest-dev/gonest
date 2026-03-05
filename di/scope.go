package di

import (
	"context"
	"fmt"
	"sync"
)

// ScopeManager manages different dependency scopes
type ScopeManager struct {
	globalContainer   *Container
	requestContainers sync.Map // map[requestID]*Container
	mu                sync.RWMutex
}

// NewScopeManager creates a new scope manager
func NewScopeManager(globalContainer *Container) *ScopeManager {
	return &ScopeManager{
		globalContainer: globalContainer,
	}
}

// CreateRequestScope creates a new request-scoped container
func (sm *ScopeManager) CreateRequestScope(ctx context.Context) (*Container, context.Context) {
	requestContainer := NewChildContainer(sm.globalContainer)

	// Generate request ID and store in context
	requestID := generateRequestID()
	sm.requestContainers.Store(requestID, requestContainer)

	// Store request ID in context
	ctx = context.WithValue(ctx, requestIDKey{}, requestID)

	return requestContainer, ctx
}

// GetRequestScope retrieves the request-scoped container from context
func (sm *ScopeManager) GetRequestScope(ctx context.Context) (*Container, bool) {
	requestID, ok := ctx.Value(requestIDKey{}).(string)
	if !ok {
		return nil, false
	}

	container, ok := sm.requestContainers.Load(requestID)
	if !ok {
		return nil, false
	}

	return container.(*Container), true
}

// CleanupRequestScope removes the request-scoped container
func (sm *ScopeManager) CleanupRequestScope(ctx context.Context) {
	requestID, ok := ctx.Value(requestIDKey{}).(string)
	if !ok {
		return
	}

	if container, ok := sm.requestContainers.Load(requestID); ok {
		container.(*Container).ClearRequestScope()
		sm.requestContainers.Delete(requestID)
	}
}

// GetContainer returns the appropriate container for the context
func (sm *ScopeManager) GetContainer(ctx context.Context) *Container {
	// Try to get request-scoped container
	if requestContainer, ok := sm.GetRequestScope(ctx); ok {
		return requestContainer
	}

	// Fallback to global container
	return sm.globalContainer
}

type requestIDKey struct{}

var requestIDCounter uint64
var requestIDMu sync.Mutex

func generateRequestID() string {
	requestIDMu.Lock()
	defer requestIDMu.Unlock()
	requestIDCounter++
	return fmt.Sprintf("req-%d", requestIDCounter)
}

// ScopeContext stores scope information in context
type ScopeContext struct {
	scope     Scope
	container *Container
}

type scopeContextKey struct{}

// WithScopeContext attaches scope information to context
func WithScopeContext(ctx context.Context, scope Scope, container *Container) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, &ScopeContext{
		scope:     scope,
		container: container,
	})
}

// GetScope retrieves scope information from context
func GetScope(ctx context.Context) (*ScopeContext, bool) {
	sc, ok := ctx.Value(scopeContextKey{}).(*ScopeContext)
	return sc, ok
}

// WithSingleton is a helper to mark a context as singleton scope
func WithSingleton(ctx context.Context, container *Container) context.Context {
	return WithScopeContext(ctx, ScopeSINGLETON, container)
}

// WithTransient is a helper to mark a context as transient scope
func WithTransient(ctx context.Context, container *Container) context.Context {
	return WithScopeContext(ctx, ScopeTRANSIENT, container)
}

// WithRequest is a helper to mark a context as request scope
func WithRequest(ctx context.Context, container *Container) context.Context {
	return WithScopeContext(ctx, ScopeREQUEST, container)
}
