package module

import "reflect"

// LazyModule is the callback argument to Module.Lazy: it exposes the same
// Imports/Exports registration surface as *Module itself (delegating to the
// owning module, owner is unexported so a LazyModule can never be mistaken
// for a full Module by external code), plus OwnProviders so
// internal/inject's Lazy dispatch branch can search this module's own
// providers -- inject holds only a *LazyModule, never the underlying
// *Module (owner is unexported), so this delegation is the only way it can
// reach OwnProviders at all.
type LazyModule struct {
	owner *Module
}

// Lazy constructs a *LazyModule wrapping m and calls fn(l) IMMEDIATELY, not
// deferred -- unlike New(fn) itself, this is safe here precisely because
// Lazy is only ever called FROM INSIDE a module's own already-deferred fn
// (assemble.go's Stage 1 BFS calls m.fn(m), then immediately reads
// m.imports/m.exports to continue the walk -- see assemble.go:43-63). By
// the time Lazy runs, that same fn is already executing synchronously
// inside assemble; running fn(l) here right away, rather than deferring it
// a second time, is what lets l.Imports/l.Exports land in time for Stage 1
// to see them.
func (m *Module) Lazy(fn func(l *LazyModule)) {
	if fn == nil {
		return
	}
	l := &LazyModule{owner: m}
	fn(l)
}

// Imports delegates to the owning Module's own Imports -- same semantics,
// same storage (including its ...TokenRef panic-on-wrong-kind contract),
// just reached through the callback argument instead of the *Module
// directly.
func (l *LazyModule) Imports(refs ...TokenRef) {
	l.owner.Imports(refs...)
}

// Exports delegates to the owning Module's own Exports -- same semantics,
// same storage (including its ...TokenRef panic-on-wrong-kind contract).
func (l *LazyModule) Exports(refs ...TokenRef) {
	l.owner.Exports(refs...)
}

// OwnProviders delegates to the owning Module's own OwnProviders (already a
// defensive copy) -- exposed so internal/inject's Lazy dispatch branch can
// search this module's own providers by ResolvedType, since it only ever
// holds a *LazyModule, never the unexported owner field's *Module directly.
func (l *LazyModule) OwnProviders() []ProviderRef {
	return l.owner.OwnProviders()
}

// LazyInjectFn and LazySafeInjectFn are wired by internal/inject's init to avoid import cycles.
var (
	LazyInjectFn     func(owner any, t reflect.Type) any
	LazySafeInjectFn func(owner any, t reflect.Type) (any, error)
)

// Inject declares a dependency on type T for this lazy module.
func (l *LazyModule) Inject[T any]() T {
	if LazyInjectFn == nil {
		panic("gonest: LazyInjectFn not wired")
	}
	return LazyInjectFn(l, reflect.TypeFor[T]()).(T)
}

// SafeInject declares a dependency on type T for this lazy module.
func (l *LazyModule) SafeInject[T any]() (T, error) {
	if LazySafeInjectFn == nil {
		panic("gonest: LazySafeInjectFn not wired")
	}
	v, err := LazySafeInjectFn(l, reflect.TypeFor[T]())
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

