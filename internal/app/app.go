// Package app implements App/NewApp/MustNewApp -- the DI bootstrap
// orchestration that runs a *module.Module root through all 3 DI stages.
// Per AD-004 (see .specs/project/STATE.md), it lives in its own internal/
// package; the gonest root package only re-exports it (type alias +
// generic-safe wrapper functions).
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/guard"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/middleware"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolver"
	"github.com/gonest-dev/gonest/internal/route"
)

// bootstrapTimeout bounds Stage 3 (Parallel Resolution): every Provider's
// Constructor -- including ones that don't accept a context.Context
// themselves -- runs within this deadline collectively, since it is the ctx
// passed to errgroup.WithContext. Not yet configurable via AppOptions
// (that arrives with the "App Bootstrap & Listen" feature); a fixed
// generous default keeps NewApp usable today without hanging forever on a
// Constructor that never returns.
const bootstrapTimeout = 30 * time.Second

// App is the minimal handle returned once NewApp has finished bootstrapping
// the whole dependency graph. It is intentionally opaque at this stage --
// Listen/AppOptions/UseLogger etc. belong to the future "App Bootstrap &
// Listen" feature. It exists now only so NewApp/MustNewApp have a return
// type to hand back.
type App struct {
	root    *module.Module
	adapter HttpAdapter
	opts    AppOptions
}

// Adapter returns the HttpAdapter NewApp[T] constructed and registered
// routes on during Stage 2.5. Unexported callers within internal/app (and
// this package's own tests) use it to reach the concrete adapter instance --
// e.g. to call fiberapp.FiberApp's Listen for a later feature, or in this
// task's own end-to-end test to dispatch a request through the real
// *fiber.App. Not part of any public re-export decision yet; that belongs to
// whichever future feature adds Listen to the public API.
func (a *App) Adapter() HttpAdapter {
	return a.adapter
}

// MustListen starts the app serving on addr via the underlying adapter's
// Listen, blocking until it stops. onListen, if non-nil, is wrapped into a
// plain func() and passed through to adapter.Listen -- a nil onListen is
// passed straight through as nil, relying on HttpAdapter.Listen's own
// documented nil-safety rather than gonest wrapping it in a no-op closure.
// Panics, using the same "Must"-prefixed panic-on-error convention as
// MustNewApp/MustInject/MustParam, if adapter.Listen returns an error (e.g.
// the addr is already in use) -- the panic message contains both addr and
// the underlying error.
func (a *App) MustListen(addr string, onListen OnListen) {
	var onListenFunc func()
	if onListen != nil {
		onListenFunc = func() { onListen() }
	}
	if err := a.adapter.Listen(addr, onListenFunc); err != nil {
		panic(fmt.Sprintf("gonest: failed to listen on %q: %v", addr, err))
	}
}

// HttpAdapter is the minimal contract an HTTP adapter must satisfy for
// NewApp[T] to use it as T. Mirrors design.md's "FiberApp (adapter)"
// component's stated contract exactly: RegisterRoute wires one gonest Route
// onto the real underlying HTTP engine, Listen starts it serving (only used
// by a later feature, per INSIGHT.md's note next to the httpAdapter
// contract -- included here now so implementers, like *fiberapp.FiberApp,
// already satisfy the whole contract T8 needs).
//
// Init is the one addition beyond design.md's stated RegisterRoute/Listen
// pair, needed to solve a real construction problem: NewApp[T] builds T's
// zero value via reflect (see newAdapter below), and for a type like
// *fiberapp.FiberApp whose real work happens in its own New() constructor
// (wrapping a *fiber.App), a reflect-constructed zero value has a nil
// underlying engine -- calling RegisterRoute on it would nil-panic. Init is
// the lazy-init hook NewApp[T] calls once, immediately after construction,
// so any adapter needing real setup work gets the chance to do it (see
// FiberApp.Init in internal/fiberapp/fiberapp.go for the concrete case).
type HttpAdapter interface {
	// Init performs any setup a freshly, reflectively constructed zero
	// value of this adapter type needs before RegisterRoute/Listen are
	// safe to call. Implementations must be idempotent -- NewApp[T] calls
	// it exactly once, but nothing prevents an adapter's own constructor
	// (like fiberapp.New) from also calling it internally.
	Init()
	// RegisterRoute wires one route (method + full path) onto the real
	// underlying HTTP engine, translating h into whatever handler shape
	// that engine expects.
	RegisterRoute(method route.HttpMethod, path string, h func(ctx *httpctx.Context)) error
	// Listen starts the underlying HTTP engine serving on addr, blocking
	// until it stops. onListen, if non-nil, is invoked exactly once, after
	// the underlying engine has successfully bound addr but before Listen's
	// call blocks for good (i.e. before its accept loop takes over
	// permanently) -- implementations must guard against a nil onListen
	// rather than calling it unconditionally. Not exercised by NewApp[T]
	// itself -- reserved for the "App Bootstrap & Listen" feature that
	// follows this one (see App.MustListen).
	Listen(addr string, onListen func()) error
}

