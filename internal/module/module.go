// Package module implements the declarative Module API of the DI graph:
// Module.New(fn) defers fn until Stage 1 assembly runs, and Module.Imports/
// Providers/Controllers/Exports register structural references between
// modules, providers, and controllers.
package module

import (
	"reflect"
)

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

// ResolverRef is the GraphQL Resolver equivalent of ControllerRef
// (graphql-support feature, Milestone 17) -- same cross-package marker
// rationale, mirroring ControllerRef's exact shape. Satisfied by the real
// *gqlresolver.Resolver type (internal/gqlresolver), declared here rather
// than importing that package directly for the same reason ControllerRef
// avoids importing internal/controller.
type ResolverRef interface {
	IsResolver()
	// SetOwnerModule associates this resolver with the module that owns
	// it. Called by assemble during Stage 1.
	SetOwnerModule(m *Module)
}

// MiddlewareRef is a minimal marker interface satisfied by the real
// *middleware.Middleware type (owned by internal/middleware). Module never
// needs to know the concrete middleware type -- it only stores registrations
// for the app package's later composition/bootstrap use. Declared here
// (rather than importing internal/middleware directly) specifically to avoid
// an import cycle: internal/middleware needs to import internal/module (for
// Declare(scope []*Module), part of this feature's AD-008 reversal), so
// internal/module cannot import internal/middleware back. Same
// cross-package marker-interface rationale as ProviderRef/ControllerRef
// above (exported marker method, not just an exported interface, since Go
// ties unexported interface methods to the declaring package).
type MiddlewareRef interface {
	IsMiddleware()
}

// FilterRef is the Filter equivalent of MiddlewareRef, same rationale (see
// MiddlewareRef's own doc comment).
type FilterRef interface {
	IsFilter()
}

// ListenerRef is a minimal marker interface satisfied by the real
// *listener.Listener type (owned by internal/emitter). Same cross-package
// rationale as ProviderRef/ControllerRef -- a Listener is owned by exactly
// ONE module (registered directly via Module.Listeners, same level as
// Module.Providers/Controllers), unlike Middleware/Guard/Interceptor/
// Filter's union-of-referencing-modules ownership.
type ListenerRef interface {
	IsListener()
	// SetOwnerModule associates this listener with the module that owns
	// it. Called by assemble during Stage 1, same as ProviderRef/
	// ControllerRef's own SetOwnerModule.
	SetOwnerModule(m *Module)
}

// SchedulerRef is the Scheduler equivalent of ListenerRef -- same
// single-module ownership rationale (a Scheduler is registered directly
// via Module.Schedulers, same level as Providers/Controllers/Listeners).
type SchedulerRef interface {
	IsScheduler()
	SetOwnerModule(m *Module)
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

	name            string
	imports         []*Module
	providers       []ProviderRef
	controllers     []ControllerRef
	resolvers       []ResolverRef
	exports         []ProviderRef
	exportedModules []*Module
	middleware      []MiddlewareRef
	filters         []FilterRef
	listeners       []ListenerRef
	schedulers      []SchedulerRef
}

// New creates a Module that defers fn until Stage 1 assembly runs. fn is
// expected to call Imports/Providers/Controllers/Exports on the *Module
// passed to it -- no resolution logic runs here.
func New(fn func(*Module)) *Module {
	return &Module{fn: fn}
}

