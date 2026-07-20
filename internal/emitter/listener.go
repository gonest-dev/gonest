package emitter

import (
	"context"
	"reflect"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Listener represents a single unit of event-handling registration, bound
// to EventType via its own type parameter -- a genuinely generic struct
// (not type-erased), unlike the free-function-based approach this replaced
// (removed MustOn). This is possible without violating Go's "no type
// parameter on a method" rule (L-001 in STATE.md) because On below does NOT
// introduce a new type parameter -- it only USES the one already bound to
// the receiver, Listener[EventType], which Go explicitly allows.
type Listener[EventType any] struct {
	fn      func(*Listener[EventType])
	handler reflect.Value

	owner    *module.Module
	declared bool
}

// NewListener creates a Listener bound to EventType, deferring fn until
// Declare runs it -- same deferred-builder pattern as Provider/Controller
// (AD-015's 3-phase bootstrap), since fn typically calls MustInject, which
// needs a known module scope to resolve against. fn receives the Listener
// itself and is expected to call On exactly once, registering the actual
// per-event handler -- resolving any dependency ONCE, at Declare time, then
// closing over it for every future event, the same resolve-once-at-
// declare-time idiom Controller's own builder (resolving a dependency once,
// outside Route's per-request Handler, then closing over it) already
// establishes throughout this codebase:
//
//	NewListener(func(l *Listener[UserCreatedEvent]) {
//	  logger := MustInject[*LoggerService](l)
//	  l.MustOn(func(ctx context.Context, event UserCreatedEvent) {
//	    logger.Log("user created", event.UserID)
//	  })
//	})
//
// Named NewListener, not New (like every other package in this codebase
// uses), because this package ALSO exports Emitter's own New() -- the two
// would collide otherwise.
func NewListener[EventType any](fn func(*Listener[EventType])) *Listener[EventType] {
	return &Listener[EventType]{fn: fn}
}

// On registers handler as this Listener's per-event callback -- the
// error-returning form. A regular method, not a free function like the
// removed MustOn (the old free-function one; see MustOn below for this
// package's current, unrelated meaning of that name) -- EventType is
// already bound to this *Listener[EventType] by NewListener, so On simply
// uses it, introducing no type parameter of its own (see Listener's own
// doc comment for why that distinction is what makes this a valid method).
// A non-nil error returned by handler is logged by Emit (internal/logger,
// same treatment a recovered panic already gets), never propagated to
// Emit's own caller -- Emit's fire-and-forget contract holds regardless of
// whether a listener fails via panic or via a returned error. Calling On
// (or MustOn) more than once overwrites the previously registered handler;
// NewListener's fn is expected to call one of the two exactly once.
func (l *Listener[EventType]) On(handler func(ctx context.Context, event EventType) error) {
	l.handler = reflect.ValueOf(handler)
}

// MustOn registers handler as this Listener's per-event callback -- the
// convenience form for a handler that never fails, so it doesn't have to
// write "return nil" itself. Despite the name, MustOn does NOT panic on
// anything (unlike this framework's other Must-prefixed functions, e.g.
// MustInject/MustParse/MustListen, which all panic on failure) -- the name
// was chosen deliberately anyway (confirmed with the user) purely to read
// naturally alongside On, not to signal panic semantics.
func (l *Listener[EventType]) MustOn(handler func(ctx context.Context, event EventType)) {
	l.On(func(ctx context.Context, event EventType) error {
		handler(ctx, event)
		return nil
	})
}

// Declare runs this listener's deferred fn exactly once -- idempotent, same
// contract as Provider.Declare/Controller.Declare -- then registers the
// handler On stored on the CURRENT bootstrap's Emitter singleton, reached
// via internal/inject's global-singleton registry (the SAME lookup
// MustInject[*Emitter] itself performs, so a handler registered here is
// guaranteed to be found by whatever Emit call later targets the SAME
// running bootstrap's Emitter -- no explicit Emitter reference needs to be
// threaded through NewListener's own call site). Panics if called before
// any bootstrap has registered an Emitter singleton (should not happen in
// practice: Declare only ever runs during phase 2 of NewApp/
// MustNewTestApp, well after the Emitter singleton is registered at the
// very start of bootstrap). A no-op (not a panic) if fn never called On --
// treated as "this Listener was registered but has nothing to listen for
// yet", not a programmer error worth failing bootstrap over.
func (l *Listener[EventType]) Declare() {
	if l.declared {
		return
	}
	l.declared = true
	if l.fn != nil {
		l.fn(l)
	}
	if !l.handler.IsValid() {
		return
	}

	v, ok := inject.GlobalSingletonFor(emitterType)
	if !ok {
		panic("gonest: Listener.Declare ran with no Emitter singleton registered (bootstrap not started?)")
	}
	em := v.Interface().(*Emitter)
	em.on(reflect.TypeFor[EventType](), l.handler)
}

// IsListener is the marker method that satisfies module.ListenerRef, so
// *Listener[EventType] can be passed to (*module.Module).Listeners without
// module needing to import this package.
func (l *Listener[EventType]) IsListener() {}

// SetOwnerModule associates this listener with the module that owns it.
// Called by module assembly (Stage 1) once ownership is known.
func (l *Listener[EventType]) SetOwnerModule(m *module.Module) {
	l.owner = m
}

// OwnerModule implements module.Owner. Returns nil until SetOwnerModule
// has been called.
func (l *Listener[EventType]) OwnerModule() *module.Module {
	return l.owner
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoped to a SINGLE module: this Listener's own OwnerModule (same
// single-module encapsulation Controller's own ResolveDirect uses -- a
// Listener has exactly one owning module, registered directly via
// Module.Listeners, unlike Middleware/Guard/Interceptor/Filter's union
// scope).
func (l *Listener[EventType]) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{l.owner}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (l *Listener[EventType]) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{l.owner}, t)
}

// emitterType is used to look up the current bootstrap's Emitter singleton
// via internal/inject's global-singleton registry (see Declare's own doc
// comment).
var emitterType = reflect.TypeFor[*Emitter]()
