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
	"fmt"
	"net/http"
	"reflect"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/adapter/fiber"
	"gonest.dev/gonest/internal/app"
	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/dotenv"
	"gonest.dev/gonest/internal/emitter"
	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/filter"
	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/interceptor"
	"gonest.dev/gonest/internal/logger"
	"gonest.dev/gonest/internal/middleware"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/openapi"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/scheduler"
	"gonest.dev/gonest/internal/schema"
	"gonest.dev/gonest/internal/scope"
	"gonest.dev/gonest/internal/value"
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

// HttpMethod identifies the HTTP verb a route responds to.
type HttpMethod = route.HttpMethod

const (
	// HttpGet corresponds to the HTTP GET method.
	HttpGet = route.HttpGet
	// HttpPost corresponds to the HTTP POST method.
	HttpPost = route.HttpPost
	// HttpPut corresponds to the HTTP PUT method.
	HttpPut = route.HttpPut
	// HttpPatch corresponds to the HTTP PATCH method.
	HttpPatch = route.HttpPatch
	// HttpDelete corresponds to the HTTP DELETE method.
	HttpDelete = route.HttpDelete
	// HttpHead corresponds to the HTTP HEAD method.
	HttpHead = route.HttpHead
	// HttpOptions corresponds to the HTTP OPTIONS method.
	HttpOptions = route.HttpOptions
	// HttpTrace corresponds to the HTTP TRACE method.
	HttpTrace = route.HttpTrace
	// HttpConnect corresponds to the HTTP CONNECT method.
	HttpConnect = route.HttpConnect
	// HttpQuery corresponds to the HTTP QUERY method.
	HttpQuery = route.HttpQuery
)

// HttpStatus* is the full standard HTTP status code enum, re-exporting every
// net/http.StatusXxx constant under an "HttpStatus" prefix (e.g.
// net/http.StatusNotFound -> gonest.HttpStatusNotFound) -- so route/response
// code can stay entirely within the gonest namespace instead of importing
// net/http alongside it just for a status code. Each value is a direct
// alias of its net/http counterpart, never a re-declared literal, so this
// list never drifts if net/http itself ever adds/changes one.
const (
	HttpStatusContinue           = http.StatusContinue
	HttpStatusSwitchingProtocols = http.StatusSwitchingProtocols
	HttpStatusProcessing         = http.StatusProcessing
	HttpStatusEarlyHints         = http.StatusEarlyHints

	HttpStatusOk                   = http.StatusOK
	HttpStatusCreated              = http.StatusCreated
	HttpStatusAccepted             = http.StatusAccepted
	HttpStatusNonAuthoritativeInfo = http.StatusNonAuthoritativeInfo
	HttpStatusNoContent            = http.StatusNoContent
	HttpStatusResetContent         = http.StatusResetContent
	HttpStatusPartialContent       = http.StatusPartialContent
	HttpStatusMultiStatus          = http.StatusMultiStatus
	HttpStatusAlreadyReported      = http.StatusAlreadyReported
	HttpStatusIMUsed               = http.StatusIMUsed

	HttpStatusMultipleChoices   = http.StatusMultipleChoices
	HttpStatusMovedPermanently  = http.StatusMovedPermanently
	HttpStatusFound             = http.StatusFound
	HttpStatusSeeOther          = http.StatusSeeOther
	HttpStatusNotModified       = http.StatusNotModified
	HttpStatusUseProxy          = http.StatusUseProxy
	HttpStatusTemporaryRedirect = http.StatusTemporaryRedirect
	HttpStatusPermanentRedirect = http.StatusPermanentRedirect

	HttpStatusBadRequest                   = http.StatusBadRequest
	HttpStatusUnauthorized                 = http.StatusUnauthorized
	HttpStatusPaymentRequired              = http.StatusPaymentRequired
	HttpStatusForbidden                    = http.StatusForbidden
	HttpStatusNotFound                     = http.StatusNotFound
	HttpStatusMethodNotAllowed             = http.StatusMethodNotAllowed
	HttpStatusNotAcceptable                = http.StatusNotAcceptable
	HttpStatusProxyAuthRequired            = http.StatusProxyAuthRequired
	HttpStatusRequestTimeout               = http.StatusRequestTimeout
	HttpStatusConflict                     = http.StatusConflict
	HttpStatusGone                         = http.StatusGone
	HttpStatusLengthRequired               = http.StatusLengthRequired
	HttpStatusPreconditionFailed           = http.StatusPreconditionFailed
	HttpStatusRequestEntityTooLarge        = http.StatusRequestEntityTooLarge
	HttpStatusRequestURITooLong            = http.StatusRequestURITooLong
	HttpStatusUnsupportedMediaType         = http.StatusUnsupportedMediaType
	HttpStatusRequestedRangeNotSatisfiable = http.StatusRequestedRangeNotSatisfiable
	HttpStatusExpectationFailed            = http.StatusExpectationFailed
	HttpStatusTeapot                       = http.StatusTeapot
	HttpStatusMisdirectedRequest           = http.StatusMisdirectedRequest
	HttpStatusUnprocessableEntity          = http.StatusUnprocessableEntity
	HttpStatusLocked                       = http.StatusLocked
	HttpStatusFailedDependency             = http.StatusFailedDependency
	HttpStatusTooEarly                     = http.StatusTooEarly
	HttpStatusUpgradeRequired              = http.StatusUpgradeRequired
	HttpStatusPreconditionRequired         = http.StatusPreconditionRequired
	HttpStatusTooManyRequests              = http.StatusTooManyRequests
	HttpStatusRequestHeaderFieldsTooLarge  = http.StatusRequestHeaderFieldsTooLarge
	HttpStatusUnavailableForLegalReasons   = http.StatusUnavailableForLegalReasons

	HttpStatusInternalServerError           = http.StatusInternalServerError
	HttpStatusNotImplemented                = http.StatusNotImplemented
	HttpStatusBadGateway                    = http.StatusBadGateway
	HttpStatusServiceUnavailable            = http.StatusServiceUnavailable
	HttpStatusGatewayTimeout                = http.StatusGatewayTimeout
	HttpStatusHTTPVersionNotSupported       = http.StatusHTTPVersionNotSupported
	HttpStatusVariantAlsoNegotiates         = http.StatusVariantAlsoNegotiates
	HttpStatusInsufficientStorage           = http.StatusInsufficientStorage
	HttpStatusLoopDetected                  = http.StatusLoopDetected
	HttpStatusNotExtended                   = http.StatusNotExtended
	HttpStatusNetworkAuthenticationRequired = http.StatusNetworkAuthenticationRequired
)

