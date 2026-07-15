// Package app implements App/NewApp/MustNewApp -- the DI bootstrap
// orchestration that runs a *module.Module root through all 3 DI stages.
// Per AD-004 (see .specs/project/STATE.md), it lives in its own internal/
// package; the gonest root package only re-exports it (type alias +
// generic-safe wrapper functions).
package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/gonest-dev/gonest/internal/emitter"
	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/filter"
	"github.com/gonest-dev/gonest/internal/guard"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/logger"
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

	// moduleCount/controllerCount/routeCount are populated by
	// registerRoutes (Stage 2.5) -- used by MustListen's own startup log
	// line ("Loaded: Modules(N), Controllers(N), Routes(N)").
	moduleCount     int
	controllerCount int
	routeCount      int
}

// Adapter returns the HttpAdapter NewApp[T] constructed and registered
// routes on during Stage 2.5. Unexported callers within internal/app (and
// this package's own tests) use it to reach the concrete adapter instance --
// e.g. to call fiber.FiberApp's Listen for a later feature, or in this
// task's own end-to-end test to dispatch a request through the real
// *fiber.App. Not part of any public re-export decision yet; that belongs to
// whichever future feature adds Listen to the public API.
func (a *App) Adapter() HttpAdapter {
	return a.adapter
}

// Root returns the same *module.Module NewApp/MustNewApp was called with --
// the root of the fully assembled module tree, post-bootstrap. It exists so
// something OUTSIDE internal/app (a future schema generator, most
// immediately -- see the "Schema Generation from Metadata" feature's
// context.md "Discovery" section) can walk the whole app graph after the
// fact: recurse via Module.ImportedModules(), then Module.OwnControllers()
// and Controller.OwnRoutes() (all three already exist) to reach every
// controller/route this app registered.
//
// This is needed because registerRoutes (Stage 2.5, above) only ever
// extracts 3 primitives per route (method, path, a composed handler
// closure) for the HTTP adapter and discards its own *route.Route/
// *controller.Controller references immediately after -- nothing from that
// specific walk survives. The richer object graph itself does survive
// (Module/Controller retain their own Own*() accessors for the app's whole
// lifetime); Root is simply the missing entry point to reach it from
// outside this package.
func (a *App) Root() *module.Module {
	return a.root
}