// httpAdapterPtr is the generic-constraint counterpart to HttpAdapter,
// needed because the call-site contract (INSIGHT.md lines 103/317/658) is
// `gonest.NewApp[gonest.FiberApp](AppModule)` -- FiberApp passed BY VALUE as
// the sole type argument -- while *fiberapp.FiberApp's methods (and any
// other real adapter's) are pointer-receiver only, so FiberApp itself does
// not satisfy HttpAdapter, only *FiberApp does.
//
// This is the standard Go idiom for "T is a value type, but *T implements
// the interface I actually need to call methods on": a second, unexported
// type parameter PT constrained to `*T` plus HttpAdapter's method set. Go's
// type inference derives PT from T at call sites that only ever name T
// explicitly (see NewApp/MustNewApp below), so `NewApp[FiberApp](root)`
// keeps working with a single type argument -- PT is never spelled out by
// callers.
type httpAdapterPtr[T any] interface {
	*T
	HttpAdapter
}

// NewApp bootstraps root through all 3 DI stages plus Stage 2.5 (route
// collection/registration), in order:
//
//  1. Structural Assembly (root.Assemble): walks Imports, wires
//     OwnerModule on every registered provider/controller, validates
//     Exports.
//  2. Builder Execution: runs Declare on every provider and controller
//     across the assembled module tree, so MustInject calls inside their
//     builder fn run against a fully-known module tree and record pending
//     edges. This is also the point at which every Controller's own Route
//     calls have run (Controller.Declare runs the controller's deferred fn,
//     which is expected to call Controller.Route) -- so by the time Stage 2
//     finishes, every module's OwnControllers()/OwnRoutes() reflects the
//     complete, final set of declared routes.
//  3. Cycle detection, then Parallel Resolution: resolves every registered
//     Provider concurrently (respecting dependency edges recorded in Stage
//     2), copying each resolved instance in place into every placeholder
//     MustInject returned for it.
//     2.5. Route collection/registration: constructs T (the HttpAdapter),
//     walks the same assembled module tree Stage 1 produced, builds the
//     full path (Controller.PathPrefix()+Route.Path()) for every route on
//     every controller, detects a method+path collision BEFORE registering
//     anything (an app with a colliding route must never partially come
//     up), then registers every route via adapter.RegisterRoute. Runs after
//     Stage 3 (not interleaved with it): route registration has no
//     dependency-ordering concerns among itself, and running it last means
//     a DI failure (Stage 1-3) is reported before any HTTP-specific error,
//     matching the existing precedence of structural errors over
//     resolution errors.
//
// It returns once the whole graph is resolved and every route registered,
// or an error from Stage 1 (structural/export validation), cycle detection,
// Stage 3 (a Constructor's returned error or recovered panic), or Stage 2.5
// (a duplicate route).
//
// NewApp calls inject.Reset() at the very start, before Stage 2, to clear
// internal/inject's process-global pending-edge bookkeeping left over from
// any previous NewApp call -- see inject.Reset's doc comment for the "one
// bootstrap at a time per process" contract this establishes. Calling
// NewApp concurrently from multiple goroutines in the same process is not
// supported; NewApp is meant to run once, synchronously, at process
// startup.
//
// opts is a required second positional parameter (not optional/variadic --
// ground truth is INSIGHT.md's own call sites, which always pass it, even
// as a zero value AppOptions{}). It is stored on the returned *App after
// every bootstrap stage above completes, and does not influence any of
// them -- no Logger exists yet in this codebase to act on BufferLogs/
// LogLevels (see AppOptions' doc comment in options.go).
func NewApp[T any, PT httpAdapterPtr[T]](root *module.Module, opts AppOptions) (*App, error) {
	inject.Reset()

	modules, err := root.Assemble()
	if err != nil {
		return nil, err
	}

	declareAll(modules)

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := resolver.Resolve(ctx, modules); err != nil {
		return nil, err
	}

	adapter := newAdapter[T, PT]()
	if err := registerRoutes(adapter, root, modules); err != nil {
		return nil, err
	}

	return &App{root: root, adapter: adapter, opts: opts}, nil
}

// MustNewApp calls NewApp and panics if it returns an error. Convenience
// for callers (typically main) that treat bootstrap failure as fatal.
func MustNewApp[T any, PT httpAdapterPtr[T]](root *module.Module, opts AppOptions) *App {
	app, err := NewApp[T, PT](root, opts)
	if err != nil {
		panic(err)
	}
	return app
}

