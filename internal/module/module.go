// Package module implements the declarative Module API of the DI graph:
// Module.New(fn) defers fn until Stage 1 assembly runs, and Module.Imports/
// Providers/Controllers/Exports register structural references between
// modules, providers, and controllers.
package module

// ProviderRef is a minimal marker interface satisfied by the real
// *provider.Provider type (owned by internal/provider). Module never needs
// to know the concrete provider type — it only tracks registered
// participants for Stage 1's structural bookkeeping.
//
// Exported (not just structurally implementable) because Go ties
// unexported interface methods to the declaring package: a method named
// isProvider() defined on *provider.Provider in a different package can
// never satisfy an unexported isProvider() declared here, even with an
// identical signature. Exporting the interface — not the marker method
// itself — is what makes cross-package satisfaction possible.
type ProviderRef interface {
	IsProvider()
}

// ControllerRef is the Controller equivalent of ProviderRef. Same
// cross-package rationale.
type ControllerRef interface {
	IsController()
}

// Owner is the contract implemented by Provider and Controller to report
// which Module they belong to. Used later by MustResolve to determine
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