// Controller represents a declarative unit that consumes providers via
// MustInject. It does not participate in the provider resolution graph --
// it only consumes placeholders it requests.
type Controller = controller.Controller

// NewController creates a Controller that defers fn until bootstrap runs
// it.
var NewController = controller.New

// MustInject declares a dependency on type T from owner's builder fn --
// used inside a Provider's, Controller's, Middleware's, Guard's,
// Interceptor's, or Filter's deferred builder fn. owner is typed any (not
// module.Owner) because Middleware/Guard/Interceptor/Filter have no single
// owning Module (see internal/inject.MustInject's own doc comment) -- only
// Provider/Controller happen to also implement module.Owner.
//
// For an interface T, resolves to the one registered provider whose
// concrete type implements T (exact match or reflect.Type.Implements()),
// panicking on zero or 2+ matches (ambiguous -- use MustInjectAll). For a
// pointer T, resolves via exact type match, panicking if not found. Called
// from a Controller/Middleware/Guard/Interceptor/Filter builder fn, this is
// a DIRECT lookup against the already-resolved provider graph (no
// placeholder); called from a Provider's own builder fn (Provider-to-
// Provider dependency), T must be a pointer type and resolution is
// deferred via the existing placeholder+topological mechanism. Go cannot
// re-export a generic function via var, so this is a real wrapper calling
// the internal one.
func MustInject[T any](owner any) T {
	return inject.MustInject[T](owner)
}

// MustInjectAll returns every provider whose resolved concrete type
// satisfies interface T, as []T -- for a plugin/strategy pattern where the
// exact count is intentionally open-ended (see INSIGHT.md's "exemplo de
// MustInjectAll" section). T must be an interface kind (panics otherwise --
// multi-binding only makes sense for interfaces, see MustInject's single-
// match contract for the pointer case). owner must be a Controller,
// Middleware, Guard, Interceptor, or Filter (panics otherwise -- Provider-
// to-Provider dependencies stay single-value via MustInject). Returns an
// empty slice, never panics, if zero providers match.
func MustInjectAll[T any](owner any) []T {
	return inject.MustInjectAll[T](owner)
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

// FiberApp is v1's only real HttpAdapter implementation -- the type
// argument every INSIGHT.md example passes to NewApp[gonest.FiberApp]/
// MustNewApp[gonest.FiberApp]. Long-standing gap closed here (see STATE.md's
// Deferred Ideas): no feature before this had added the root re-export,
// even though it was already used as a literal call-site example
// throughout INSIGHT.md.
type FiberApp = fiber.FiberApp

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels). See internal/app.Options's doc comment for the
// full contract.
type AppOptions = app.Options

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
// the internal one (same pattern as MustInject, see AD-004 in
// STATE.md).
//
// T is passed BY VALUE at the call site (e.g. FiberApp, not *FiberApp) per
// INSIGHT.md's ground-truth call-site contract; the second type parameter
// enforcing "*T implements HttpAdapter" is inferred automatically -- see
// internal/app's httpAdapterPtr for why that extra parameter exists. See
// internal/app.NewApp's doc comment for the full bootstrap contract.
//
// opts is optional (at most one) and threaded straight through to
// internal/app.NewApp -- this wrapper does not interpret it.
func NewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts ...AppOptions) (*App, error) {
	return app.NewApp[T, PT](root, opts...)
}

// MustNewApp calls NewApp and panics if it returns an error.
func MustNewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts ...AppOptions) *App {
	return app.MustNewApp[T, PT](root, opts...)
}

// ---------------------------------------------------------------------------
// Testing (Test App Bootstrap feature)
// ---------------------------------------------------------------------------

// TestBuilder collects overrides for MustNewTestApp's provider substitution
// (see MustOverride). See internal/app.TestBuilder's own doc comment.
type TestBuilder = app.TestBuilder

// TestApp is MustNewTestApp's return value -- the same fully-bootstrapped
// module tree NewApp produces, minus a real bound network port. Usable
// directly with MustInject[T](tester) for unit-style tests (module-scoped,
// override-aware, same as a Controller's own MustInject). See
// internal/app.TestApp's own doc comment.
type TestApp = app.TestApp