// MustListen starts the app serving on addr via the underlying adapter's
// Listen, blocking until it stops. Before onListen (if non-nil) runs,
// gonest prints its OWN 3-line startup log via internal/logger -- NOT the
// underlying adapter's own default console output (e.g. Fiber's ASCII-art
// banner, suppressed at the adapter level -- see
// internal/adapter/fiber.FiberApp.Listen) -- matching Nest's own startup
// log convention, so every adapter (present or future) presents identically
// regardless of which HTTP engine actually answers requests underneath.
// Panics, using the same "Must"-prefixed panic-on-error convention as
// MustNewApp/MustInject/MustParam, if adapter.Listen returns an error (e.g.
// the addr is already in use) -- the panic message contains both addr and
// the underlying error.
func (a *App) MustListen(addr string, onListen OnListen) {
	onListenFunc := func() {
		logger.Info(fmt.Sprintf("Gonest started on: http://%s", addr))
		logger.Info(fmt.Sprintf("Loaded:            Modules(%d), Controllers(%d), Routes(%d)", a.moduleCount, a.controllerCount, a.routeCount))
		logger.Info(fmt.Sprintf("PID:               %d", os.Getpid()))
		if onListen != nil {
			onListen()
		}
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
// contract -- included here now so implementers, like *fiber.FiberApp,
// already satisfy the whole contract T8 needs).
//
// Init is the one addition beyond design.md's stated RegisterRoute/Listen
// pair, needed to solve a real construction problem: NewApp[T] builds T's
// zero value via reflect (see newAdapter below), and for a type like
// *fiber.FiberApp whose real work happens in its own New() constructor
// (wrapping a *fiber.App), a reflect-constructed zero value has a nil
// underlying engine -- calling RegisterRoute on it would nil-panic. Init is
// the lazy-init hook NewApp[T] calls once, immediately after construction,
// so any adapter needing real setup work gets the chance to do it (see
// FiberApp.Init in internal/adapter/fiber/fiber.go for the concrete case).
type HttpAdapter interface {
	// Init performs any setup a freshly, reflectively constructed zero
	// value of this adapter type needs before RegisterRoute/Listen are
	// safe to call. Implementations must be idempotent -- NewApp[T] calls
	// it exactly once, but nothing prevents an adapter's own constructor
	// (like fiber.New) from also calling it internally.
	Init()
	// RegisterRoute wires one route (method + full path) onto the real
	// underlying HTTP engine, translating h into whatever handler shape
	// that engine expects.
	RegisterRoute(method route.HttpMethod, path string, h func(ctx *execution.Context)) error
	// Listen starts the underlying HTTP engine serving on addr, blocking
	// until it stops. onListen, if non-nil, is invoked exactly once, after
	// the underlying engine has successfully bound addr but before Listen's
	// call blocks for good (i.e. before its accept loop takes over
	// permanently) -- implementations must guard against a nil onListen
	// rather than calling it unconditionally. Not exercised by NewApp[T]
	// itself -- reserved for the "App Bootstrap & Listen" feature that
	// follows this one (see App.MustListen).
	Listen(addr string, onListen func()) error
	// Test dispatches req against the underlying HTTP engine IN-MEMORY, with
	// no real network port bound -- used by MustNewTestApp's MustRequest
	// (the "HTTP Test Client" feature) to exercise a registered route
	// without ever calling Listen. Every adapter implementation (today only
	// Fiber, via *fiber.App's own Test method) provides this identically to
	// however its underlying engine already supports in-memory dispatch.
	Test(req *http.Request) (*http.Response, error)
}

// httpAdapterPtr is the generic-constraint counterpart to HttpAdapter,
// needed because the call-site contract (INSIGHT.md lines 103/317/658) is
// `gonest.NewApp[gonest.FiberApp](AppModule)` -- FiberApp passed BY VALUE as
// the sole type argument -- while *fiber.FiberApp's methods (and any
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
	logger.Configure(opts.LogLevels)
	inject.Reset()
	registerFrameworkSingletons()

	modules, err := root.Assemble()
	if err != nil {
		return nil, err
	}

	// Phase 1: declare + fully resolve every module's own Providers. Any
	// MustInject call inside a Provider's builder closure is Provider-to-
	// Provider (placeholder+PendingEdge, unchanged) -- see
	// .specs/features/test-app-bootstrap/design.md's Architecture Overview.
	declareProviders(modules)

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := resolver.Resolve(ctx, modules); err != nil {
		return nil, err
	}

	// Phase 2: declare every module's own Controllers, now that every
	// Provider is a real, resolved value -- MustInject/MustInjectAll calls
	// inside a Controller's builder closure resolve DIRECTLY (Controller
	// satisfies internal/inject's directResolver post-Stage-1 assembly, see
	// controller.go's ResolveDirect/ResolveDirectAll).
	declareControllers(modules)
	declareListeners(modules)
	declareSchedulers(modules)

	// Phase 3: for every DISTINCT Middleware/Guard/Interceptor/Filter value
	// referenced anywhere (by a Controller's Use/Guards/Interceptors/
	// Filters, now fully recorded since phase 2 just completed, or by a
	// Module's own global Use/Filters), run its OWN (now-deferred, AD-008
	// reversed) builder closure exactly once, with an effective
	// MustInject/MustInjectAll search scope equal to the union of every
	// referencing module's own+exported providers.
	ownership := discoverPipelineStageOwnership(modules)
	declarePipelineStageTypes(ownership)

	adapter := newAdapter[T, PT]()
	if err := registerRoutes(adapter, root, modules); err != nil {
		return nil, err
	}

	moduleCount, controllerCount, routeCount := countTree(modules)
	return &App{root: root, adapter: adapter, opts: opts, moduleCount: moduleCount, controllerCount: controllerCount, routeCount: routeCount}, nil
}

// countTree tallies how many modules/controllers/routes the assembled tree
// contains -- used by MustListen's own startup log line. Controllers
// without a routableController's own OwnRoutes() (should not happen for
// *controller.Controller, but this mirrors registerRoutes' own defensive
// type-assertion) are simply skipped for the route tally, same as
// registerRoutes does for actual registration.
func countTree(modules []*module.Module) (moduleCount, controllerCount, routeCount int) {
	moduleCount = len(modules)
	for _, m := range modules {
		controllers := m.OwnControllers()
		controllerCount += len(controllers)
		for _, c := range controllers {
			if rc, ok := c.(routableController); ok {
				routeCount += len(rc.OwnRoutes())
			}
		}
	}
	return moduleCount, controllerCount, routeCount
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
	OwnFilters() []*filter.Filter
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
// middleware (via OwnMiddleware()), wrapping the controller's Guards,
// wrapping the controller's Interceptor chain, wrapping the route Handler
// itself (innermost) -- Middleware -> Guard -> Interceptor -> Handler, per
// ROADMAP.md's documented order and the interceptor feature's design.md
// CORRECTION note (a Guard must be able to reject a request BEFORE any
// Interceptor "before" logic runs) -- see composeHandler/gatedHandler/
// interceptedHandler below for the composition algorithm.
func registerRoutes(adapter HttpAdapter, root *module.Module, modules []*module.Module) error {
	type collected struct {
		method  route.HttpMethod
		path    string
		handler func(ctx *execution.Context)
	}

	var routes []collected
	seen := map[string]bool{}
	globalMiddleware := asMiddleware(root.OwnMiddleware())
	globalFilters := asFilters(root.OwnFilters())

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
			controllerFilters := rc.OwnFilters()
			for _, r := range rc.OwnRoutes() {
				fullPath := prefix + r.Path()
				key := r.Method().String() + " " + fullPath
				if seen[key] {
					return fmt.Errorf("duplicate route: %s", key)
				}
				seen[key] = true
				intercepted := interceptedHandler(controllerInterceptors, r.HandlerFunc())
				gated := gatedHandler(controllerGuards, intercepted)
				composedHandler := composeHandler(globalMiddleware, controllerMiddleware, gated)
				// withRoute is the outermost layer of all: it attaches r
				// (this specific *route.Route) to ctx via ctx.WithRoute
				// BEFORE anything else in the composed chain runs --
				// MustParams[T]/MustQuery[T] (internal/validate) rely on
				// ctx.Route() returning the current *route.Route to check
				// route.HasParam(key) for presence; without this, ctx.Route()
				// would always be nil during real dispatch and every path
				// param would be treated as absent. currentRoute is
				// captured per-iteration (loop variable, not a pointer to a
				// shared slice element) so each route's closure captures
				// its own *route.Route correctly. Named currentRoute, not
				// route, to avoid shadowing the internal/route package
				// import used elsewhere in this function/file.
				currentRoute := r
				withRoute := func(ctx *execution.Context) {
					ctx.WithRoute(currentRoute)
					composedHandler(ctx)
				}
				// filteredHandler is the NEW outermost layer of all: it wraps
				// withRoute (which already wraps everything else) in its own
				// selective recover, so it is the first thing to run and the
				// last to get a chance at a panic before the adapter's own
				// recover does. See filteredHandler's own doc comment.
				filtered := filteredHandler(controllerFilters, globalFilters, withRoute)
				routes = append(routes, collected{method: r.Method(), path: fullPath, handler: filtered})
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

// filteredHandler builds the NEW OUTERMOST layer of the whole per-route
// dispatch chain -- it wraps next (as wired from registerRoutes, next is
// withRoute, which itself already wraps composeHandler -> gatedHandler ->
// interceptedHandler -> the bare route Handler) in its own, separate
// defer/recover. This is a MORE INNER recover than the adapter's own
// (internal/adapter/fiber, unchanged): it runs first, selectively handling a
// recovered exception.Exception whose concrete type matches a registered
// Catch, and re-panicking anything it doesn't handle so the adapter's
// EXISTING recover still applies its unchanged default {name,message,details}
// formatting.
//
// Lookup order is controller-level filters first, then global (root module)
// filters -- per design.md's Tech Decisions, more specific scope overrides
// broader scope, matching Nest's own filter-precedence convention. A
// recovered value that does not even satisfy exception.Exception (a bare
// error, panic(nil)'s *runtime.PanicNilError, etc.) is re-panicked
// immediately, without ever consulting any Filter's Catch map -- Filters only
// ever catch structured exception.Exception values.
func filteredHandler(controllerFilters, globalFilters []*filter.Filter, next func(ctx *execution.Context)) func(ctx *execution.Context) {
	return func(ctx *execution.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			if exc, ok := r.(exception.Exception); ok {
				excType := reflect.TypeOf(exc)
				if h, found := findCatch(controllerFilters, excType); found {
					h.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})
					return
				}
				if h, found := findCatch(globalFilters, excType); found {
					h.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})
					return
				}
			}
			panic(r) // not caught by any Filter -- re-panic, the adapter's
			// own EXISTING recover (unchanged) applies the default formatting
		}()
		next(ctx)
	}
}

