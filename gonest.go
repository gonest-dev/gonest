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
	"context"
	"net/http"
	"reflect"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/app"
	"github.com/gonest-dev/gonest/internal/controller"
	"github.com/gonest-dev/gonest/internal/emitter"
	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/filter"
	"github.com/gonest-dev/gonest/internal/guard"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/metadata"
	"github.com/gonest-dev/gonest/internal/middleware"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/openapi"
	"github.com/gonest-dev/gonest/internal/provider"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/scope"
	"github.com/gonest-dev/gonest/internal/validate"
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
	// HttpDelete corresponds to the HTTP DELETE method.
	HttpDelete = route.HttpDelete
	// HttpQuery corresponds to the HTTP QUERY method, used for list/query
	// endpoints (see INSIGHT.md).
	HttpQuery = route.HttpQuery
)

// HttpStatusOk/HttpStatusServiceUnavailable are the 2 raw HTTP status codes
// INSIGHT.md's own examples reference directly (e.g. "exemplo de Testing"'s
// res.AssertStatus(t, gonest.HttpStatusOk), "exemplo de Probes / health"'s
// readiness probe). Deliberately NOT a full status-code enum -- only these
// 2 are demonstrated anywhere; a fuller enum is a separate, later
// housekeeping task if a real need appears (same "don't invent unused API
// surface" stance as every prior feature this session).
const (
	// HttpStatusOk corresponds to net/http.StatusOK (200).
	HttpStatusOk = http.StatusOK
	// HttpStatusServiceUnavailable corresponds to net/http.StatusServiceUnavailable (503).
	HttpStatusServiceUnavailable = http.StatusServiceUnavailable
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
// the internal one (same pattern as MustInject, see AD-004 in
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
	app.MustOverride[T](b, mockValue)
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

// NewHttpException builds an HttpException from its four parts, returning a
// VALUE (not a pointer) for embedding into a struct literal, e.g.
// `HttpException: gonest.NewHttpException(status, name, message, details)`.
// Unlike NewApp elsewhere in this package, NewHttpException is not
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

// ---------------------------------------------------------------------------
// Filter (Filter feature)
// ---------------------------------------------------------------------------

// Filter represents a reusable set of per-exception-type response handlers,
// registered via Catch and keyed by the exact exception type, mirroring
// Nest's exception filters. It is attached via Controller.Filters/
// Module.Filters (e.g. INSIGHT.md's `controller.Filters(FooExampleFilter)`).
// See internal/filter.Filter's doc comment for the full contract.
type Filter = filter.Filter

// NewFilter creates a Filter and runs fn on it immediately (unlike
// Provider/Module/Controller/Pipe, which defer fn until bootstrap). This
// feature deliberately has no MustInject support (AD-008 in STATE.md:
// pipeline-stage types don't support MustInject in v1, same reasoning as
// NewGuard/NewMiddleware/NewInterceptor), so there is no bootstrap stage
// left to usefully defer fn to. Like NewGuard/NewMiddleware/NewInterceptor/
// NewHttpException, New here is not generic, so Go allows aliasing the plain
// func directly via var -- no wrapper function is needed (root package is
// the only public door since Go blocks external import of internal/*, per
// AD-004 in STATE.md; see also AD-009 in STATE.md for why this section lives
// in this consolidated file rather than a separate filter.go). See
// internal/filter.New's doc comment for the full contract.
var NewFilter = filter.New

// ---------------------------------------------------------------------------
// Metadata (Metadata Registration Core feature)
// ---------------------------------------------------------------------------

// Metadata holds the whole-type description plus every field registered via
// Property, for a single NewMetadata[T] call (e.g. INSIGHT.md's
// `gonest.NewMetadata[UserEntity](func(t *UserEntity, m *gonest.Metadata) {
// ... })`). See internal/metadata.Metadata's doc comment for the full
// contract.
type Metadata = metadata.Metadata

// PropertyBuilder holds one field's own constraints -- Required/Nullable/
// Description/Examples in this feature; future type+format branch features
// (String(), Integer(), etc. -- see ROADMAP.md, explicitly out of scope
// here) add their own methods on top. See internal/metadata.PropertyBuilder's
// doc comment for the full contract.
type PropertyBuilder = metadata.PropertyBuilder

// StringMetadata is the branch-specific builder returned by all 10
// string-family branch methods on PropertyBuilder (String/Email/Uuid/Uri/
// Hostname/Ipv4/Ipv6/Password/Byte/Binary -- String-family Branches
// feature). It is a true Go type alias, so its own methods (Min/Max/
// Pattern) plus the manually re-declared chain methods (Required/Nullable/
// Description/Examples) are automatically visible on gonest.StringMetadata
// with zero extra wrapper code, same as App/Module/Provider/etc above. See
// internal/metadata.StringMetadata's doc comment for why those 4 chain
// methods are manually re-declared rather than promoted (AD-009 in
// STATE.md: this section lives in this consolidated file rather than a
// separate metadata.go).
type StringMetadata = metadata.StringMetadata

// NewMetadata builds a *Metadata for T, identifying fields purely by their
// own pointer address within a zero value of T (INSIGHT.md's
// `m.Property(&t.Id)` call shape) -- no struct tags required. Go cannot
// re-export a generic function via var, so this is a real wrapper calling
// the internal one (same pattern as MustInject/NewApp, see AD-004
// in STATE.md; see also AD-009 in STATE.md for why this section lives in
// this consolidated file rather than a separate metadata.go).
//
// Internally: a zero value of T is allocated, its address is passed to
// internal/metadata.New as the base address every later Property(&t.Field)
// call's offset is measured against, then fn runs against that zero value
// and the freshly built *Metadata (see internal/metadata.Metadata's doc
// comment for the full field-identification algorithm, empirically
// confirmed working by T1's own test suite).
func NewMetadata[T any](fn func(t *T, m *Metadata)) *Metadata {
	var zero T
	m := metadata.New(reflect.TypeOf(zero), uintptr(unsafe.Pointer(&zero)))
	fn(&zero, m)
	return m
}

// ---------------------------------------------------------------------------
// NumericMetadata (Numeric & Boolean Branches feature)
// ---------------------------------------------------------------------------

// NumericMetadata is the branch-specific builder returned by all 4
// numeric-family branch methods on PropertyBuilder (Integer/Int32/Float/
// Double -- Numeric & Boolean Branches feature). It is a true Go type alias,
// so its own methods (Min/Max) plus the manually re-declared chain methods
// (Required/Nullable/Description/Examples) are automatically visible on
// gonest.NumericMetadata with zero extra wrapper code, same as
// gonest.StringMetadata above. Boolean() needs no re-export of its own here
// -- it returns a plain *PropertyBuilder (already a root alias since
// Metadata Registration Core), since OpenAPI's boolean type carries no
// format-specific extra validators the way the numeric/string families do.
// See internal/metadata.NumericMetadata's doc comment for why those 4 chain
// methods are manually re-declared rather than promoted (AD-009 in
// STATE.md: this section lives in this consolidated file rather than a
// separate metadata.go).
type NumericMetadata = metadata.NumericMetadata

// ---------------------------------------------------------------------------
// ArrayMetadata (Array Builder feature)
// ---------------------------------------------------------------------------

// ArrayMetadata is the dual-state branch builder returned by
// PropertyBuilder.Array (Array Builder feature). Unlike StringMetadata/
// NumericMetadata above, which describe a single field's own constraints,
// ArrayMetadata holds TWO separate states: the embedded *PropertyBuilder
// (the FIELD itself, e.g. `Tags []string` -- mutated by Required/Nullable/
// Description/Examples) and a synthetic item builder (the array's ELEMENTS
// -- mutated by the type+format branch methods String()/Integer()/.../
// Object() inside an Items(fn) callback), plus the array's own item-count
// bounds via Min/Max (distinct from the item's own Min/Max). It is a true Go
// type alias, so all of its methods are automatically visible on
// gonest.ArrayMetadata with zero extra wrapper code, same as
// gonest.StringMetadata/gonest.NumericMetadata above. See
// internal/metadata.ArrayMetadata's doc comment for the full dual-state
// contract (AD-009 in STATE.md: this section lives in this consolidated
// file rather than a separate metadata.go).
type ArrayMetadata = metadata.ArrayMetadata

// ---------------------------------------------------------------------------
// ObjectMetadata (Object Builder feature)
// ---------------------------------------------------------------------------

// ObjectMetadata is the branch-specific builder returned by
// PropertyBuilder.Object (Object Builder feature). Unlike ArrayMetadata
// above, it is a SINGLE-STATE builder: it embeds the field's own
// *PropertyBuilder directly (the same shared object already held in
// Metadata.properties[offset]), with no synthetic secondary builder --
// the field itself IS the nested object (e.g. `Address AddressEntity`), so
// there is no separate "element" to describe the way an array's items are.
// It is a true Go type alias, so its own methods (Metadata/MetadataRef/
// AdditionalProperties/IsAdditionalProperties) plus the manually
// re-declared chain methods (Required/Nullable/Description/Examples) are
// automatically visible on gonest.ObjectMetadata with zero extra wrapper
// code, same as gonest.ArrayMetadata/gonest.NumericMetadata above. See
// internal/metadata.ObjectMetadata's doc comment for the full single-state
// contract (AD-009 in STATE.md: this section lives in this consolidated
// file rather than a separate metadata.go).
type ObjectMetadata = metadata.ObjectMetadata

// ---------------------------------------------------------------------------
// Validation (JSON Body Validation feature)
// ---------------------------------------------------------------------------

// MustJsonBody reads ctx's raw request body, validates it against T's
// (dereferenced) registered *Metadata (built via NewMetadata[T] beforehand,
// e.g. INSIGHT.md's UserEntity example), and returns a populated T. It
// panics *BadRequestException (collecting EVERY violation found, not just
// the first) if the body fails to parse as JSON or any field/array-item/
// nested-object constraint is violated, and panics with a plain string if T
// was never registered via NewMetadata[T]. Go cannot re-export a generic
// function via var, so this is a real wrapper calling the internal one
// (same pattern as MustInject/NewApp, see AD-004 in STATE.md). ctx is
// *execution.Context directly -- no root Context alias exists yet (a
// pre-existing gap from an earlier feature).
func MustJsonBody[T any](ctx *execution.Context) T {
	return validate.MustJsonBody[T](ctx)
}

// MustParams reads ctx's current route's path params, validates every field
// of T (dereferenced) against its registered *Metadata (built via
// NewMetadata[T] beforehand, matched via a `param:"name"` struct tag) and
// returns a populated T. It panics *BadRequestException (collecting EVERY
// violation found, not just the first) if a required param is missing or
// any param fails its declared constraint, and panics with a plain string
// if T was never registered via NewMetadata[T]. Go cannot re-export a
// generic function via var, so this is a real wrapper calling the internal
// one (same pattern as MustJsonBody, see AD-004 in STATE.md).
func MustParams[T any](ctx *execution.Context) T {
	return validate.MustParams[T](ctx)
}

// MustQuery reads ctx's request query string, validates every field of T
// (dereferenced) against its registered *Metadata (built via NewMetadata[T]
// beforehand, matched via a `query:"name"` struct tag) and returns a
// populated T. It panics *BadRequestException (collecting EVERY violation
// found, not just the first) if a required query param is missing or any
// query param fails its declared constraint, and panics with a plain
// string if T was never registered via NewMetadata[T]. Go cannot re-export
// a generic function via var, so this is a real wrapper calling the
// internal one (same pattern as MustJsonBody/MustParams, see AD-004 in
// STATE.md).
func MustQuery[T any](ctx *execution.Context) T {
	return validate.MustQuery[T](ctx)
}

// ---------------------------------------------------------------------------
// OpenAPI (OpenAPI Document Builder feature)
// ---------------------------------------------------------------------------

// OpenApiDocument holds every document-level field set via NewOpenApiDocument
// (title, description, version, contact, license, bearer auth) --
// INSIGHT.md's own bootstrap example (`# exemplo de bootstrap completo`):
// `gonest.NewOpenApiDocument("3.1.0", func (b *gonest.OpenApiDocument) {
// ... })`. See internal/openapi.OpenApiDocument's doc comment for the full
// contract. Serving it (SetupSwagger/SwaggerOptions) and generating
// paths/schemas from registered routes/Metadata are separate ROADMAP
// features (spec.md's Out of Scope).
type OpenApiDocument = openapi.OpenApiDocument

// NewOpenApiDocument builds a *OpenApiDocument, storing specVersion (the
// OpenAPI SPEC version, e.g. "3.1.0" -- distinct from the API's own
// Version(s)) and running fn against it before returning it. Unlike
// NewApp/NewMetadata elsewhere in this package, New here is not generic, so
// Go allows aliasing the plain func directly via var -- no wrapper function
// is needed (same precedent as NewHttpException/NewMiddleware/NewGuard, see
// AD-004 in STATE.md). See internal/openapi.New's doc comment for the full
// contract.
var NewOpenApiDocument = openapi.New

// Route holds one HTTP method+path registration (see Controller.Route)
// plus the documentation builder methods a caller uses to describe it for
// schema generation (Summary/Description/OperationId/Tags/BearerAuth/
// RequestBody/Response/PathParams/QueryParams/ExcludeFromDocs/Deprecated).
// It is a true Go type alias, so every one of those methods is automatically
// visible on gonest.Route with zero extra wrapper code, same as
// App/Module/Metadata/etc above. See internal/route.Route's doc comment for
// the full contract.
type Route = route.Route

// GenerateOpenApiSchema walks app's assembled module tree (via app.Root(),
// recursing into every imported module -- see internal/app.App.Root's doc
// comment) and populates doc's paths/components.schemas from every
// registered Controller/Route's documentation (Controller.Tags/BearerAuth,
// Route.Summary/RequestBody/Response/PathParams/QueryParams/
// ExcludeFromDocs/etc, plus every *Metadata those routes reference,
// recursively deduplicated by pointer identity). Call it after NewApp (so
// the module tree is assembled) and before serving doc.Document() (e.g. via
// a future SetupSwagger -- out of scope here, see ROADMAP.md's "Swagger UI
// Setup" feature). Takes *App directly (not *Module) so callers never need
// to reach for app.Root() themselves -- this wrapper does that internally.
// Unlike NewApp/NewMetadata elsewhere in this package, internal/openapi.Generate
// is not generic, but it takes *module.Module (an internal-only concept
// gonest.go otherwise never exposes as a caller-facing parameter), so this
// is a real wrapper rather than a plain var alias -- it accepts *App and
// forwards app.Root() to internal/openapi.Generate. See
// internal/openapi.Generate's doc comment for the full walking/dedup
// contract.
func GenerateOpenApiSchema(app *App, doc *OpenApiDocument) {
	openapi.Generate(doc, app.Root())
}

// SwaggerOptions is INSIGHT.md's exact field set for SetupSwagger's options
// argument (JsonDocumentUrl/PersistAuth/DocExpansion). It is a true Go type
// alias, so it round-trips through internal/openapi.SwaggerOptions with zero
// extra wrapper code, same as OpenApiDocument above. See
// internal/openapi.SwaggerOptions's doc comment for the full contract.
type SwaggerOptions = openapi.SwaggerOptions

// SetupSwagger registers two routes directly on app's adapter (post-
// bootstrap, no DI/Module involvement): a JSON endpoint serving
// doc.Document() at options.JsonDocumentUrl, and an HTML endpoint at uiPath
// serving a Swagger UI page (CDN-loaded, no vendored assets) configured from
// options -- INSIGHT.md's own bootstrap example
// (gonest.SetupSwagger(app, config.OpenApi.UiPath, doc, gonest.SwaggerOptions{...})).
// Unlike NewOpenApiDocument elsewhere in this package, internal/openapi.SetupSwagger
// takes *app.App (an internal-only concept gonest.go otherwise never exposes
// as a caller-facing parameter type), so this is a real wrapper rather than a
// plain var alias -- it forwards app straight through. See
// internal/openapi.SetupSwagger's doc comment for the full registration
// contract.
func SetupSwagger(app *App, uiPath string, doc *OpenApiDocument, options SwaggerOptions) error {
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

// Listener represents a single unit of event-handling registration --
// its builder fn (deferred until bootstrap runs it, same New*-deferred
// pattern as Provider/Controller) is expected to call MustOn.
type Listener = emitter.Listener

// NewListener creates a Listener that defers fn until bootstrap runs it.
var NewListener = emitter.NewListener

// MustOn registers handler as the callback for events of type EventType on
// the current bootstrap's Emitter singleton. Go cannot re-export a generic
// function via var, so this is a real wrapper calling the internal one
// (same pattern as MustInject/MustInjectAll, see AD-004 in STATE.md).
func MustOn[EventType any](listener *Listener, handler func(ctx context.Context, event EventType)) {
	emitter.MustOn[EventType](listener, handler)
}