// MustOverride registers mockValue as the substitute for any Provider whose
// resolved type exactly matches T (pointer override) or is implemented by
// T (interface override) -- that Provider's real Constructor never runs.
// Go cannot re-export a generic function via var, so this is a real
// wrapper calling the internal one (same pattern as MustInject/
// MustInjectAll, see AD-004 in STATE.md).
func MustOverride[T any](b *TestBuilder, mockValue T) {
	app.MustOverride(b, mockValue)
}

// MustNewTestApp runs the same 3-phase bootstrap NewApp does, with an
// optional configure callback to register MustOverride calls before
// provider resolution, and never binds a real network port (no Listen).
// See internal/app.MustNewTestApp's own doc comment for the full contract.
func MustNewTestApp(root *Module, configure func(*TestBuilder)) *TestApp {
	return app.MustNewTestApp(root, configure)
}

// TestResponse wraps the response TestApp.MustRequest dispatched, exposing
// AssertStatus/AssertJsonPath test-assertion helpers. See
// internal/app.TestResponse's own doc comment.
type TestResponse = app.TestResponse

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

// NewHttpException builds a zero-arg HttpException (status defaults to
// 500), returning a VALUE (not a pointer) for embedding into a struct
// literal via the chainable SetStatus/SetName/SetMessage/SetDetails
// methods, e.g.
// `HttpException: gonest.NewHttpException().SetStatus(http.StatusConflict).SetName("DuplicateEmailException")`.
// Unlike NewApp elsewhere in this package, NewHttpException is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed. See internal/exception.NewHttpException's doc
// comment for the full contract.
var NewHttpException = exception.NewHttpException

// ExceptionName returns exc.Name() if non-empty, otherwise a reflect-based
// default derived from exc's own concrete type (e.g. a dev-defined
// *FooError that never called SetName still gets "FooError" instead of "")
// -- used by the framework's own default (unhandled) exception formatting;
// exposed here so a dev-defined Filter can apply the same fallback. See
// internal/exception.EffectiveName's doc comment for why this can't live
// inside HttpException.Name()/MarshalJSON() itself.
var ExceptionName = exception.EffectiveName

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

// NewMiddleware creates a Middleware that defers fn until bootstrap runs it
// (phase 3, per the "Test App Bootstrap" feature's 3-phase reorder --
// AD-008 in STATE.md was REVERSED, this no longer runs fn immediately at
// package-init time; MustInject/MustInjectAll now work inside fn, scoped to
// the union of every referencing module). Like NewHttpException, New here
// is not generic, so Go allows aliasing the plain func directly via var --
// no wrapper function is needed (root package is the only public door
// since Go blocks external import of internal/*, per AD-004 in STATE.md).
// See internal/middleware.New's doc comment for the full contract.
var NewMiddleware = middleware.New

// NewLoggerMiddleware creates a ready-made Middleware that logs one line per
// request -- method, path, response status and duration -- once the rest of
// the chain (any inner Middleware, Guards, Interceptors, and the route
// Handler itself) has run, e.g.:
//
//	POST /auth/register 201 - 17ms
//
// Attach via Controller.Use/Module.Use like any other Middleware (e.g.
// `root.Use(gonest.NewLoggerMiddleware())`). Uses internal/logger.Info, so
// output is subject to AppOptions.LogLevels the same as gonest's own
// startup logging.
func NewLoggerMiddleware() *Middleware {
	return middleware.New(func(m *Middleware) {
		m.Handler(func(req *Request, res *Response, next Next) {
			start := time.Now()
			// A rejecting Guard/an exception.Exception thrown by the Handler
			// unwinds via panic straight past next(req, res) below, without
			// ever coming back here -- it is only turned into a status code
			// FURTHER UP the stack, by filteredHandler/the adapter's own
			// recover (internal/app/app.go, internal/adapter/fiber/fiber.go).
			// This defer/recover mirrors that mapping (exception.Exception's
			// own Status(), 500 for anything else) so the log line still
			// reports the real status the caller will see, then re-panics
			// unchanged so the existing Filter/default-formatting behavior
			// is untouched.
			r := recoverAfter(req, res, next)
			status := r.status
			duration := time.Since(start)

			logger.Info(fmt.Sprintf("%s %s %d - %dms", req.Method(), req.Path(), status, duration.Milliseconds()))

			if r.panicValue != nil {
				panic(r.panicValue)
			}
		})
	})
}

// loggerOutcome is NewLoggerMiddleware's own internal record of what
// happened after calling next(req, res): the status to log, and (if next
// panicked) the original panic value to re-panic once logging is done.
type loggerOutcome struct {
	status     int
	panicValue any
}

// recoverAfter calls next(req, res), catching any panic so
// NewLoggerMiddleware can log a line before re-propagating it unchanged.
func recoverAfter(req *Request, res *Response, next Next) (outcome loggerOutcome) {
	defer func() {
		if r := recover(); r != nil {
			outcome.panicValue = r
			if exc, ok := r.(exception.Exception); ok {
				outcome.status = exc.Status()
			} else {
				outcome.status = http.StatusInternalServerError
			}
		}
	}()
	next(req, res)
	outcome.status = res.StatusCode()
	return outcome
}

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