// findCatch looks up excType across filters in order, returning the first
// match's handler -- used by filteredHandler for both the controller-level
// and global lookup passes.
func findCatch(filters []*filter.Filter, excType reflect.Type) (reflect.Value, bool) {
	for _, f := range filters {
		if h, ok := f.HandlerFor(excType); ok {
			return h, true
		}
	}
	return reflect.Value{}, false
}

// gatedHandler builds the OUTER of the two new layers Stage 2.5 feeds into
// the EXISTING middleware-composition loop (composeHandler, unchanged): it
// evaluates controllerGuards in order BEFORE calling routeHandler (which, as
// wired from registerRoutes, is actually interceptedHandler's output -- see
// the CORRECTION note in the interceptor feature's design.md). Any guard
// whose HandlerFunc() returns false stops the chain by panicking
// exception.NewForbiddenException(nil) -- caught by the same recover
// wrapper any other panic (Handler or middleware) already is, per
// design.md's Error Handling Strategy table. A guard's own handler panicking
// with a custom exception.Exception propagates unchanged (no recover here)
// so that panic is formatted with ITS OWN status/body downstream. Guards run
// in registration order and short-circuit on the first false -- later
// guards never run once an earlier one fails, and (per this fix) an
// Interceptor's own "before" logic never runs either, since gatedHandler now
// wraps interceptedHandler's output rather than the bare route Handler. When
// controllerGuards is empty, the loop runs zero iterations and routeHandler
// runs immediately -- behaviorally identical to calling routeHandler
// directly (zero regression for controllers that never call Guards).
func gatedHandler(controllerGuards []*guard.Guard, routeHandler func(ctx *execution.Context)) func(ctx *execution.Context) {
	return func(ctx *execution.Context) {
		for _, g := range controllerGuards {
			if !g.HandlerFunc()(ctx) {
				panic(exception.NewForbiddenException(nil))
			}
		}
		routeHandler(ctx)
	}
}

