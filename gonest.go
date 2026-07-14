// Package gonest is the public entry point of the framework. Per AD-004 (see
// .specs/project/STATE.md): Go's internal/ path convention blocks any
// package outside this module from importing internal/*, so this file is
// the ONLY door external consumers have -- everything below is a type alias
// (for types) or a var-aliased/wrapped function (for constructors) pointing
// at the real implementation living under internal/<concept>. All real
// logic, encapsulation, and test coverage live in internal/*; this file
// intentionally carries none of its own.
package gonest

import (
	"github.com/gonest-dev/gonest/internal/app"
	"github.com/gonest-dev/gonest/internal/controller"
	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/guard"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/middleware"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/pipe"
	"github.com/gonest-dev/gonest/internal/provider"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/scope"
)

// ---------------------------------------------------------------------------
// Module / Provider / Scope / Controller -- DI graph (Provider & DI Graph,
// Module Composition features)
// ---------------------------------------------------------------------------

// Module represents a declarative unit of the DI graph: its own providers
// and controllers, the modules it imports, and the subset of its own
// providers it exports to importers.
type Module = module.Module

// NewModule creates a Module that defers fn until Stage 1 assembly runs
// during bootstrap.
var NewModule = module.New

// Provider represents one resolvable type in the DI graph: it holds the
// Scope, Constructor, and (after bootstrap) the resolved instance.
type Provider = provider.Provider

// NewProvider creates a Provider that defers fn until Stage 2 builder
// execution runs during bootstrap.
var NewProvider = provider.New

// Scope defines the lifetime of a provider instance within the DI container.
type Scope = scope.Scope

const (
	// ScopeSingleton means a single shared instance is created and reused
	// for the lifetime of the application.
	ScopeSingleton = scope.Singleton
	// ScopeTransient means a new instance is created every time the
	// provider is resolved.
	ScopeTransient = scope.Transient
	// ScopeRequest means a single instance is created per incoming request
	// and shared across resolutions within that request.
	ScopeRequest = scope.Request
)

// Controller represents a declarative unit that consumes providers via
// MustInject. It does not participate in the provider resolution graph --
// it only consumes placeholders it requests.
type Controller = controller.Controller

// NewController creates a Controller that defers fn until bootstrap runs
// it.
var NewController = controller.New

// MustInject declares a dependency on type T (which must be a pointer
// type, e.g. *Foo) from owner's builder fn -- used inside a Provider's or
// Controller's deferred builder fn. It allocates and returns a placeholder
// value; the real module-scoped search happens in a later bootstrap stage.
// It panics if T is not a pointer type. Go cannot re-export a generic
// function via var, so this is a real wrapper calling the internal one.
func MustInject[T any](owner module.Owner) T {
	return inject.MustInject[T](owner)
}

// ---------------------------------------------------------------------------
// App / bootstrap (App Bootstrap & Listen feature)
// ---------------------------------------------------------------------------

// App is the minimal handle returned once NewApp has finished bootstrapping
// the whole dependency graph. Because this is a true Go type alias (not a
// defined type), every method on app.App -- including MustListen -- is
// automatically visible on gonest.App with zero extra wrapper code.
type App = app.App

// HttpAdapter is the constraint NewApp[T]/MustNewApp[T] require of their
// type argument -- e.g. FiberApp. See internal/app.HttpAdapter's doc
// comment for the full contract (RegisterRoute, Listen, Init).
type HttpAdapter = app.HttpAdapter

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels). See internal/app.AppOptions's doc comment for the
// full contract.
type AppOptions = app.AppOptions

// LogLevel identifies one of Nest's 5 standard log severities. See
// internal/app.LogLevel's doc comment for the full contract (iota-based
// const block plus a debug-friendly String()).
type LogLevel = app.LogLevel

const (
	// LogLevelError is the most severe level -- unrecoverable failures.
	LogLevelError = app.LogLevelError
	// LogLevelWarn signals a recoverable but noteworthy condition.
	LogLevelWarn = app.LogLevelWarn
	// LogLevelLog is Nest's default, general-purpose informational level.
	LogLevelLog = app.LogLevelLog
	// LogLevelDebug carries diagnostic detail useful during development.
	LogLevelDebug = app.LogLevelDebug
	// LogLevelVerbose is the most granular, chattiest level.
	LogLevelVerbose = app.LogLevelVerbose
)

// OnListen is the "bind succeeded" callback shape passed to App.MustListen.
// See internal/app.OnListen's doc comment for its nil-safety contract.
type OnListen = app.OnListen