// NewGuard creates a Guard that defers fn until bootstrap runs it (phase 3
// -- AD-008 in STATE.md was REVERSED: a *Guard attached to multiple
// controllers across different modules now runs its own builder fn exactly
// once, with a search scope equal to the union of every referencing
// module, so MustInject/MustInjectAll work inside fn). Like NewMiddleware/
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

// NewInterceptor creates an Interceptor that defers fn until bootstrap
// runs it (phase 3 -- AD-008 in STATE.md was REVERSED, same as
// NewGuard/NewMiddleware/NewFilter: MustInject/MustInjectAll now work
// inside fn, union-scoped across every referencing module). Like
// NewGuard/NewMiddleware/NewHttpException, New here is not generic, so Go
// allows aliasing the plain func directly via var -- no wrapper function
// is needed (root package is the only public door since Go blocks
// external import of internal/*, per AD-004 in STATE.md). See
// internal/interceptor.New's doc comment for the full contract.
var NewInterceptor = interceptor.New

// InterceptorNext is interceptor.Next's own root alias -- a package-own
// type (NOT gonest.Next/middleware.Next, even though both share the
// identical underlying shape func(ctx *Context)) since internal/
// interceptor and internal/middleware are parallel, conceptually
// independent pipeline-stage packages. Long-standing gap closed here (see
// STATE.md's Deferred Ideas) -- needed to write an Interceptor.Handler
// callback using only the root gonest package, with no internal/
// interceptor import.
type InterceptorNext = interceptor.Next

// ---------------------------------------------------------------------------
// Filter (Filter feature)
// ---------------------------------------------------------------------------

// Filter represents a reusable set of per-exception-type response handlers,
// registered via Catch and keyed by the exact exception type, mirroring
// Nest's exception filters. It is attached via Controller.Filters/
// Module.Filters (e.g. INSIGHT.md's `controller.Filters(FooExampleFilter)`).
// See internal/filter.Filter's doc comment for the full contract.
type Filter = filter.Filter

// NewFilter creates a Filter that defers fn until bootstrap runs it (phase
// 3 -- AD-008 in STATE.md was REVERSED, same as NewGuard/NewMiddleware/
// NewInterceptor: MustInject/MustInjectAll now work inside fn, union-scoped
// across every referencing module). Like NewGuard/NewMiddleware/
// NewInterceptor/NewHttpException, New here is not generic, so Go allows
// aliasing the plain func directly via var -- no wrapper function is
// needed (root package is the only public door since Go blocks external
// import of internal/*, per AD-004 in STATE.md; see also AD-009 in
// STATE.md for why this section lives in this consolidated file rather
// than a separate filter.go). See internal/filter.New's doc comment for
// the full contract.
var NewFilter = filter.New

// ---------------------------------------------------------------------------
// Schema (Schema Registration Core feature)
// ---------------------------------------------------------------------------

// Schema holds the whole-type description plus every field registered via
// Property, for a single NewSchema[T] call (e.g. INSIGHT.md's
// `gonest.NewSchema[UserEntity](func(t *UserEntity, s *gonest.Schema) {
// ... })`). See internal/schema.Schema's doc comment for the full
// contract.
type Schema = schema.Schema

// PropertyBuilder holds one field's own constraints -- Required/Nullable/
// Description/Examples in this feature; future type+format branch features
// (String(), Integer(), etc. -- see ROADMAP.md, explicitly out of scope
// here) add their own methods on top. See internal/schema.PropertyBuilder's
// doc comment for the full contract.
type PropertyBuilder = schema.PropertyBuilder

// StringSchema is the branch-specific builder returned by all 10
// string-family branch methods on PropertyBuilder (String/Email/Uuid/Uri/
// Hostname/Ipv4/Ipv6/Password/Byte/Binary -- String-family Branches
// feature). It is a true Go type alias, so its own methods (Min/Max/
// Pattern) plus the manually re-declared chain methods (Required/Nullable/
// Description/Examples) are automatically visible on gonest.StringSchema
// with zero extra wrapper code, same as App/Module/Provider/etc above. See
// internal/schema.StringSchema's doc comment for why those 4 chain
// methods are manually re-declared rather than promoted (AD-009 in
// STATE.md: this section lives in this consolidated file rather than a
// separate schema.go).
type StringSchema = schema.StringSchema

// NewSchema builds a *Schema for T, identifying fields purely by their
// own pointer address within a zero value of T (INSIGHT.md's
// `m.Property(&t.Id)` call shape) -- no struct tags required. Go cannot
// re-export a generic function via var, so this is a real wrapper calling
// the internal one (same pattern as MustInject/NewApp, see AD-004
// in STATE.md; see also AD-009 in STATE.md for why this section lives in
// this consolidated file rather than a separate schema.go).
//
// Internally: a zero value of T is allocated, its address is passed to
// internal/schema.New as the base address every later Property(&t.Field)
// call's offset is measured against, then fn runs against that zero value
// and the freshly built *Schema (see internal/schema.Schema's doc
// comment for the full field-identification algorithm, empirically
// confirmed working by T1's own test suite).
func NewSchema[T any](fn func(t *T, m *Schema)) *Schema {
	var zero T
	m := schema.New(reflect.TypeOf(zero), uintptr(unsafe.Pointer(&zero)))
	fn(&zero, m)
	return m
}