// interceptedHandler builds the INNER of the two new layers Stage 2.5
// inserts BELOW gatedHandler (Guard) and the existing middleware-composition
// loop (composeHandler, unchanged): it wraps the BARE route Handler
// (routeHandler, NOT gatedHandler's output) with the controller's
// interceptor chain, using the exact same composition shape composeHandler
// itself already uses (registration order, outward composition -- the first
// registered interceptor ends up OUTERMOST, matching Nest's own interceptor
// semantics and this feature's design.md "Composition change"). Each
// interceptor's HandlerFunc is a (ctx, next) continuation -- calling next
// runs the rest of the chain (a later interceptor, or eventually routeHandler
// itself); code physically after that call in the interceptor's own function
// body runs AFTER the whole inner chain returns. registerRoutes then feeds
// THIS function's output into gatedHandler (Guard wraps Interceptor, not the
// other way around) -- see the CORRECTION note in the interceptor feature's
// design.md for why: a Guard must be able to reject a request BEFORE any
// Interceptor "before" logic runs, matching ROADMAP.md's documented order
// (Middleware -> Guard -> Interceptor -> Handler). When controllerInterceptors
// is empty, the loop runs zero iterations and the returned func behaves
// identically to calling routeHandler directly -- zero regression for
// controllers that never call Interceptors.
func interceptedHandler(controllerInterceptors []*interceptor.Interceptor, routeHandler func(ctx *execution.Context)) func(ctx *execution.Context) {
	next := interceptor.Next(routeHandler)
	for i := len(controllerInterceptors) - 1; i >= 0; i-- {
		it := controllerInterceptors[i]
		captured := next // capture per-iteration -- classic Go closure-loop-variable bug otherwise
		next = func(ctx *execution.Context) { it.HandlerFunc()(ctx, captured) }
	}

	return func(ctx *execution.Context) { next(ctx) }
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
func composeHandler(globalMiddleware, controllerMiddleware []*middleware.Middleware, handler func(ctx *execution.Context)) func(ctx *execution.Context) {
	chain := append(append([]*middleware.Middleware{}, globalMiddleware...), controllerMiddleware...)

	next := middleware.Next(handler)
	for i := len(chain) - 1; i >= 0; i-- {
		mw := chain[i]
		captured := next // capture per-iteration -- classic Go closure-loop-variable bug otherwise
		next = func(ctx *execution.Context) { mw.HandlerFunc()(ctx, captured) }
	}

	return func(ctx *execution.Context) { next(ctx) }
}

// asMiddleware converts root's []module.MiddlewareRef (module stores these
// as a marker interface only, to avoid an import cycle with
// internal/middleware -- see module.MiddlewareRef's own doc comment) back
// to the concrete []*middleware.Middleware this file's composition helpers
// (composeHandler) actually need. Every value module.Module.OwnMiddleware()
// returns was constructed via middleware.New, so this type assertion never
// fails in practice.
func asMiddleware(refs []module.MiddlewareRef) []*middleware.Middleware {
	out := make([]*middleware.Middleware, len(refs))
	for i, r := range refs {
		out[i] = r.(*middleware.Middleware)
	}
	return out
}

// asFilters is asMiddleware's Filter equivalent (see its own doc comment).
func asFilters(refs []module.FilterRef) []*filter.Filter {
	out := make([]*filter.Filter, len(refs))
	for i, r := range refs {
		out[i] = r.(*filter.Filter)
	}
	return out
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

// declareProviders runs Declare (phase 1) on every provider registered
// across modules, exactly once each. MustInject calls made during Declare
// are Provider-to-Provider (placeholder+PendingEdge, resolved by the
// following resolver.Resolve call in NewApp) -- order among providers does
// not matter here, only that every provider's OWN Declare has run before
// Stage 3 (resolver.Resolve) walks pending edges.
func declareProviders(modules []*module.Module) {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			if d, ok := p.(declarable); ok {
				d.Declare()
			}
		}
	}
}

