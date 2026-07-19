package emitter

import (
	"context"
	"reflect"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Listener represents a single unit of event-handling registration: exactly
// one event type, bound at construction via NewListener's own type
// parameter. Internally type-erased (eventType/handler stored as
// reflect.Type/reflect.Value) because a struct field cannot itself carry an
// unresolved type parameter from a generic constructor -- the same
// type-erasure Provider's own constructor/resolvedValue fields already use.
type Listener struct {
	fn func(*Listener) reflect.Value

	eventType reflect.Type
	owner     *module.Module
	declared  bool
}

// NewListener creates a Listener bound to EventType, deferring fn until
// Declare runs it -- same deferred-builder pattern as Provider/Controller
// (AD-015's 3-phase bootstrap), since fn typically calls MustInject, which
// needs a known module scope to resolve against. fn receives the Listener
// itself (for MustInject/MustInjectAll) and returns the actual per-event
// handler -- resolving any dependency ONCE, at Declare time, then closing
// over it for every future event, rather than re-resolving per event:
//
//	NewListener(func(l *Listener) func(context.Context, UserCreatedEvent) {
//	  logger := MustInject[*LoggerService](l)
//	  return func(ctx context.Context, event UserCreatedEvent) {
//	    logger.Log("user created", event.UserID)
//	  }
//	})
//
// Named NewListener, not New (like every other package in this codebase
// uses), because this package ALSO exports Emitter's own New() -- the two
// would collide otherwise. A free function (not a method), and EventType is
// NewListener's own type parameter (not Listener's), because Go does not
// allow a type parameter on a method and a generic struct here would force
// every call site using a *Listener (module.ListenerRef, ResolveDirect,
// etc.) to also carry EventType around for no benefit (L-001 in STATE.md;
// same rationale the removed free-standing MustOn used to document).
func NewListener[EventType any](fn func(*Listener) func(context.Context, EventType)) *Listener {
	l := &Listener{eventType: reflect.TypeFor[EventType]()}
	l.fn = func(l *Listener) reflect.Value {
		return reflect.ValueOf(fn(l))
	}
	return l
}

// Declare runs this listener's deferred fn exactly once -- idempotent, same
// contract as Provider.Declare/Controller.Declare -- then registers the
// returned handler on the CURRENT bootstrap's Emitter singleton, reached via
// internal/inject's global-singleton registry (the SAME lookup
// MustInject[*Emitter] itself performs, so a handler registered here is
// guaranteed to be found by whatever Emit call later targets the SAME
// running bootstrap's Emitter -- no explicit Emitter reference needs to be
// threaded through NewListener's own call site). Panics if called before
// any bootstrap has registered an Emitter singleton (should not happen in
// practice: Declare only ever runs during phase 2 of NewApp/
// MustNewTestApp, well after the Emitter singleton is registered at the
// very start of bootstrap).
func (l *Listener) Declare() {
	if l.declared {
		return
	}
	l.declared = true
	if l.fn == nil {
		return
	}
	handler := l.fn(l)

	v, ok := inject.GlobalSingletonFor(emitterType)
	if !ok {
		panic("gonest: Listener.Declare ran with no Emitter singleton registered (bootstrap not started?)")
	}
	em := v.Interface().(*Emitter)
	em.on(l.eventType, handler)
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
// via internal/inject's global-singleton registry (see Declare's own doc
// comment).
var emitterType = reflect.TypeFor[*Emitter]()