// Value is the builder handed to NewValue's callback -- a true Go type
// alias of PropertyBuilder (schema-value-support feature), since a
// standalone value needs none of PropertyBuilder's struct-field bookkeeping,
// only its type+format branch methods (String()/Integer()/Boolean()/
// Array()/Object()) and Min/Max/Pattern/Custom, all already implemented and
// reused here with zero duplication.
type Value = schema.Value

// NewValue constructs a *Schema for a standalone value of type T -- no
// struct required, unlike NewSchema[T]. Go cannot re-export a generic
// function via var, so this is a real wrapper (same AD-004 precedent as
// NewSchema itself).
//
//	cpfSchema := gonest.NewValue[string](func(m *gonest.Value) {
//	    m.String().Min(11).Max(11).Pattern(`^\d{11}$`).Required()
//	})
//	cpf := gonest.MustParse[string](req.Body(), cpfSchema)
func NewValue[T any](fn func(m *Value)) *Schema {
	var zero T
	m, pb := schema.NewValue(reflect.TypeOf(zero))
	if fn != nil {
		fn(pb)
	}
	return m
}

// ---------------------------------------------------------------------------
// NumericSchema (Numeric & Boolean Branches feature)
// ---------------------------------------------------------------------------

// NumericSchema is the branch-specific builder returned by all 4
// numeric-family branch methods on PropertyBuilder (Integer/Int32/Float/
// Double -- Numeric & Boolean Branches feature). It is a true Go type alias,
// so its own methods (Min/Max) plus the manually re-declared chain methods
// (Required/Nullable/Description/Examples) are automatically visible on
// gonest.NumericSchema with zero extra wrapper code, same as
// gonest.StringSchema above. Boolean() needs no re-export of its own here
// -- it returns a plain *PropertyBuilder (already a root alias since
// Schema Registration Core), since OpenAPI's boolean type carries no
// format-specific extra validators the way the numeric/string families do.
// See internal/schema.NumericSchema's doc comment for why those 4 chain
// methods are manually re-declared rather than promoted (AD-009 in
// STATE.md: this section lives in this consolidated file rather than a
// separate schema.go).
type NumericSchema = schema.NumericSchema

// ---------------------------------------------------------------------------
// ArraySchema (Array Builder feature)
// ---------------------------------------------------------------------------

// ArraySchema is the dual-state branch builder returned by
// PropertyBuilder.Array (Array Builder feature). Unlike StringSchema/
// NumericSchema above, which describe a single field's own constraints,
// ArraySchema holds TWO separate states: the embedded *PropertyBuilder
// (the FIELD itself, e.g. `Tags []string` -- mutated by Required/Nullable/
// Description/Examples) and a synthetic item builder (the array's ELEMENTS
// -- mutated by the type+format branch methods String()/Integer()/.../
// Object() inside an Items(fn) callback), plus the array's own item-count
// bounds via Min/Max (distinct from the item's own Min/Max). It is a true Go
// type alias, so all of its methods are automatically visible on
// gonest.ArraySchema with zero extra wrapper code, same as
// gonest.StringSchema/gonest.NumericSchema above. See
// internal/schema.ArraySchema's doc comment for the full dual-state
// contract (AD-009 in STATE.md: this section lives in this consolidated
// file rather than a separate schema.go).
type ArraySchema = schema.ArraySchema

// ---------------------------------------------------------------------------
// ObjectSchema (Object Builder feature)
// ---------------------------------------------------------------------------

// ObjectSchema is the branch-specific builder returned by
// PropertyBuilder.Object (Object Builder feature). Unlike ArraySchema
// above, it is a SINGLE-STATE builder: it embeds the field's own
// *PropertyBuilder directly (the same shared object already held in
// Schema.properties[offset]), with no synthetic secondary builder --
// the field itself IS the nested object (e.g. `Address AddressEntity`), so
// there is no separate "element" to describe the way an array's items are.
// It is a true Go type alias, so its own methods (Schema/SchemaRef/
// AdditionalProperties/IsAdditionalProperties) plus the manually
// re-declared chain methods (Required/Nullable/Description/Examples) are
// automatically visible on gonest.ObjectSchema with zero extra wrapper
// code, same as gonest.ArraySchema/gonest.NumericSchema above. See
// internal/schema.ObjectSchema's doc comment for the full single-state
// contract (AD-009 in STATE.md: this section lives in this consolidated
// file rather than a separate schema.go).
type ObjectSchema = schema.ObjectSchema

// Request encapsulates the READ side of a single REST route Handler's
// request/response cycle -- request-response-split feature, replaces the
// single RestContext type (AD-021) with a (req, res) pair mirroring the
// Express/NestJS convention gonest's target devuser already knows (see
// context.md's "Motivação de Produto"). Because this is a true Go type
// alias, every method on execution.Request (Method/Path/Header/Param/
// Params/Query/Headers/Body/Route/WithRoute/WithSources/Queries/FormStream)
// is automatically visible on gonest.Request with zero extra wrapper code.
type Request = execution.Request

// Response encapsulates the WRITE side of a single REST route Handler's
// request/response cycle -- request-response-split feature. Because this is
// a true Go type alias, every method on execution.Response (Status/
// StatusCode/SetHeader/Json/Html/Text/Request) is automatically visible on
// gonest.Response with zero extra wrapper code.
type Response = execution.Response

