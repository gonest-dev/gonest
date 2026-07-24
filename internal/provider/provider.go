// Package provider implements the declarative Provider API of the DI
// graph: Provider.New(fn) defers fn until Stage 2 builder execution runs,
// and Provider.Scope/Constructor register configuration on the *Provider
// passed to fn.
package provider

import (
	"context"
	"reflect"
	"sync"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/scope"
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
	declared    bool

	resolvedValueMu sync.Mutex
	resolvedValue   reflect.Value

	onModuleInit              reflect.Value
	onApplicationBootstrap    reflect.Value
	onModuleDestroy           reflect.Value
	beforeApplicationShutdown reflect.Value
	onApplicationShutdown     reflect.Value
}

// New creates a Provider that defers fn until Stage 2 builder execution
// runs. fn is expected to call Scope/Constructor on the *Provider passed
// to it -- no resolution logic runs here.
func New(fn func(*Provider)) *Provider {
	return &Provider{fn: fn, scope: scope.Singleton}
}

// Declare runs this provider's deferred fn exactly once. It is a no-op on
// any call after the first (including when fn is nil), so callers that walk
// the assembled module tree (Stage 2 of bootstrap) can call Declare on every
// registered provider without needing to track which ones they already
// visited. This is what makes MustInject calls inside fn actually happen
// and get recorded as pending edges -- before Declare runs, fn has never
// executed (see New's doc comment).
func (p *Provider) Declare() {
	if p.declared {
		return
	}
	p.declared = true
	if p.fn != nil {
		p.fn(p)
	}
}

// IsProvider is a marker method that satisfies module.ProviderRef, so
// *Provider can be passed to (*module.Module).Providers without module
// needing to import this package. Exported: Go ties unexported interface
// methods to the declaring package, so an unexported marker here could
// never satisfy module's interface across packages.
func (p *Provider) IsProvider() {}

// IsToken is a marker method that satisfies module.TokenRef (via
// module.ProviderRef, which embeds it), so *Provider can be passed to
// (*module.Module).Exports alongside *module.Module re-export arguments.
func (p *Provider) IsToken() {}

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

// SetResolvedValue stores v as this provider's fully-resolved instance. It
// is called by internal/resolver's Stage 3, right after callConstructor
// succeeds for this provider -- in ADDITION to the existing
// placeholder-copy-in-place behavior Stage 3 already performed before this
// was added (purely additive, see ResolvedValue's own doc comment for why a
// separate storage is used rather than repurposing the placeholder
// mechanism). Guarded by a mutex: a scope.Transient provider's Stage 3 path
// (invokeAndCopyEdge) invokes Constructor once per pending edge, each in its
// own goroutine, so multiple concurrent SetResolvedValue calls on the SAME
// *Provider are a real possibility (last-write-wins is an accepted
// implementation detail for Transient -- direct resolution of a Transient
// provider's "resolved value" is not a scenario this feature's spec
// requires, only Singleton's single stable value is), not just a
// theoretical race the mutex happens to also close off.
func (p *Provider) SetResolvedValue(v reflect.Value) {
	p.resolvedValueMu.Lock()
	defer p.resolvedValueMu.Unlock()
	p.resolvedValue = v
}

// ResolvedValue returns the value stored via SetResolvedValue, and whether
// one has been set yet. Used by internal/resolver's direct-resolution
// helpers (FindDirect/FindDirectAll), which run strictly after Stage 3
// completes and only ever need to READ an already-resolved value once --
// unlike the placeholder mechanism (which exists to hand a caller a stable
// address BEFORE the value exists), a direct-resolution caller has no need
// for that indirection at all.
func (p *Provider) ResolvedValue() (reflect.Value, bool) {
	p.resolvedValueMu.Lock()
	defer p.resolvedValueMu.Unlock()
	if !p.resolvedValue.IsValid() {
		return reflect.Value{}, false
	}
	return p.resolvedValue, true
}

// Scope sets the lifetime of this provider's instance. If never called,
// the default is scope.Singleton.
func (p *Provider) Scope(s scope.Scope) {
	p.scope = s
}

// ResolvedScope returns the scope this provider was configured with (or
// scope.Singleton if Scope was never called). Named ResolvedScope, not
// Scope, because Scope is already the setter -- Go does not allow method
// overloading. Used by internal/resolver's Stage 3 engine to decide
// per-scope resolution strategy (only scope.Singleton is handled by T9;
// Transient/Request are later tasks).
func (p *Provider) ResolvedScope() scope.Scope {
	return p.scope
}

// ConstructorFunc returns the reflect.Value of the Constructor stored via
// Constructor(fn any), ready to be invoked directly (fn.Call(...)) by
// internal/resolver's Stage 3 engine. Returns an invalid (zero)
// reflect.Value if Constructor has not been called yet.
func (p *Provider) ConstructorFunc() reflect.Value {
	return p.constructor
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