// newAdapter constructs T's zero value and returns it as PT (which, per the
// httpAdapterPtr constraint, is *T and satisfies HttpAdapter). Using
// `PT(new(T))` rather than reflect keeps this a plain, inference-friendly
// generic conversion -- reflect.New was the originally suggested approach,
// but the two-type-parameter constraint above already gets Go's compiler to
// do the "value type in, pointer-that-implements-the-interface out" work,
// so reaching for reflect here would just be redundant indirection. Init is
// then called exactly once so the adapter can replace any
// construction-produced nil internals with real ones (see HttpAdapter's own
// doc comment for why this exists).
func newAdapter[T any, PT httpAdapterPtr[T]]() PT {
	adapter := PT(new(T))
	adapter.Init()
	return adapter
}

// routableController is a locally-declared interface used to type-assert
// module.ControllerRef values down to the methods Stage 2.5 needs
// (PathPrefix, OwnRoutes, OwnMiddleware, OwnGuards, OwnInterceptors) but
// that module.ControllerRef itself does not expose -- same pattern as this
// file's own declarable interface just below, and for the same reason:
// PathPrefix/OwnRoutes/OwnMiddleware/OwnGuards/OwnInterceptors are route-
// collection/composition concerns, not something module.Module needs to
// know about a registered controller. Already implemented by
// *controller.Controller (see internal/controller/controller.go) --
// OwnMiddleware was added there by T2 of the "Middleware" feature,
// OwnGuards by T2 of the "Guard" feature, OwnInterceptors by T2 of the
// "Interceptor" feature -- no controller-side change was needed here beyond
// widening this interface.
type routableController interface {
	PathPrefix() string
	OwnRoutes() []*route.Route
	OwnMiddleware() []*middleware.Middleware
	OwnGuards() []*guard.Guard
	OwnInterceptors() []*interceptor.Interceptor
}

// registerRoutes implements Stage 2.5: walk every module's OwnControllers,
// build each route's full path, detect a method+path collision across the
// WHOLE tree before registering anything, then register every route on
// adapter. Two passes over the same collected route list (collision check,
// then registration) rather than registering-as-we-go, specifically so a
// collision found on, say, the last controller visited still means NOTHING
// got registered -- per design.md's Error Handling Strategy ("servidor não
// sobe"), a colliding app must never end up with some routes live and
// others rejected.
//
// root is the literal *module.Module NewApp was called with -- the SAME
// value, not every module in the assembled tree -- since per the
// "Middleware" feature's design.md Tech Decisions, global middleware
// (root.OwnMiddleware()) is scoped to the root module only, never cascaded
// to imported modules. Each route's registered handler is not the bare
// route.HandlerFunc() but a composed chain: root's global middleware
// (outermost, runs first) wrapping that route's own controller's
// middleware (via OwnMiddleware()) wrapping the route Handler itself
// (innermost) -- see composeHandler below for the composition algorithm.
func registerRoutes(adapter HttpAdapter, root *module.Module, modules []*module.Module) error {
	type collected struct {
		method  route.HttpMethod
		path    string
		handler func(ctx *httpctx.Context)
	}

	var routes []collected
	seen := map[string]bool{}
	globalMiddleware := root.OwnMiddleware()

	for _, m := range modules {
		for _, c := range m.OwnControllers() {
			rc, ok := c.(routableController)
			if !ok {
				continue
			}
			prefix := rc.PathPrefix()
			controllerMiddleware := rc.OwnMiddleware()
			controllerGuards := rc.OwnGuards()
			controllerInterceptors := rc.OwnInterceptors()
			for _, r := range rc.OwnRoutes() {
				fullPath := prefix + r.Path()
				key := r.Method().String() + " " + fullPath
				if seen[key] {
					return fmt.Errorf("duplicate route: %s", key)
				}
				seen[key] = true
				gated := gatedHandler(controllerGuards, r.HandlerFunc())
				intercepted := interceptedHandler(controllerInterceptors, gated)
				composedHandler := composeHandler(globalMiddleware, controllerMiddleware, intercepted)
				routes = append(routes, collected{method: r.Method(), path: fullPath, handler: composedHandler})
			}
		}
	}

	for _, rt := range routes {
		if err := adapter.RegisterRoute(rt.method, rt.path, rt.handler); err != nil {
			return err
		}
	}

	return nil
}