// NewApp bootstraps root through all 3 DI stages (Structural Assembly,
// Builder Execution, Cycle Detection + Parallel Resolution) plus Stage 2.5
// (route collection/registration onto a T-typed HttpAdapter, e.g.
// gonest.NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})). Go cannot
// re-export a generic function via var, so this is a real wrapper calling
// the internal one (same pattern as MustInject/MustParam, see AD-004 in
// STATE.md).
//
// T is passed BY VALUE at the call site (e.g. FiberApp, not *FiberApp) per
// INSIGHT.md's ground-truth call-site contract; the second type parameter
// enforcing "*T implements HttpAdapter" is inferred automatically -- see
// internal/app's httpAdapterPtr for why that extra parameter exists. See
// internal/app.NewApp's doc comment for the full bootstrap contract.
//
// opts is threaded straight through to internal/app.NewApp -- this wrapper
// does not interpret it.
func NewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts AppOptions) (*App, error) {
	return app.NewApp[T, PT](root, opts)
}

// MustNewApp calls NewApp and panics if it returns an error.
func MustNewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts AppOptions) *App {
	return app.MustNewApp[T, PT](root, opts)
}

// ---------------------------------------------------------------------------
// Route params (Controller & Route Registration feature)
// ---------------------------------------------------------------------------

// MustParam converts the route param named name (from ctx.Param) into the
// requested type T, using a custom Pipe if the current route registered one
// for name (via Route.Param), or the default reflect+strconv coercion
// otherwise. Panics if name isn't declared on the current route's path, or
// if the value fails to convert. Go cannot re-export a generic function via
// var, so this is a real wrapper calling the internal one (same pattern as
// MustInject, see AD-004 in STATE.md).
func MustParam[T any](ctx *execution.Context, name string) T {
	return route.MustParam[T](ctx, name)
}

// Pipe represents a param transform: a reflect-validated function that
// coerces a raw route param string into a typed value, attached to a
// specific route param via Route.Param (e.g. INSIGHT.md's
// `route.Param("user_id", ParseIntPipe)`). Unlike Middleware/Guard/
// Interceptor, Pipe.New(fn) defers fn until Route.Param declares it -- see
// internal/pipe.Pipe's doc comment for the full contract.
type Pipe = pipe.Pipe

// NewPipe creates a Pipe that defers fn until it is attached to a route via
// Route.Param, which declares it at that point (see internal/route.Route's
// Param doc comment for why: nothing in the DI bootstrap walks
// Route-registered Pipes, since Pipes aren't tracked by any Module -- Route
// itself is responsible for declaring any Pipe it's given). Like
// NewMiddleware/NewGuard/NewInterceptor/NewHttpException, New here is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed (root package is the only public door since
// Go blocks external import of internal/*, per AD-004 in STATE.md). See
// internal/pipe.New's doc comment for the full contract.
var NewPipe = pipe.New

// ---------------------------------------------------------------------------
// Exceptions (HttpException Core feature)
// ---------------------------------------------------------------------------

// Exception is the single assertion point for "is this value a structured
// HTTP exception". It is satisfied purely structurally -- any type that
// embeds HttpException gets Status/Name/Message/Details promoted
// automatically, with no explicit "implements Exception" needed. See
// internal/exception.Exception's doc comment for the full contract.
type Exception = exception.Exception

// HttpException is the concrete carrier of an exception's four pieces of
// data (status, name, message, details). It is meant to be embedded BY
// VALUE by both the framework's built-in exceptions (below) and any
// dev-defined exception type, e.g. INSIGHT.md's
// `type FooExampleError struct { gonest.HttpException }`. See
// internal/exception.HttpException's doc comment for the full contract.
type HttpException = exception.HttpException

// NewHttpException builds an HttpException from its four parts, returning a
// VALUE (not a pointer) for embedding into a struct literal, e.g.
// `HttpException: gonest.NewHttpException(status, name, message, details)`.
// Unlike NewApp/MustParam elsewhere in this package, NewHttpException is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed. See internal/exception.NewHttpException's doc
// comment for the full contract.
var NewHttpException = exception.NewHttpException

// NotFoundException is the framework's built-in exception for a missing
// resource.
type NotFoundException = exception.NotFoundException

// NewNotFoundException builds a *NotFoundException fixed at
// http.StatusNotFound with name "NotFoundException". See
// internal/exception.NewNotFoundException's doc comment for the
// pointer-return and empty-message rationale.
var NewNotFoundException = exception.NewNotFoundException

// BadRequestException is the framework's built-in exception for a malformed
// or invalid request.
type BadRequestException = exception.BadRequestException

// NewBadRequestException builds a *BadRequestException fixed at
// http.StatusBadRequest with name "BadRequestException". See
// internal/exception.NewBadRequestException's doc comment for the
// pointer-return and empty-message rationale.
var NewBadRequestException = exception.NewBadRequestException

// ConflictException is the framework's built-in exception for a request
// that conflicts with the current state of a resource.
type ConflictException = exception.ConflictException

// NewConflictException builds a *ConflictException fixed at
// http.StatusConflict with name "ConflictException". See
// internal/exception.NewConflictException's doc comment for the
// pointer-return and empty-message rationale.
var NewConflictException = exception.NewConflictException

