package provider

import (
	"context"
	"fmt"
	"reflect"

	"gonest.dev/gonest/internal/scope"
)

// isValidHookSignature reports whether t matches one of the 4 accepted
// lifecycle hook signatures:
//
//	func(T)
//	func(T) error
//	func(T, context.Context)
//	func(T, context.Context) error
//
// This is deliberately NOT the same shape space as Constructor's
// isValidConstructorSignature: a Constructor's T is a RETURN value, so
// Constructor never accepts a zero-return shape (it must always produce
// something). A lifecycle hook's T is instead its first PARAMETER (the
// provider's already-resolved instance, fed back in) -- the hook's job is a
// side effect, so a bare `func(T)` with no return at all is perfectly valid
// and is in fact expected to be the common case.
func isValidHookSignature(t reflect.Type) bool {
	switch t.NumIn() {
	case 1:
		// func(T) / func(T) error -- no context.Context param.
	case 2:
		if t.In(1) != contextType {
			return false
		}
	default:
		return false
	}

	switch t.NumOut() {
	case 0:
		return true
	case 1:
		return t.Out(0) == errorType
	default:
		return false
	}
}

// validateHookFunc reflects on fn, panicking with "gonest: invalid <name>
// signature" if it does not match one of the 4 shapes isValidHookSignature
// accepts, and otherwise returns fn's reflect.Value ready to be stored and
// later invoked by fn.Call.
func validateHookFunc(fn any, name string) reflect.Value {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || !isValidHookSignature(v.Type()) {
		panic("gonest: invalid " + name + " signature")
	}
	return v
}

// stringType is used alongside contextType/errorType (declared in
// provider.go) to validate the signal-carrying shutdown hooks' trailing
// `signal string` parameter via reflect.
var stringType = reflect.TypeOf("")

// isValidSignalHookSignature reports whether t matches one of the 4
// accepted signal-carrying lifecycle hook signatures:
//
//	func(T, string)
//	func(T, string) error
//	func(T, context.Context, string)
//	func(T, context.Context, string) error
//
// This mirrors isValidHookSignature's structure exactly, but
// BeforeApplicationShutdown/OnApplicationShutdown always receive the OS
// shutdown signal (e.g. "SIGTERM") as their last argument -- Nest's
// equivalent interfaces pass the signal string too, since a hook may want to
// react differently to a graceful signal vs. a forceful one. The signal is
// mandatory (unlike the optional context.Context), so unlike
// isValidHookSignature it always occupies the final parameter slot.
func isValidSignalHookSignature(t reflect.Type) bool {
	switch t.NumIn() {
	case 2:
		// func(T, string) / func(T, string) error -- no context.Context param.
		if t.In(1) != stringType {
			return false
		}
	case 3:
		if t.In(1) != contextType || t.In(2) != stringType {
			return false
		}
	default:
		return false
	}

	switch t.NumOut() {
	case 0:
		return true
	case 1:
		return t.Out(0) == errorType
	default:
		return false
	}
}

// validateSignalHookFunc is validateHookFunc's counterpart for the
// signal-carrying hooks: it panics with "gonest: invalid <name> signature"
// if fn does not match one of the 4 shapes isValidSignalHookSignature
// accepts, and otherwise returns fn's reflect.Value ready to be stored and
// later invoked by fn.Call.
func validateSignalHookFunc(fn any, name string) reflect.Value {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || !isValidSignalHookSignature(v.Type()) {
		panic("gonest: invalid " + name + " signature")
	}
	return v
}

// OnModuleInit registers fn to run once, right after this provider's
// instance has been constructed, before the application is considered
// bootstrapped. Mirrors Nest's OnModuleInit lifecycle interface, but as a
// plain registered function rather than an interface method -- gonest
// providers are built from plain constructor funcs, not structs
// implementing marker interfaces, so hooks follow the same registration
// style as Constructor/Scope. fn must match one of:
//
//	func(T)
//	func(T) error
//	func(T, context.Context)
//	func(T, context.Context) error
//
// where T is this provider's resolved type. Any other shape panics with
// "gonest: invalid OnModuleInit signature".
func (p *Provider) OnModuleInit(fn any) {
	p.onModuleInit = validateHookFunc(fn, "OnModuleInit")
}