// declareControllers runs Declare (phase 2) on every controller registered
// across modules, exactly once each. Called AFTER resolver.Resolve (Stage
// 3) has fully resolved every Provider, so MustInject/MustInjectAll calls
// made during a Controller's Declare resolve DIRECTLY (no placeholder) --
// see controller.go's ResolveDirect/ResolveDirectAll.
func declareControllers(modules []*module.Module) {
	for _, m := range modules {
		for _, c := range m.OwnControllers() {
			if d, ok := c.(declarable); ok {
				d.Declare()
			}
		}
	}
}

// declareListeners runs Declare on every listener registered across
// modules, exactly once each -- same phase 2 as declareControllers (a
// Listener has exactly one owning module, registered directly via
// Module.Listeners, like Controller; MustOn/MustInject calls inside its
// builder fn resolve DIRECTLY, same as Controller's own).
func declareListeners(modules []*module.Module) {
	for _, m := range modules {
		for _, l := range m.OwnListeners() {
			if d, ok := l.(declarable); ok {
				d.Declare()
			}
		}
	}
}

// declareSchedulers runs Declare on every scheduler registered across
// modules, exactly once each -- same phase 2 as declareControllers/
// declareListeners (a Scheduler has exactly one owning module, registered
// directly via Module.Schedulers; Cron/Interval/Timeout calls made inside
// its builder fn spawn their background goroutines immediately, as fn
// runs).
func declareSchedulers(modules []*module.Module) {
	for _, m := range modules {
		for _, s := range m.OwnSchedulers() {
			if d, ok := s.(declarable); ok {
				d.Declare()
			}
		}
	}
}

