// Package provider implements the declarative Provider API of the DI
// graph: Provider.New(fn) defers fn until Stage 2 builder execution runs,
// and Provider.Scope/Constructor register configuration on the *Provider
// passed to fn.
package provider

import (
	"context"
	"reflect"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/scope"
)

// contextType and errorType are used to validate Constructor's accepted
// signatures via reflect.
var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// Provider represents one resolvable type in the DI graph: it holds the
// Scope, Constructor, and (after a future Stage 3) the resolved instance.
// It implements module.Owner.
//
// New(fn) does not execute fn at call time. fn is only invoked during
// Stage 2 builder execution (a future task), since that is the earliest
// point at which the provider's owning Module is known.
type Provider struct {
	fn func(*Provider)

	scope       scope.Scope
	constructor reflect.Value

	ownerModule *module.Module
}

// New creates a Provider that defers fn until Stage 2 builder execution
// runs. fn is expected to call Scope/Constructor on the *Provider passed
// to it -- no resolution logic runs here.
func New(fn func(*Provider)) *Provider {
	return &Provider{fn: fn, scope: scope.Singleton}
}

// IsProvider is a marker method that satisfies module.ProviderRef, so
// *Provider can be passed to (*module.Module).Providers without module
// needing to import this package. Exported: Go ties unexported interface
// methods to the declaring package, so an unexported marker here could
// never satisfy module's interface across packages.
func (p *Provider) IsProvider() {}

// SetOwnerModule associates this provider with the module that owns it.
// It is called by module assembly once ownership is known (structural
// assembly walks Module.Providers registrations); a later task wires this
// call in automatically. Exposed for now so callers of the DI bootstrap
// machinery can establish ownership explicitly.
func (p *Provider) SetOwnerModule(m *module.Module) {
	p.ownerModule = m
}

// OwnerModule implements module.Owner. It returns the Module this
// Provider was registered under. Association happens during Stage 1
// structural assembly (a future task) -- until then it returns nil.
func (p *Provider) OwnerModule() *module.Module {
	return p.ownerModule
}

// ResolvedType returns the reflect.Type this provider resolves: the first
// return value's type of the stored Constructor (e.g. *Foo for both
// func() *Foo and func() (*Foo, error)). It implements module.ProviderRef.
// Returns nil if Constructor has not been called yet.
func (p *Provider) ResolvedType() reflect.Type {
	if !p.constructor.IsValid() {
		return nil
	}
	return p.constructor.Type().Out(0)
}

// Scope sets the lifetime of this provider's instance. If never called,
// the default is scope.Singleton.
func (p *Provider) Scope(s scope.Scope) {
	p.scope = s
}

// Constructor registers the builder function used to produce this
// provider's instance. It accepts exactly 4 signatures (validated via
// reflect immediately):
//
//	func() T
//	func() (T, error)
//	func(context.Context) T
//	func(context.Context) (T, error)
//
// Any other signature panics with "gonest: invalid Constructor signature".
// Actual invocation of the stored constructor happens in a future task
// (Stage 3 resolution) -- this method only validates and stores it.
func (p *Provider) Constructor(fn any) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || !isValidConstructorSignature(v.Type()) {
		panic("gonest: invalid Constructor signature")
	}
	p.constructor = v
}

// isValidConstructorSignature reports whether t matches one of the 4
// accepted Constructor signatures:
//
//	func() T
//	func() (T, error)
//	func(context.Context) T
//	func(context.Context) (T, error)
func isValidConstructorSignature(t reflect.Type) bool {
	switch t.NumIn() {
	case 0:
		// func() T / func() (T, error)
	case 1:
		if t.In(0) != contextType {
			return false
		}
	default:
		return false
	}

	switch t.NumOut() {
	case 1:
		return true
	case 2:
		return t.Out(1) == errorType
	default:
		return false
	}
}