// OnApplicationBootstrap registers fn to run once, after every provider in
// the application has completed OnModuleInit -- Nest's two-phase init
// split (per-module init, then whole-app bootstrap) matters when a
// provider's setup logic needs to assume every OTHER provider is already
// fully initialized, not just itself. Accepted shapes are identical to
// OnModuleInit's; see that method's doc comment. Any other shape panics
// with "gonest: invalid OnApplicationBootstrap signature".
func (p *Provider) OnApplicationBootstrap(fn any) {
	p.onApplicationBootstrap = validateHookFunc(fn, "OnApplicationBootstrap")
}

// OnModuleDestroy registers fn to run once during application shutdown,
// giving a provider a chance to release resources (close connections, flush
// buffers, etc.) using its already-resolved instance. Accepted shapes are
// identical to OnModuleInit's; see that method's doc comment. Any other
// shape panics with "gonest: invalid OnModuleDestroy signature".
func (p *Provider) OnModuleDestroy(fn any) {
	p.onModuleDestroy = validateHookFunc(fn, "OnModuleDestroy")
}

// BeforeApplicationShutdown registers fn to run once, right after a
// shutdown signal (e.g. "SIGTERM") is received but before any
// OnApplicationShutdown/OnModuleDestroy teardown begins -- mirroring Nest's
// BeforeApplicationShutdown, this is the hook to use for work that must
// observe the "still fully live" application (e.g. draining in-flight
// requests) before other providers start tearing down. Unlike
// OnModuleInit's shapes, fn must also accept the shutdown signal as its
// trailing argument. fn must match one of:
//
//	func(T, string)
//	func(T, string) error
//	func(T, context.Context, string)
//	func(T, context.Context, string) error
//
// where T is this provider's resolved type and the string is the shutdown
// signal. Any other shape -- including one of OnModuleInit's 4
// no-signal shapes -- panics with "gonest: invalid BeforeApplicationShutdown
// signature".
func (p *Provider) BeforeApplicationShutdown(fn any) {
	p.beforeApplicationShutdown = validateSignalHookFunc(fn, "BeforeApplicationShutdown")
}

// OnApplicationShutdown registers fn to run once during final application
// teardown, after BeforeApplicationShutdown/OnModuleDestroy have run --
// mirroring Nest's OnApplicationShutdown, this is the last hook to fire
// before the process exits. Accepted shapes are identical to
// BeforeApplicationShutdown's; see that method's doc comment. Any other
// shape panics with "gonest: invalid OnApplicationShutdown signature".
func (p *Provider) OnApplicationShutdown(fn any) {
	p.onApplicationShutdown = validateSignalHookFunc(fn, "OnApplicationShutdown")
}

// RunModuleInit invokes the hook registered via OnModuleInit against this
// provider's ResolvedValue(). See runHook's doc comment for the shared
// no-op/invocation rules every RunXxx method follows.
func (p *Provider) RunModuleInit(ctx context.Context) error {
	return p.runHook(ctx, p.onModuleInit, "OnModuleInit")
}

// RunApplicationBootstrap invokes the hook registered via
// OnApplicationBootstrap against this provider's ResolvedValue(). See
// runHook's doc comment for the shared no-op/invocation rules every RunXxx
// method follows.
func (p *Provider) RunApplicationBootstrap(ctx context.Context) error {
	return p.runHook(ctx, p.onApplicationBootstrap, "OnApplicationBootstrap")
}

// RunModuleDestroy invokes the hook registered via OnModuleDestroy against
// this provider's ResolvedValue(). See runHook's doc comment for the
// shared no-op/invocation rules every RunXxx method follows.
func (p *Provider) RunModuleDestroy(ctx context.Context) error {
	return p.runHook(ctx, p.onModuleDestroy, "OnModuleDestroy")
}

// RunBeforeApplicationShutdown invokes the hook registered via
// BeforeApplicationShutdown against this provider's ResolvedValue(), always
// passing signal as the hook's last argument. See runHook's doc comment for
// the shared no-op rules every RunXxx method follows.
func (p *Provider) RunBeforeApplicationShutdown(ctx context.Context, signal string) error {
	return p.runSignalHook(ctx, p.beforeApplicationShutdown, "BeforeApplicationShutdown", signal)
}

