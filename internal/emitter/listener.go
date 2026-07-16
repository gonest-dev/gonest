package emitter

import (
	"context"
	"reflect"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Listener represents a single unit of event-handling registration: its
// builder fn (deferred until Declare runs, same New*-deferred pattern
// Provider/Controller already use) is expected to call MustOn one or more
// times, registering handlers on the CURRENT bootstrap's Emitter singleton.
type Listener struct {
	fn func(*Listener)

	owner    *module.Module
	declared bool
}

// NewListener creates a Listener that defers fn until Declare runs it --
// same deferred-builder pattern as Provider/Controller (AD-015's 3-phase
// bootstrap), since fn is expected to call MustOn/MustInject, which need a
// known module scope to resolve against. Named NewListener, not New (like
// every other package in this codebase uses), because this package ALSO
// exports Emitter's own New() -- the two would collide otherwise.
func NewListener(fn func(*Listener)) *Listener {
	return &Listener{fn: fn}
}

// Declare runs this listener's deferred fn exactly once -- idempotent, same
// contract as Provider.Declare/Controller.Declare.
func (l *Listener) Declare() {
	if l.declared {
		return
	}
	l.declared = true
	if l.fn != nil {
		l.fn(l)
	}
}

// IsListener is the marker method that satisfies module.ListenerRef, so
// *Listener can be passed to (*module.Module).Listeners without module
// needing to import this package.
func (l *Listener) IsListener() {}

// SetOwnerModule associates this listener with the module that owns it.
// Called by module assembly (Stage 1) once ownership is known.
func (l *Listener) SetOwnerModule(m *module.Module) {
	l.owner = m
}

// OwnerModule implements module.Owner. Returns nil until SetOwnerModule
// has been called.
func (l *Listener) OwnerModule() *module.Module {
	return l.owner
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoped to a SINGLE module: this Listener's own OwnerModule (same
// single-module encapsulation Controller's own ResolveDirect uses -- a
// Listener has exactly one owning module, registered directly via
// Module.Listeners, unlike Middleware/Guard/Interceptor/Filter's union
// scope).
func (l *Listener) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{l.owner}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (l *Listener) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{l.owner}, t)
}

// emitterType is used to look up the current bootstrap's Emitter singleton
// via internal/inject's global-singleton registry (see MustOn's own doc
// comment).
var emitterType = reflect.TypeOf((*Emitter)(nil))

// MustOn registers handler as the callback for events of type EventType on
// the CURRENT bootstrap's Emitter singleton -- a free function, not a
// method on *Listener, because Go does not allow a type parameter on a
// method (L-001 in STATE.md). Reaches the Emitter singleton via
// internal/inject.GlobalSingletonFor, the SAME lookup MustInject[*Emitter]
// itself performs, so a handler registered here is guaranteed to be found
// by whatever Emit call later targets the SAME running bootstrap's
// Emitter -- no explicit Emitter reference needs to be threaded through
// MustOn's own call site. Panics if called before any bootstrap has
// registered an Emitter singleton (should not happen in practice: MustOn is
// only ever called from within a Listener's builder fn, itself only run
// during phase 2 of NewApp/MustNewTestApp, well after the Emitter singleton
// is registered at the very start of bootstrap).
func MustOn[EventType any](listener *Listener, handler func(ctx context.Context, event EventType)) {
	v, ok := inject.GlobalSingletonFor(emitterType)
	if !ok {
		panic("gonest: MustOn called with no Emitter singleton registered (bootstrap not started?)")
	}
	em := v.Interface().(*Emitter)
	em.on(reflect.TypeFor[EventType](), reflect.ValueOf(handler))
}