// ---------------------------------------------------------------------------
// Accessor (dirty-tracking field wrapper)
// ---------------------------------------------------------------------------

// Accessor[T] is a generic field wrapper that tracks whether its value was
// explicitly set (dirty-tracking). It is the canonical way to model
// optional/partial updates in PATCH-style handlers: decode the JSON body
// into a struct whose fields are Accessor[T]; only fields that were present
// in the payload (including explicit null) are marked dirty -- omitted
// fields stay clean and can be safely ignored when writing changes to a
// database.
//
// JSON is transparent: MarshalJSON emits T's value directly with no extra
// wrapper in the wire format; UnmarshalJSON marks the field dirty whenever
// it appears in the payload.
//
// SPEC_DEVIATION (schema-value-support feature, T2): renamed from Value[T]
// -- "dirty" is a STATE of an accessor, not the type's own name (literature
// term for "something with get/set" is "accessor") -- freeing the name
// Value for the new standalone-schema construct (NewValue[T]/Value, below
// in the Schema section).
//
//	type UpdateUserDTO struct {
//	  Name  gonest.Accessor[string] `json:"name"`
//	  Email gonest.Accessor[string] `json:"email"`
//	}
//
//	// PATCH /users/:id -- only sent fields are applied
//	body := gonest.MustParse[*UpdateUserDTO](req, updateUserSchema)
//	body.Name.OnDirty(func(name string) { user.Name = name })
//	body.Email.Apply(&user.Email)
type Accessor[T any] = value.Accessor[T]

// NewAccessor creates an Accessor[T]. If an initial value is provided it
// starts dirty; the zero-arg form creates a clean (not-dirty) value.
//
// SPEC_DEVIATION (schema-value-support feature, T2): renamed from NewValue,
// same reason as Accessor's own doc comment.
//
//	name := gonest.NewAccessor("alice")  // dirty=true,  value="alice"
//	age  := gonest.NewAccessor[int]()    // dirty=false, value=0
func NewAccessor[T any](val ...T) Accessor[T] {
	return value.New(val...)
}

// AccessorsToDirtyMap converts a struct (or pointer to struct) whose fields
// are Accessor[T] into a map[string]any containing only the dirty fields,
// keyed by their json struct tag (or field name when the tag is absent or
// "-"). Non-Accessor fields are always ignored. Useful for building
// partial-update queries without reflection on the caller's side.
//
// SPEC_DEVIATION (schema-value-support feature, T2): renamed from
// ValueToDirtyMap, same reason as Accessor's own doc comment.
//
//	changes := gonest.AccessorsToDirtyMap(body)
//	db.Model(&user).Updates(changes)
func AccessorsToDirtyMap(obj any) map[string]any {
	return value.ToDirtyMap(obj)
}

// ---------------------------------------------------------------------------
// Validation (Unified Parse API feature)
// ---------------------------------------------------------------------------

// Parseable is the opaque value every req.Params()/req.Query()/req.Headers()/
// req.Body().Json()/req.Body().Form(onFile) call returns -- it carries its
// own parse logic alongside whatever state it needs to execute it. Devusers
// never implement Parseable directly; they only receive values of it from
// Request methods and pass them straight into Parse[T]/MustParse[T].
// See execution.Parseable's own doc comment for why its one method is
// exported despite being meant as a sealed contract.
type Parseable = execution.Parseable

// FormFile is one file part from a multipart/form-data stream, still
// un-consumed at the moment Body().Form(onFile)'s onFile callback runs --
// reading FormFile.Reader() consumes the underlying multipart stream in
// real time (Multipart Form Streaming feature, AD-022 in STATE.md). Not
// safe to retain past onFile's own call. See execution.FormFile's doc
// comment for the full contract.
type FormFile = execution.FormFile

// Parse reads and validates src (one of ctx.Params(), ctx.Query(),
// ctx.Headers(), ctx.Body().Json(), ctx.Body().Form(onFile)) against m (the
// *Schema built via NewSchema[T] for that same T -- passed explicitly,
// AD-019) and returns a populated T. Returns a non-nil *BadRequestException
// (collecting EVERY violation found, not just the first) as the error on
// any validation failure -- AD-021's non-panicking half of the family (see
// MustParse for the panicking twin). Still panics with a plain string if m
// was not built for T (a coding-error class of failure, not a
// request-validation one) -- src.ParseInto itself calls resolveSchema
// before touching anything else.
//
// Parse/MustParse contain NO source-specific logic: they declare `var zero
// T`, call src.ParseInto(&zero, m), and return -- every ctx method
// (unified-parse-api feature) hands back a Parseable that already knows how
// to read and validate its own source, so adding a new source later (e.g.
// ctx.Body().Xml()) never touches these two functions.
func Parse[T any](src Parseable, m *Schema) (T, error) {
	var zero T
	if err := src.ParseInto(&zero, m); err != nil {
		return zero, err
	}
	return zero, nil
}

// MustParse is Parse, panicking on any returned error instead of returning
// it -- AD-021's panicking twin, for call sites that don't want to handle
// the error themselves.
func MustParse[T any](src Parseable, m *Schema) T {
	v, err := Parse[T](src, m)
	if err != nil {
		panic(err)
	}
	return v
}

// ---------------------------------------------------------------------------
// OpenAPI (OpenAPI Document Builder feature)
// ---------------------------------------------------------------------------