// RunApplicationShutdown invokes the hook registered via
// OnApplicationShutdown against this provider's ResolvedValue(), always
// passing signal as the hook's last argument. See runHook's doc comment for
// the shared no-op rules every RunXxx method follows.
func (p *Provider) RunApplicationShutdown(ctx context.Context, signal string) error {
	return p.runSignalHook(ctx, p.onApplicationShutdown, "OnApplicationShutdown", signal)
}

// runHook is the shared no-op/invocation gate for the 3 no-signal RunXxx
// methods: it returns nil without invoking anything when this provider is
// not scope.Singleton (Transient/Request instances are not tracked as a
// single stable "the" instance to run a lifecycle hook against -- see
// ResolvedValue's doc comment on why only Singleton has a meaningful
// resolved value at all), when hook was never registered (its zero
// reflect.Value fails IsValid), or when ResolvedValue()'s bool is false
// (defensive: call-site ordering in a future task guarantees Stage 3
// resolution completes before any RunXxx call, so this should not happen in
// practice, but a nil hook run should never panic just because it ran
// early).
func (p *Provider) runHook(ctx context.Context, hook reflect.Value, phaseName string) error {
	if p.ResolvedScope() != scope.Singleton {
		return nil
	}
	if !hook.IsValid() {
		return nil
	}
	resolved, ok := p.ResolvedValue()
	if !ok {
		return nil
	}
	return invokeHook(ctx, hook, resolved, phaseName, p.ResolvedType(), nil)
}

// runSignalHook is runHook's counterpart for the 2 signal-carrying RunXxx
// methods: it applies the exact same no-op gate (non-Singleton scope, never
// registered, resolved value not yet set), differing only in that it always
// forwards signal down to invokeHook so it reaches the hook as its last
// call argument.
func (p *Provider) runSignalHook(ctx context.Context, hook reflect.Value, phaseName string, signal string) error {
	if p.ResolvedScope() != scope.Singleton {
		return nil
	}
	if !hook.IsValid() {
		return nil
	}
	resolved, ok := p.ResolvedValue()
	if !ok {
		return nil
	}
	return invokeHook(ctx, hook, resolved, phaseName, p.ResolvedType(), &signal)
}

// invokeHook calls fn (a validated lifecycle hook reflect.Value) with
// resolved as its first argument, appending ctx as the next argument only
// if fn's registered shape accepts one, then appending *signal as the final
// argument if signal is non-nil (the caller passes non-nil only for the
// signal-carrying shutdown hooks, whose accepted shapes -- see
// isValidSignalHookSignature -- always end in a mandatory string param
// after the optional context.Context one), recovering any panic and
// converting it to an error, and interpreting fn's 0-or-1 return values as
// the resulting error (nil if fn returned nothing). Mirrors
// internal/resolver/stage3.go's callConstructor's panic-recovery +
// optional-context-arg + optional-error-return call convention, adapted
// for a hook whose T is a PARAMETER rather than a return value. Keeping
// this single function for both the signal and no-signal hook families
// (rather than a copy-pasted variant) is what guarantees the panic-recovery
// and error-return-interpretation logic can never drift between them.
func invokeHook(ctx context.Context, fn reflect.Value, resolved reflect.Value, phaseName string, typ reflect.Type, signal *string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("gonest: provider for type %s panicked during %s: %v", typ, phaseName, r)
		}
	}()

	args := []reflect.Value{resolved}
	numIn := fn.Type().NumIn()
	if signal != nil {
		// Signal-carrying shapes are func(T, [context.Context], string):
		// a 3-arg func accepts context.Context before the mandatory signal.
		if numIn == 3 {
			args = append(args, reflect.ValueOf(ctx))
		}
		args = append(args, reflect.ValueOf(*signal))
	} else if numIn == 2 {
		args = append(args, reflect.ValueOf(ctx))
	}

	out := fn.Call(args)
	if len(out) == 1 && !out[0].IsNil() {
		return out[0].Interface().(error)
	}
	return nil
}
