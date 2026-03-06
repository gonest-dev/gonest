package core

import (
	"fmt"
	"reflect"
	"sync"
)

// ModuleBuilder helps build module configurations
type ModuleBuilder struct {
	imports     []Module
	controllers []any
	providers   []Provider
	exports     []any
}

// Imports adds modules to be imported
func (b *ModuleBuilder) Imports(modules ...Module) *ModuleBuilder {
	b.imports = append(b.imports, modules...)
	return b
}

// Controllers adds controllers to the module
func (b *ModuleBuilder) Controllers(controllers ...any) *ModuleBuilder {
	b.controllers = append(b.controllers, controllers...)
	return b
}

// Providers adds providers to the module
func (b *ModuleBuilder) Providers(providers ...Provider) *ModuleBuilder {
	b.providers = append(b.providers, providers...)
	return b
}

// Exports marks providers as exportable to other modules
func (b *ModuleBuilder) Exports(exports ...any) *ModuleBuilder {
	b.exports = append(b.exports, exports...)
	return b
}

// ModuleRef represents a compiled module instance
type ModuleRef struct {
	name        string
	module      Module
	imports     []*ModuleRef
	controllers []ControllerRef
	providers   map[reflect.Type]any
	exports     map[reflect.Type]any
	metadata    *ModuleMetadata
	mu          sync.RWMutex
}

// ControllerRef represents a controller instance
type ControllerRef struct {
	instance   any
	controller Controller
	routes     []RouteDefinition
}

// ModuleMetadata stores module metadata
type ModuleMetadata struct {
	imports     []Module
	controllers []any
	providers   []Provider
	exports     []any
}

// ModuleCompiler compiles modules into executable units
type ModuleCompiler struct {
	modules map[string]*ModuleRef
	mu      sync.RWMutex
}

// NewModuleCompiler creates a new module compiler
func NewModuleCompiler() *ModuleCompiler {
	return &ModuleCompiler{
		modules: make(map[string]*ModuleRef),
	}
}

// Compile compiles a module and its dependencies
func (mc *ModuleCompiler) Compile(module Module) (*ModuleRef, error) {
	moduleName := getModuleName(module)

	// Check if already compiled
	mc.mu.RLock()
	if ref, exists := mc.modules[moduleName]; exists {
		mc.mu.RUnlock()
		return ref, nil
	}
	mc.mu.RUnlock()

	// Create module reference
	ref := &ModuleRef{
		name:        moduleName,
		module:      module,
		imports:     make([]*ModuleRef, 0),
		controllers: make([]ControllerRef, 0),
		providers:   make(map[reflect.Type]any),
		exports:     make(map[reflect.Type]any),
		metadata:    &ModuleMetadata{},
	}

	// Configure module
	builder := &ModuleBuilder{
		imports:     make([]Module, 0),
		controllers: make([]any, 0),
		providers:   make([]Provider, 0),
		exports:     make([]any, 0),
	}

	module.Configure(builder)

	// Store metadata
	ref.metadata.imports = builder.imports
	ref.metadata.controllers = builder.controllers
	ref.metadata.providers = builder.providers
	ref.metadata.exports = builder.exports

	// Compile imports first (dependency resolution)
	for _, importedModule := range builder.imports {
		importedRef, err := mc.Compile(importedModule)
		if err != nil {
			return nil, fmt.Errorf("failed to compile imported module: %w", err)
		}
		ref.imports = append(ref.imports, importedRef)
	}

	// Register in compiler
	mc.mu.Lock()
	mc.modules[moduleName] = ref
	mc.mu.Unlock()

	return ref, nil
}

// Get retrieves a compiled module by type
func (mc *ModuleCompiler) Get(module Module) (*ModuleRef, bool) {
	moduleName := getModuleName(module)
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	ref, exists := mc.modules[moduleName]
	return ref, exists
}

// GetAll returns all compiled modules
func (mc *ModuleCompiler) GetAll() []*ModuleRef {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	refs := make([]*ModuleRef, 0, len(mc.modules))
	for _, ref := range mc.modules {
		refs = append(refs, ref)
	}
	return refs
}

// RegisterProvider registers a provider in the module
func (mr *ModuleRef) RegisterProvider(providerType reflect.Type, instance any) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.providers[providerType] = instance
}

// GetProvider retrieves a provider from the module
func (mr *ModuleRef) GetProvider(providerType reflect.Type) (any, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	// Check own providers
	if provider, exists := mr.providers[providerType]; exists {
		return provider, true
	}

	// Check imported modules' exported providers
	for _, importedModule := range mr.imports {
		if provider, exists := importedModule.exports[providerType]; exists {
			return provider, true
		}
	}

	return nil, false
}

// RegisterController registers a controller in the module
func (mr *ModuleRef) RegisterController(instance any) error {
	controller, ok := instance.(Controller)
	if !ok {
		return fmt.Errorf("instance does not implement Controller interface")
	}

	routes := controller.Routes()

	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.controllers = append(mr.controllers, ControllerRef{
		instance:   instance,
		controller: controller,
		routes:     routes,
	})

	return nil
}

// GetControllers returns all controllers in the module
func (mr *ModuleRef) GetControllers() []ControllerRef {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	controllers := make([]ControllerRef, len(mr.controllers))
	copy(controllers, mr.controllers)
	return controllers
}

// ExportProvider marks a provider as exported
func (mr *ModuleRef) ExportProvider(providerType reflect.Type, instance any) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.exports[providerType] = instance
}

// Name returns the module name
func (mr *ModuleRef) Name() string {
	return mr.name
}

// getModuleName extracts the module name from its type
func getModuleName(module Module) string {
	t := reflect.TypeOf(module)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath() + "." + t.Name()
}

// circularDependencyDetector detects circular dependencies in module imports
type circularDependencyDetector struct {
	visiting map[string]bool
	visited  map[string]bool
}

// newCircularDependencyDetector creates a new detector
func newCircularDependencyDetector() *circularDependencyDetector {
	return &circularDependencyDetector{
		visiting: make(map[string]bool),
		visited:  make(map[string]bool),
	}
}

// detect checks for circular dependencies starting from a module
func (cdd *circularDependencyDetector) detect(module Module, metadata *ModuleMetadata) error {
	moduleName := getModuleName(module)

	if cdd.visiting[moduleName] {
		return fmt.Errorf("circular dependency detected: %s", moduleName)
	}

	if cdd.visited[moduleName] {
		return nil
	}

	cdd.visiting[moduleName] = true

	for _, importedModule := range metadata.imports {
		builder := &ModuleBuilder{}
		importedModule.Configure(builder)

		importedMetadata := &ModuleMetadata{
			imports:     builder.imports,
			controllers: builder.controllers,
			providers:   builder.providers,
			exports:     builder.exports,
		}

		if err := cdd.detect(importedModule, importedMetadata); err != nil {
			return err
		}
	}

	cdd.visiting[moduleName] = false
	cdd.visited[moduleName] = true

	return nil
}