// gatedHandler builds the new innermost layer Stage 2.5 feeds into the
// EXISTING middleware-composition loop (composeHandler, unchanged): it
// evaluates controllerGuards in order BEFORE calling routeHandler. Any guard
// whose HandlerFunc() returns false stops the chain by panicking
// exception.NewForbiddenException(nil) -- caught by the same recover
// wrapper any other panic (Handler or middleware) already is, per
// design.md's Error Handling Strategy table. A guard's own handler panicking
// with a custom exception.Exception propagates unchanged (no recover here)
// so that panic is formatted with ITS OWN status/body downstream. Guards run
// in registration order and short-circuit on the first false -- later
// guards never run once an earlier one fails. When controllerGuards is
// empty, the loop runs zero iterations and routeHandler runs immediately --
// behaviorally identical to calling routeHandler directly (zero regression
// for controllers that never call Guards).
func gatedHandler(controllerGuards []*guard.Guard, routeHandler func(ctx *httpctx.Context)) func(ctx *httpctx.Context) {
	return func(ctx *httpctx.Context) {
		for _, g := range controllerGuards {
			if !g.HandlerFunc()(ctx) {
				panic(exception.NewForbiddenException(nil))
			}
		}
		routeHandler(ctx)
	}
}

// interceptedHandler builds the new layer Stage 2.5 inserts BETWEEN the
// existing gatedHandler (Guard) and the existing middleware-composition loop
// (composeHandler, unchanged): it wraps gatedHandler with the controller's
// interceptor chain, using the exact same composition shape composeHandler
// itself already uses (registration order, outward composition -- the first
// registered interceptor ends up OUTERMOST, matching Nest's own interceptor
// semantics and this feature's design.md "Composition change"). Each
// interceptor's HandlerFunc is a (ctx, next) continuation -- calling next
// runs the rest of the chain (a later interceptor, or eventually
// gatedHandler itself); code physically after that call in the
// interceptor's own function body runs AFTER the whole inner chain returns.
// Neither gatedHandler nor composeHandler's loop is restructured by this --
// interceptedHandler only changes what gets passed BETWEEN them (was
// gatedHandler directly, now interceptedHandler's output). When
// controllerInterceptors is empty, the loop runs zero iterations and the
// returned func behaves identically to calling gatedHandler directly --
// zero regression for controllers that never call Interceptors.
func interceptedHandler(controllerInterceptors []*interceptor.Interceptor, gated func(ctx *httpctx.Context)) func(ctx *httpctx.Context) {
	next := interceptor.Next(gated)
	for i := len(controllerInterceptors) - 1; i >= 0; i-- {
		it := controllerInterceptors[i]
		captured := next // capture per-iteration -- classic Go closure-loop-variable bug otherwise
		next = func(ctx *httpctx.Context) { it.HandlerFunc()(ctx, captured) }
	}

	return func(ctx *httpctx.Context) { next(ctx) }
}

// composeHandler builds the final handler for one route: global middleware
// (outermost, runs first) wrapping controllerMiddleware wrapping handler
// (innermost). Follows design.md's "Composition algorithm" exactly --
// starting from handler as the innermost middleware.Next and composing
// outward so chain[0] (global-first) ends up OUTERMOST. When both
// globalMiddleware and controllerMiddleware are empty (an app with zero
// Use() calls anywhere), the loop body never runs and the returned func
// behaves identically to registering handler directly -- zero regression
// per design.md's Error Handling Strategy table.
func composeHandler(globalMiddleware, controllerMiddleware []*middleware.Middleware, handler func(ctx *httpctx.Context)) func(ctx *httpctx.Context) {
	chain := append(append([]*middleware.Middleware{}, globalMiddleware...), controllerMiddleware...)

	next := middleware.Next(handler)
	for i := len(chain) - 1; i >= 0; i-- {
		mw := chain[i]
		captured := next // capture per-iteration -- classic Go closure-loop-variable bug otherwise
		next = func(ctx *httpctx.Context) { mw.HandlerFunc()(ctx, captured) }
	}

	return func(ctx *httpctx.Context) { next(ctx) }
}

// declarable is satisfied by both *provider.Provider and
// *controller.Controller (both expose Declare). Declared locally so this
// file can invoke Declare on module.ProviderRef/module.ControllerRef values
// without either of those interfaces needing to expose it -- Declare is a
// bootstrap-orchestration concern, not part of what Module needs from a
// registered participant.
type declarable interface {
	Declare()
}

// declareAll runs Declare (Stage 2 builder execution) on every provider and
// controller registered across modules, exactly once each. Order does not
// matter: MustInject calls made during Declare only ever look up an
// already-assembled module tree (Stage 1 has already fully run by the time
// this is called from NewApp), never another provider/controller's
// Declare-time state.
func declareAll(modules []*module.Module) {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			if d, ok := p.(declarable); ok {
				d.Declare()
			}
		}
		for _, c := range m.OwnControllers() {
			if d, ok := c.(declarable); ok {
				d.Declare()
			}
		}
	}
}