// UnauthorizedException is the framework's built-in exception for a missing
// or invalid authentication credential.
type UnauthorizedException = exception.UnauthorizedException

// NewUnauthorizedException builds a *UnauthorizedException fixed at
// http.StatusUnauthorized with name "UnauthorizedException". See
// internal/exception.NewUnauthorizedException's doc comment for the
// pointer-return and empty-message rationale.
var NewUnauthorizedException = exception.NewUnauthorizedException

// ForbiddenException is the framework's built-in exception for a request
// that is authenticated but not permitted.
type ForbiddenException = exception.ForbiddenException

// NewForbiddenException builds a *ForbiddenException fixed at
// http.StatusForbidden with name "ForbiddenException". See
// internal/exception.NewForbiddenException's doc comment for the
// pointer-return and empty-message rationale.
var NewForbiddenException = exception.NewForbiddenException

// ---------------------------------------------------------------------------
// Middleware (Middleware feature)
// ---------------------------------------------------------------------------

// Middleware represents a single reusable request-observation/mutation
// unit with a continuation-passing (ctx, next) shape, mirroring
// Express/Nest middleware. It is attached via Controller.Use/Module.Use
// (e.g. INSIGHT.md's `controller.Use(RequestIdMiddleware)`). See
// internal/middleware.Middleware's doc comment for the full contract.
type Middleware = middleware.Middleware

// Next represents the continuation of the middleware chain: calling it
// runs whatever comes after the current middleware (the next middleware,
// or eventually the route's own Handler). See internal/middleware.Next's
// doc comment for why its underlying type is identical in shape to a route
// Handler.
type Next = middleware.Next

// NewMiddleware creates a Middleware and runs fn on it immediately (unlike
// Provider/Module/Controller/Pipe, which defer fn until bootstrap). Like
// NewHttpException, New here is not generic, so Go allows aliasing the
// plain func directly via var -- no wrapper function is needed (root
// package is the only public door since Go blocks external import of
// internal/*, per AD-004 in STATE.md). See internal/middleware.New's doc
// comment for the full contract.
var NewMiddleware = middleware.New

// ---------------------------------------------------------------------------
// Guard (Guard feature)
// ---------------------------------------------------------------------------

// Guard represents a single authorization-check unit: it holds the
// ctx-in/bool-out handler function registered via Handler. Unlike
// Middleware, a Guard doesn't decorate/wrap a continuation -- it gates:
// true means continue, false (or a panic'd Exception) means stop. It is
// attached via Controller.Guards (e.g. INSIGHT.md's
// `controller.Guards(AuthGuard)`). See internal/guard.Guard's doc comment
// for the full contract.
type Guard = guard.Guard

// NewGuard creates a Guard and runs fn on it immediately (unlike
// Provider/Module/Controller/Pipe, which defer fn until bootstrap). This
// feature deliberately has no MustInject support (AD-008 in STATE.md: a
// *Guard can be attached to multiple controllers across different modules,
// with no clean single "owner" to resolve MustInject against), so there is
// no bootstrap stage left to usefully defer fn to. Like NewMiddleware/
// NewHttpException, New here is not generic, so Go allows aliasing the
// plain func directly via var -- no wrapper function is needed (root
// package is the only public door since Go blocks external import of
// internal/*, per AD-004 in STATE.md). See internal/guard.New's doc
// comment for the full contract.
var NewGuard = guard.New

// ---------------------------------------------------------------------------
// Interceptor (Interceptor feature)
// ---------------------------------------------------------------------------

// Interceptor represents a single reusable before/after-Handler-execution
// unit with a continuation-passing (ctx, next) shape, wrapping a route's
// Handler for AOP-style pre/post-processing (timing, transformation,
// caching, etc.), mirroring Nest interceptors. It is attached via
// Controller.Interceptors (e.g. INSIGHT.md's
// `controller.Interceptors(TimingInterceptor)`). See
// internal/interceptor.Interceptor's doc comment for the full contract.
type Interceptor = interceptor.Interceptor

// NewInterceptor creates an Interceptor and runs fn on it immediately
// (unlike Provider/Module/Controller/Pipe, which defer fn until
// bootstrap). This feature deliberately has no MustInject support (AD-008
// in STATE.md: pipeline-stage types don't support MustInject in v1, same
// reasoning as NewGuard/NewMiddleware), so there is no bootstrap stage
// left to usefully defer fn to. Like NewGuard/NewMiddleware/
// NewHttpException, New here is not generic, so Go allows aliasing the
// plain func directly via var -- no wrapper function is needed (root
// package is the only public door since Go blocks external import of
// internal/*, per AD-004 in STATE.md). See internal/interceptor.New's doc
// comment for the full contract.
var NewInterceptor = interceptor.New