// Name sets a human-readable identifier for this module, used in place of
// the pointer-address fallback in error messages produced by Assemble and
// internal/resolver. Calling Name more than once overwrites the previous
// value -- the last call wins. Calling Name("") is equivalent to never
// calling Name: error messages still fall back to the pointer address.
func (m *Module) Name(name string) *Module {
	m.name = name
	return m
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

// Resolvers registers GraphQL resolvers owned by this module (graphql-
// support feature, Milestone 17) -- same registration shape as Controllers.
func (m *Module) Resolvers(rs ...ResolverRef) {
	m.resolvers = append(m.resolvers, rs...)
}

// Exports registers the subset of this module's own providers that are
// visible to importing modules. Every exported provider must have also
// been registered via Providers on this same module -- validated at the
// end of Stage 1 assembly.
func (m *Module) Exports(ps ...ProviderRef) {
	m.exports = append(m.exports, ps...)
}

// ExportModules registers whole modules to re-export: every provider that
// mods themselves expose (via their own Exports, transitively through their
// own re-exported modules) becomes visible through this module too, without
// this module needing to name each provider individually. Every module
// passed here must also have been passed to Imports on this same module --
// validated at the end of Stage 1 assembly, symmetric to how Exports
// requires its providers to also be in Providers.
func (m *Module) ExportModules(mods ...*Module) {
	m.exportedModules = append(m.exportedModules, mods...)
}

// Use registers middleware owned by this module. Go cannot restrict this
// method to "the root module only" at the type level -- any *Module can
// call Use. Per this feature's design.md Tech Decisions, this is
// intentional: Use is merely the storage primitive (mirroring how
// Providers/Controllers store their own registrations); whether a given
// module's middleware is actually CONSULTED during dispatch -- today only
// the root module's is, imported modules' Use registrations are not
// cascaded -- is decided later, by the code that reads OwnMiddleware (a
// later task), not by this method.
func (m *Module) Use(items ...MiddlewareRef) {
	m.middleware = append(m.middleware, items...)
}

// Filters registers filters owned by this module. Go cannot restrict this
// method to "the root module only" at the type level -- any *Module can
// call Filters. Per this feature's design.md Tech Decisions, this is
// intentional: Filters is merely the storage primitive (mirroring how
// Use stores middleware); whether a given module's filters are actually
// CONSULTED during dispatch -- and whether that consultation cascades
// across imported modules or is limited to the root module -- is decided
// later, by the code that reads OwnFilters (a later task), not by this
// method.
func (m *Module) Filters(items ...FilterRef) {
	m.filters = append(m.filters, items...)
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

// OwnResolvers returns a copy of the GraphQL resolvers registered on this
// module via Resolvers. Read-only: mutating the returned slice does not
// affect this Module's internal state. Same defensive-copy pattern as
// OwnControllers.
func (m *Module) OwnResolvers() []ResolverRef {
	return append([]ResolverRef(nil), m.resolvers...)
}

// OwnMiddleware returns a copy of the middleware registered on this module
// via Use. Read-only: mutating the returned slice does not affect this
// Module's internal state. Used by a later task to determine which
// middleware wrap the root module's dispatch chain.
func (m *Module) OwnMiddleware() []MiddlewareRef {
	return append([]MiddlewareRef(nil), m.middleware...)
}

// OwnFilters returns a copy of the filters registered on this module via
// Filters. Read-only: mutating the returned slice does not affect this
// Module's internal state. Used by a later task to determine which filters
// apply during exception handling.
func (m *Module) OwnFilters() []FilterRef {
	return append([]FilterRef(nil), m.filters...)
}

// Listeners registers listeners owned by this module -- same registration
// level as Providers/Controllers (a Listener has exactly ONE owning
// module, unlike Middleware/Guard/Interceptor/Filter's union-of-
// referencing-modules ownership).
func (m *Module) Listeners(ls ...ListenerRef) {
	m.listeners = append(m.listeners, ls...)
}

// OwnListeners returns a copy of the listeners registered on this module
// via Listeners. Read-only: mutating the returned slice does not affect
// this Module's internal state.
func (m *Module) OwnListeners() []ListenerRef {
	return append([]ListenerRef(nil), m.listeners...)
}

// Schedulers registers schedulers owned by this module -- same
// registration level as Providers/Controllers/Listeners.
func (m *Module) Schedulers(ss ...SchedulerRef) {
	m.schedulers = append(m.schedulers, ss...)
}

// OwnSchedulers returns a copy of the schedulers registered on this module
// via Schedulers. Read-only: mutating the returned slice does not affect
// this Module's internal state.
func (m *Module) OwnSchedulers() []SchedulerRef {
	return append([]SchedulerRef(nil), m.schedulers...)
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

// OwnExportedModules returns a copy of the modules registered on this
// module via ExportModules. Read-only: mutating the returned slice does not
// affect this Module's internal state. Used by Stage 1 assembly to validate
// that every re-exported module was also imported, and by EffectiveExports
// to walk the re-export chain.
func (m *Module) OwnExportedModules() []*Module {
	return append([]*Module(nil), m.exportedModules...)
}

// EffectiveExports returns the full set of providers visible to an importer
// of this module: this module's own Exports, plus -- for every module
// registered via ExportModules -- that module's own EffectiveExports,
// walked transitively. Unlike ExportedProviders (this module's own Exports
// only), EffectiveExports follows re-export chains of arbitrary depth, so
// an importer of B can reach a provider exported by D even when B only
// re-exports C and C re-exports D.
//
// A provider that is merely declared via Providers on a re-exported module
// -- but never itself passed to that module's own Exports -- is never
// included: re-exporting a module only propagates what that module already
// chose to make visible, it does not implicitly expose everything the
// module owns.
//
// The result is deduplicated by ProviderRef identity, and cycles in the
// re-export graph (a module re-exporting itself, directly or through a
// mutual chain) terminate safely: a module is only ever expanded once
// across the whole walk.
func (m *Module) EffectiveExports() []ProviderRef {
	return effectiveExports(m, map[*Module]bool{})
}

// effectiveExports is EffectiveExports's recursive worker. visited is
// carried across the entire walk (not reset per branch) so that a cycle --
// including a module re-exporting itself -- terminates: once a module has
// been expanded, revisiting it (from any branch) returns nil instead of
// recursing again.
func effectiveExports(m *Module, visited map[*Module]bool) []ProviderRef {
	if visited[m] {
		return nil
	}
	visited[m] = true

	seen := make(map[ProviderRef]bool, len(m.exports))
	var out []ProviderRef
	add := func(p ProviderRef) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, p := range m.exports {
		add(p)
	}
	for _, re := range m.exportedModules {
		for _, p := range effectiveExports(re, visited) {
			add(p)
		}
	}
	return out
}
