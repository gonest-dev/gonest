package core

import (
	"context"
	"fmt"
	"sync"
)

// LifecycleManager manages application and module lifecycle hooks
type LifecycleManager struct {
	modules []*ModuleRef
	mu      sync.RWMutex
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		modules: make([]*ModuleRef, 0),
	}
}

// RegisterModule registers a module for lifecycle management
func (lm *LifecycleManager) RegisterModule(module *ModuleRef) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.modules = append(lm.modules, module)
}

// CallOnModuleInit calls OnModuleInit on all modules and their providers
func (lm *LifecycleManager) CallOnModuleInit(ctx context.Context) error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, moduleRef := range lm.modules {
		// Call on module itself
		if lifecycle, ok := moduleRef.module.(OnModuleInit); ok {
			if err := lifecycle.OnModuleInit(ctx); err != nil {
				return fmt.Errorf("module %s OnModuleInit failed: %w", moduleRef.name, err)
			}
		}

		// Call on providers
		moduleRef.mu.RLock()
		for _, provider := range moduleRef.providers {
			if lifecycle, ok := provider.(OnModuleInit); ok {
				if err := lifecycle.OnModuleInit(ctx); err != nil {
					moduleRef.mu.RUnlock()
					return fmt.Errorf("provider OnModuleInit failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
		moduleRef.mu.RUnlock()

		// Call on controllers
		for _, ctrlRef := range moduleRef.GetControllers() {
			if lifecycle, ok := ctrlRef.instance.(OnModuleInit); ok {
				if err := lifecycle.OnModuleInit(ctx); err != nil {
					return fmt.Errorf("controller OnModuleInit failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
	}

	return nil
}

// CallOnModuleDestroy calls OnModuleDestroy on all modules and their providers
func (lm *LifecycleManager) CallOnModuleDestroy(ctx context.Context) error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Call in reverse order (LIFO)
	for i := len(lm.modules) - 1; i >= 0; i-- {
		moduleRef := lm.modules[i]

		// Call on controllers
		controllers := moduleRef.GetControllers()
		for j := len(controllers) - 1; j >= 0; j-- {
			if lifecycle, ok := controllers[j].instance.(OnModuleDestroy); ok {
				if err := lifecycle.OnModuleDestroy(ctx); err != nil {
					return fmt.Errorf("controller OnModuleDestroy failed in module %s: %w", moduleRef.name, err)
				}
			}
		}

		// Call on providers
		moduleRef.mu.RLock()
		for _, provider := range moduleRef.providers {
			if lifecycle, ok := provider.(OnModuleDestroy); ok {
				if err := lifecycle.OnModuleDestroy(ctx); err != nil {
					moduleRef.mu.RUnlock()
					return fmt.Errorf("provider OnModuleDestroy failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
		moduleRef.mu.RUnlock()

		// Call on module itself
		if lifecycle, ok := moduleRef.module.(OnModuleDestroy); ok {
			if err := lifecycle.OnModuleDestroy(ctx); err != nil {
				return fmt.Errorf("module %s OnModuleDestroy failed: %w", moduleRef.name, err)
			}
		}
	}

	return nil
}

// CallOnApplicationBootstrap calls OnApplicationBootstrap on all components
func (lm *LifecycleManager) CallOnApplicationBootstrap(ctx context.Context) error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, moduleRef := range lm.modules {
		// Call on module
		if lifecycle, ok := moduleRef.module.(OnApplicationBootstrap); ok {
			if err := lifecycle.OnApplicationBootstrap(ctx); err != nil {
				return fmt.Errorf("module %s OnApplicationBootstrap failed: %w", moduleRef.name, err)
			}
		}

		// Call on providers
		moduleRef.mu.RLock()
		for _, provider := range moduleRef.providers {
			if lifecycle, ok := provider.(OnApplicationBootstrap); ok {
				if err := lifecycle.OnApplicationBootstrap(ctx); err != nil {
					moduleRef.mu.RUnlock()
					return fmt.Errorf("provider OnApplicationBootstrap failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
		moduleRef.mu.RUnlock()

		// Call on controllers
		for _, ctrlRef := range moduleRef.GetControllers() {
			if lifecycle, ok := ctrlRef.instance.(OnApplicationBootstrap); ok {
				if err := lifecycle.OnApplicationBootstrap(ctx); err != nil {
					return fmt.Errorf("controller OnApplicationBootstrap failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
	}

	return nil
}

// CallOnApplicationShutdown calls OnApplicationShutdown on all components
func (lm *LifecycleManager) CallOnApplicationShutdown(ctx context.Context) error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Call in reverse order (LIFO)
	for i := len(lm.modules) - 1; i >= 0; i-- {
		moduleRef := lm.modules[i]

		// Call on controllers
		controllers := moduleRef.GetControllers()
		for j := len(controllers) - 1; j >= 0; j-- {
			if lifecycle, ok := controllers[j].instance.(OnApplicationShutdown); ok {
				if err := lifecycle.OnApplicationShutdown(ctx); err != nil {
					return fmt.Errorf("controller OnApplicationShutdown failed in module %s: %w", moduleRef.name, err)
				}
			}
		}

		// Call on providers
		moduleRef.mu.RLock()
		for _, provider := range moduleRef.providers {
			if lifecycle, ok := provider.(OnApplicationShutdown); ok {
				if err := lifecycle.OnApplicationShutdown(ctx); err != nil {
					moduleRef.mu.RUnlock()
					return fmt.Errorf("provider OnApplicationShutdown failed in module %s: %w", moduleRef.name, err)
				}
			}
		}
		moduleRef.mu.RUnlock()

		// Call on module
		if lifecycle, ok := moduleRef.module.(OnApplicationShutdown); ok {
			if err := lifecycle.OnApplicationShutdown(ctx); err != nil {
				return fmt.Errorf("module %s OnApplicationShutdown failed: %w", moduleRef.name, err)
			}
		}
	}

	return nil
}

// LifecycleHookOrder defines the order of lifecycle hooks
type LifecycleHookOrder int

const (
	// OnModuleInitOrder - called when module dependencies are resolved
	OnModuleInitOrder LifecycleHookOrder = iota

	// OnApplicationBootstrapOrder - called when all modules are initialized
	OnApplicationBootstrapOrder

	// OnModuleDestroyOrder - called when module is being destroyed
	OnModuleDestroyOrder

	// OnApplicationShutdownOrder - called before application shutdown
	OnApplicationShutdownOrder
)

// String returns the string representation of lifecycle hook order
func (o LifecycleHookOrder) String() string {
	switch o {
	case OnModuleInitOrder:
		return "OnModuleInit"
	case OnApplicationBootstrapOrder:
		return "OnApplicationBootstrap"
	case OnModuleDestroyOrder:
		return "OnModuleDestroy"
	case OnApplicationShutdownOrder:
		return "OnApplicationShutdown"
	default:
		return "Unknown"
	}
}