// registerFrameworkSingletons registers every framework-provided singleton
// (today: only *emitter.Emitter) via internal/inject.RegisterGlobalSingleton,
// called once per bootstrap right after inject.Reset() -- so
// MustInject[*Emitter] resolves from ANY module, with no explicit
// registration, for the ENTIRE lifetime of this specific bootstrap (a new
// Emitter instance is created per NewApp/MustNewTestApp call, matching
// inject.Reset()'s own "one bootstrap at a time per process" contract).
func registerFrameworkSingletons() {
	em := emitter.New()
	inject.RegisterGlobalSingleton(reflect.TypeOf(em), reflect.ValueOf(em))
}

// pipelineStageType is satisfied by *middleware.Middleware/*guard.Guard/
// *interceptor.Interceptor/*filter.Filter (all 4 expose an identically
// shaped Declare(scope []*module.Module), per this feature's AD-008
// reversal). Declared as a small interface (rather than keying
// discoverPipelineStageOwnership's map by `any`) so declarePipelineStageTypes
// can call Declare directly on every distinct key without a further type
// switch across 4 concrete types.
type pipelineStageType interface {
	Declare(scope []*module.Module)
}

// discoverPipelineStageOwnership walks every module's OwnControllers() (
// fully declared by this point, phase 2 having just completed -- so every
// Use()/Guards()/Interceptors()/Filters() call any Controller's builder
// closure made has already run) plus every module's OWN global Use()/
// Filters() registrations, and builds the union-of-referencing-modules
// ownership map that phase 3 (declarePipelineStageTypes) needs: for every
// DISTINCT Middleware/Guard/Interceptor/Filter value (deduplicated by
// pointer identity, since the map key is the interface value itself),
// the set of every module that referenced it, deduplicated by module
// identity as well (the SAME module referencing the same value twice, e.g.
// via two different controllers, must not appear twice in its own scope
// slice). See .specs/features/test-app-bootstrap/design.md's "Direct
// resolution scope" section for the full worked example.
func discoverPipelineStageOwnership(modules []*module.Module) map[pipelineStageType][]*module.Module {
	ownership := make(map[pipelineStageType][]*module.Module)

	addRef := func(ref pipelineStageType, m *module.Module) {
		for _, existing := range ownership[ref] {
			if existing == m {
				return
			}
		}
		ownership[ref] = append(ownership[ref], m)
	}

	for _, m := range modules {
		for _, c := range m.OwnControllers() {
			rc, ok := c.(routableController)
			if !ok {
				continue
			}
			for _, mw := range rc.OwnMiddleware() {
				addRef(mw, m)
			}
			for _, g := range rc.OwnGuards() {
				addRef(g, m)
			}
			for _, ic := range rc.OwnInterceptors() {
				addRef(ic, m)
			}
			for _, f := range rc.OwnFilters() {
				addRef(f, m)
			}
		}

		for _, mw := range asMiddleware(m.OwnMiddleware()) {
			addRef(mw, m)
		}
		for _, f := range asFilters(m.OwnFilters()) {
			addRef(f, m)
		}
	}

	return ownership
}

// declarePipelineStageTypes runs Declare(scope) exactly once on every
// distinct pipeline-stage-type value discovered by
// discoverPipelineStageOwnership -- this is phase 3, the point at which
// Middleware/Guard/Interceptor/Filter builder closures (and any
// MustInject/MustInjectAll calls inside them) actually run, with the
// union-scoped visibility computed above.
func declarePipelineStageTypes(ownership map[pipelineStageType][]*module.Module) {
	for ref, scope := range ownership {
		ref.Declare(scope)
	}
}