// OpenAPI holds every document-level field set via NewOpenAPI
// (title, description, version, contact, license, bearer auth) --
// INSIGHT.md's own bootstrap example (`# exemplo de bootstrap completo`):
// `gonest.NewOpenAPI("3.1.0", func (b *gonest.OpenAPI) {
// ... })`. See internal/openapi.OpenAPI's doc comment for the full
// contract. Serving it (SetupSwagger/SwaggerOptions) and generating
// paths/schemas from registered routes/Schema are separate ROADMAP
// features (spec.md's Out of Scope).
type OpenAPI = openapi.OpenAPI

// NewOpenAPI builds a *OpenAPI, storing specVersion (the
// OpenAPI SPEC version, e.g. "3.1.0" -- distinct from the API's own
// Version(s)) and running fn against it before returning it. Unlike
// NewApp/NewSchema elsewhere in this package, New here is not generic, so
// Go allows aliasing the plain func directly via var -- no wrapper function
// is needed (same precedent as NewHttpException/NewMiddleware/NewGuard, see
// AD-004 in STATE.md). See internal/openapi.New's doc comment for the full
// contract.
var NewOpenAPI = openapi.New

// Route holds one HTTP method+path registration (see Controller.Route)
// plus the documentation builder methods a caller uses to describe it for
// schema generation (Summary/Description/OperationId/Tags/BearerAuth/
// RequestBody/Response/Params/Query/ExcludeFromDocs/Deprecated).
// It is a true Go type alias, so every one of those methods is automatically
// visible on gonest.Route with zero extra wrapper code, same as
// App/Module/Schema/etc above. See internal/route.Route's doc comment for
// the full contract.
type Route = route.Route

// RouteResponse is the per-status response builder passed to Route.Response's
// optional callback (`route.Response(201, func(response *gonest.RouteResponse) {
// response.Schema(userEntitySchema) })`) -- lets a route configure that
// status's documented body schema (Schema) and/or description
// (Description) in one place. It is a true Go type alias, so both methods
// are automatically visible on gonest.RouteResponse with zero extra wrapper
// code, same as Route/Schema/etc above. See internal/route.RouteResponse's
// doc comment for the full contract. Named RouteResponse (not plain
// Response) since request-response-split feature claimed gonest.Response
// for the write-side of the (req, res) HTTP pair.
type RouteResponse = route.RouteResponse

// GenerateOpenApiSchema walks app's assembled module tree (via app.Root(),
// recursing into every imported module -- see internal/app.App.Root's doc
// comment) and populates doc's paths/components.schemas from every
// registered Controller/Route's documentation (Controller.Tags/BearerAuth,
// Route.Summary/RequestBody/Response/Params/Query/
// ExcludeFromDocs/etc, plus every *Schema those routes reference,
// recursively deduplicated by pointer identity). Call it after NewApp (so
// the module tree is assembled) and before serving doc.Document() (e.g. via
// a future SetupSwagger -- out of scope here, see ROADMAP.md's "Swagger UI
// Setup" feature). Takes *App directly (not *Module) so callers never need
// to reach for app.Root() themselves -- this wrapper does that internally.
// Unlike NewApp/NewSchema elsewhere in this package, internal/openapi.Generate
// is not generic, but it takes *module.Module (an internal-only concept
// gonest.go otherwise never exposes as a caller-facing parameter), so this
// is a real wrapper rather than a plain var alias -- it accepts *App and
// forwards app.Root() to internal/openapi.Generate. See
// internal/openapi.Generate's doc comment for the full walking/dedup
// contract.
func GenerateOpenApiSchema(app *App, doc *OpenAPI) {
	openapi.Generate(doc, app.Root())
}

// SwaggerOptions is INSIGHT.md's exact field set for SetupSwagger's options
// argument (JsonDocumentUrl/PersistAuth/DocExpansion). It is a true Go type
// alias, so it round-trips through internal/openapi.SwaggerOptions with zero
// extra wrapper code, same as OpenAPI above. See
// internal/openapi.SwaggerOptions's doc comment for the full contract.
type SwaggerOptions = openapi.SwaggerOptions

// SetupSwagger registers two routes directly on app's adapter (post-
// bootstrap, no DI/Module involvement): a JSON endpoint serving
// doc.Document() at options.JsonDocumentUrl, and an HTML endpoint at uiPath
// serving a Swagger UI page (CDN-loaded, no vendored assets) configured from
// options -- INSIGHT.md's own bootstrap example
// (gonest.SetupSwagger(app, config.OpenApi.UiPath, doc, gonest.SwaggerOptions{...})).
// Unlike NewOpenAPI elsewhere in this package, internal/openapi.SetupSwagger
// takes *app.App (an internal-only concept gonest.go otherwise never exposes
// as a caller-facing parameter type), so this is a real wrapper rather than a
// plain var alias -- it forwards app straight through. See
// internal/openapi.SetupSwagger's doc comment for the full registration
// contract.
func SetupSwagger(app *App, uiPath string, doc *OpenAPI, options SwaggerOptions) error {
	return openapi.SetupSwagger(app, uiPath, doc, options)
}

// ---------------------------------------------------------------------------
// Emitter (Emitter & Listener feature, Milestone 9)
// ---------------------------------------------------------------------------

