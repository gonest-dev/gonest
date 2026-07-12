// Package module implements the declarative Module API of the DI graph:
// Module.New(fn) defers fn until Stage 1 assembly runs, and Module.Imports/
// Providers/Controllers/Exports register structural references between
// modules, providers, and controllers.
package module

import "reflect"

// ProviderRef is a minimal marker interface satisfied by the real
// *provider.Provider type (owned by internal/provider). Module never needs
// to know the concrete provider type — it only tracks registered
// participants for Stage 1's structural bookkeeping, exposes the type it
// resolves (ResolvedType), and accepts ownership wiring (SetOwnerModule)
// during Stage 1 assembly.
//
// Exported (not just structurally implementable) because Go ties
// unexported interface methods to the declaring package: a method named
// isProvider() defined on *provider.Provider in a different package can
// never satisfy an unexported isProvider() declared here, even with an
// identical signature. Exporting the interface — not the marker method
// itself — is what makes cross-package satisfaction possible.
type ProviderRef interface {
	IsProvider()
	// ResolvedType returns the reflect.Type this provider resolves (its
	// Constructor's first return value's type), used by internal/resolver
	// to match a MustInject[T] target type against registered providers.
	ResolvedType() reflect.Type
	// SetOwnerModule associates this provider with the module that owns
	// it. Called by assemble during Stage 1 so OwnerModule() reflects
	// reality without any manual wiring by callers.
	SetOwnerModule(m *Module)
}

// ControllerRef is the Controller equivalent of ProviderRef, minus
// ResolvedType (a Controller does not itself resolve a type). Same
// cross-package rationale.
type ControllerRef interface {
	IsController()
	// SetOwnerModule associates this controller with the module that owns
	// it. Called by assemble during Stage 1.
	SetOwnerModule(m *Module)
}

// Owner is the contract implemented by Provider and Controller to report
// which Module they belong to. Used later by MustInject to determine
// resolution scope (own module -> imported exports).
type Owner interface {
	OwnerModule() *Module
}

// Module represents a declarative unit of the DI graph: its own providers
// and controllers, the modules it imports, and the subset of its own
// providers it exports to importers.
//
// New(fn) does not execute fn at call time. fn is only invoked during
// Stage 1 assembly (see assemble.go), since that is the earliest point at
// which the full import graph -- and therefore ownership -- is known.
type Module struct {
	fn func(*Module)

	imports     []*Module
	providers   []ProviderRef
	controllers []ControllerRef
	exports     []ProviderRef
}

// New creates a Module that defers fn until Stage 1 assembly runs. fn is
// expected to call Imports/Providers/Controllers/Exports on the *Module
// passed to it -- no resolution logic runs here.
func New(fn func(*Module)) *Module {
	return &Module{fn: fn}
}

// Imports registers modules this module depends on.
func (m *Module) Imports(mods ...*Module) {
	m.imports = append(m.imports, mods...)
}

// Providers registers providers owned by this module.
func (m *Module) Providers(ps ...ProviderRef) {
	m.providers = append(m.providers, ps...)
}

// Controllers registers controllers owned by this module.
func (m *Module) Controllers(cs ...ControllerRef) {
	m.controllers = append(m.controllers, cs...)
}

// Exports registers the subset of this module's own providers that are
// visible to importing modules. Every exported provider must have also
// been registered via Providers on this same module -- validated at the
// end of Stage 1 assembly.
func (m *Module) Exports(ps ...ProviderRef) {
	m.exports = append(m.exports, ps...)
}

// OwnProviders returns a copy of the providers registered on this module
// via Providers. Read-only: mutating the returned slice does not affect
// this Module's internal state. Used by internal/resolver to search this
// module's own providers before falling back to imports.
func (m *Module) OwnProviders() []ProviderRef {
	return append([]ProviderRef(nil), m.providers...)
}

// OwnControllers returns a copy of the controllers registered on this
// module via Controllers. Read-only: mutating the returned slice does not
// affect this Module's internal state. Used by Stage 2 (running Declare on
// every registered controller) and by a future task's route registration.
func (m *Module) OwnControllers() []ControllerRef {
	return append([]ControllerRef(nil), m.controllers...)
}

// ImportedModules returns a copy of the modules registered on this module
// via Imports. Read-only: mutating the returned slice does not affect this
// Module's internal state. Used by internal/resolver to walk imports when
// a type is not found among this module's own providers.
func (m *Module) ImportedModules() []*Module {
	return append([]*Module(nil), m.imports...)
}

// ExportedProviders returns a copy of the providers registered on this
// module via Exports. Read-only: mutating the returned slice does not
// affect this Module's internal state. Used by internal/resolver to
// determine which of an imported module's own providers are visible to
// importers.
func (m *Module) ExportedProviders() []ProviderRef {
	return append([]ProviderRef(nil), m.exports...)
}