// Emitter is the framework's built-in event-emitter singleton: it is
// ALWAYS resolvable via MustInject[*Emitter](owner), from ANY module, with
// no explicit Provider registration anywhere -- see
// internal/emitter.Emitter's own doc comment.
type Emitter = emitter.Emitter

// Listener represents a single unit of event-handling registration, bound
// to EventType via its own type parameter -- a real Go generic type alias
// (requires Go 1.24+'s parameterized alias support, this module already
// targets 1.25).
type Listener[EventType any] = emitter.Listener[EventType]

// NewListener creates a Listener bound to EventType, deferring fn until
// bootstrap runs it. fn receives the Listener itself and is expected to
// call On or MustOn exactly once, registering the actual per-event
// handler -- dependencies are resolved ONCE, at bootstrap time, then
// closed over for every future event. On's handler returns an error
// (logged by Emit, same treatment a recovered panic gets); MustOn is the
// convenience form for a handler that never fails:
//
//	NewListener(func(l *Listener[UserCreatedEvent]) {
//	  logger := MustInject[*LoggerService](l)
//	  l.MustOn(func(ctx context.Context, event UserCreatedEvent) {
//	    logger.Log("user created", event.UserID)
//	  })
//	})
//
// Go cannot re-export a generic function via var, so this is a real wrapper
// calling the internal one (same pattern as MustInject/MustInjectAll, see
// AD-004 in STATE.md).
func NewListener[EventType any](fn func(*Listener[EventType])) *Listener[EventType] {
	return emitter.NewListener(fn)
}

// Subscribe registers a dynamic, per-connection channel that receives
// every future Emit(EventType{...}) call until done closes (graphql-
// support feature, Milestone 17) -- complementary to the static, app-
// lifetime NewListener/Emit pair. Go cannot re-export a generic function
// via var, so this is a real wrapper (same AD-004 precedent as NewListener).
func Subscribe[EventType any](e *Emitter, done <-chan struct{}) <-chan EventType {
	return emitter.Subscribe[EventType](e, done)
}

// ---------------------------------------------------------------------------
// GraphQL (graphql-support feature, Milestone 17)
// ---------------------------------------------------------------------------

// GraphqlResolver is a declarative unit analogous to Controller, exposing
// GraphQL Query/Mutation/Subscription fields instead of REST routes.
// Registered via Module.Resolvers, its builder fn is expected to call
// Query/Mutation/Subscription and MustInject for its dependencies -- see
// internal/graphql.Resolver's own doc comment.
type GraphqlResolver = graphql.Resolver

// NewGraphqlResolver creates a GraphqlResolver that defers fn until
// bootstrap runs it.
var NewGraphqlResolver = graphql.New

// GraphqlQuery represents one declared GraphQL query field -- name, args
// schema, return schema, handler, registered via
// GraphqlResolver.Query(name, fn).
type GraphqlQuery = graphql.Query

// GraphqlMutation represents one declared GraphQL mutation field --
// identical shape to GraphqlQuery (only the root type it ends up under in
// the generated SDL differs), registered via
// GraphqlResolver.Mutation(name, fn).
type GraphqlMutation = graphql.Mutation

// GraphqlSubscription represents one declared GraphQL subscription field
// -- a STREAM, not request-response, so its Handler signature differs
// from GraphqlQuery/GraphqlMutation's (func(ctx, emit) instead of
// func(ctx) any). Registered via GraphqlResolver.Subscription(name, fn).
type GraphqlSubscription = graphql.Subscription

// GraphqlContext is the single parameter passed to every
// GraphqlQuery/GraphqlMutation/GraphqlSubscription Handler -- ctx.Args()
// returns a Parseable consumable by Parse[T]/MustParse[T] exactly like
// req.Params()/req.Query()/req.Body().Json() already are for REST;
// ctx.Done() is a Subscription's own cancellation signal (nil for
// Query/Mutation).
type GraphqlContext = graphql.GraphqlContext

// ---------------------------------------------------------------------------
// Scheduler (Milestone 10)
// ---------------------------------------------------------------------------

// Scheduler represents a single unit of job registration -- its builder fn
// (deferred until bootstrap runs it, same New*-deferred pattern as
// Provider/Controller) is expected to call Cron/Interval/Timeout.
type Scheduler = scheduler.Scheduler

// NewScheduler creates a Scheduler that defers fn until bootstrap runs it.
var NewScheduler = scheduler.New

// ---------------------------------------------------------------------------
// Dotenv (dotenv-loading feature)
// ---------------------------------------------------------------------------

// Dotenv returns the package-level Dotenv singleton (e.g.
// gonest.Dotenv().MustLoad(".env")), returning *dotenv.Dotenv directly --
// no separate `type Dotenv = dotenv.Dotenv` alias is declared here, since
// Go forbids a type and a var sharing one identifier in the same scope, and
// the var (the actual gonest.Dotenv() call shape the feature asks for)
// takes priority; callers needing the type by name can still reach it as
// the return type of gonest.Dotenv() itself (e.g. `var d = gonest.Dotenv()`
// infers it with no import of internal/dotenv). Like NewHttpException/
// NewMiddleware/NewGuard elsewhere in this package, Get here is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed (root package is the only public door since
// Go blocks external import of internal/*, per AD-004 in STATE.md). See
// internal/dotenv.Get's doc comment for the full contract.
var Dotenv = dotenv.Get
