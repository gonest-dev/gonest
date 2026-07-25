# HttpContext Unification — Design

## Core types (internal/execution)

### Rename: `Response` → `Reply`

`internal/execution/response.go` → `internal/execution/reply.go`, `response_test.go` →
`reply_test.go`. `type Response struct{...}` → `type Reply struct{...}`, every `func (res
*Response) Xxx(...)` → `func (res *Reply) Xxx(...)` (receiver var `res` unchanged). `New(res
Responder) (*Request, *Response)` → `(*Request, *Reply)`.

### Rename: `route.RouteResponse` → `route.Response`

`internal/route/response.go` (already correctly named, no file rename). `type RouteResponse
struct` → `type Response struct`, methods `func (r *RouteResponse) Xxx(...) *RouteResponse` →
`func (r *Response) Xxx(...) *Response`. `Route.responses map[int]*RouteResponse` field →
`map[int]*Response`. `Route.Response(status, fn ...func(response *RouteResponse)) *Route` (method
NAME unchanged) → `func(response *Response)`. `Route.Responses() map[int]*RouteResponse` →
`map[int]*Response`.

### New: `HttpContext`

New file `internal/execution/httpcontext.go`:

```go
// HttpContext bundles the read side (Request) and write side (Reply) of one
// HTTP request/response cycle behind a single value -- the one parameter
// every Handler/Guard/Middleware/Interceptor/Filter.Catch receives. Exactly
// 2 methods, Request()/Response() -- every other read/write operation is
// reached through one of those two, never promoted directly onto
// HttpContext itself (deliberate: one way to reach any given piece of
// data, see this feature's design.md Tech Decisions).
type HttpContext struct {
    req *Request
    res *Reply
}

// NewHttpContext wraps req/res (already built via New(Responder)) into a
// single HttpContext. Exported so internal/app (Stage 2.5 dispatch wiring)
// can construct one per request without a same-package helper.
func NewHttpContext(req *Request, res *Reply) *HttpContext {
    return &HttpContext{req: req, res: res}
}

// Request returns this context's read side.
func (c *HttpContext) Request() *Request {
    return c.req
}

// Response returns this context's write side (type *Reply -- named
// Response to read naturally at the call site, e.g.
// c.Response().Status(200).Json(...), matching Route.Response's own
// method-name-vs-type-name precedent: the METHOD describes the role
// ("give me the response half"), the TYPE (Reply) is disambiguated from
// the unrelated OpenAPI documentation builder of the same conceptual
// area, route.Response).
func (c *HttpContext) Response() *Reply {
    return c.res
}
```

`execution.New(res Responder) (*Request, *Reply)` (renamed per above) stays the low-level
constructor; `NewHttpContext` is a thin wrapper one layer up. internal/app's dispatch wiring
(where `execution.New` is actually called, once per request) calls both in sequence:
`req, res := execution.New(responder); c := execution.NewHttpContext(req, res)`.

## Signature migration -- every consumer

All of these change from a 2-arg `(req *execution.Request, res *execution.Reply)` (post-rename)
shape to a single `(c *execution.HttpContext)`:

| File | Symbol(s) |
| --- | --- |
| `internal/route/route.go` | `Route.handler` field, `Route.Handler(fn func(c *execution.HttpContext))`, `HandlerFunc() func(*execution.HttpContext)` |
| `internal/guard/guard.go` | `Guard.handler`, `Guard.Handler(fn func(c *execution.HttpContext) bool)`, `HandlerFunc()` |
| `internal/middleware/middleware.go` | `Next func(c *execution.HttpContext)`, `Middleware.handler`, `Handler(fn func(c *execution.HttpContext, next Next))`, `HandlerFunc()` |
| `internal/interceptor/interceptor.go` | same shape as middleware (`Next`, `handler`, `Handler`, `HandlerFunc`) |
| `internal/filter/filter.go` | `Catch(exemplar, handler any)`'s accepted shape becomes `func(c *execution.HttpContext, exc T)` -- see "Filter.Catch reflect validation" below, a REAL logic change, not just a type-name swap |
| `internal/app/app.go` | `HttpAdapter.RegisterRoute(method, path, h func(c *execution.HttpContext)) error`; `registerRoutes`'s `collected.handler` field; `withRoute`, `filteredHandler`, `gatedHandler`, `interceptedHandler`, `composeHandler` -- ALL of these currently take/return `func(req *execution.Request, res *execution.Response)` and must become `func(c *execution.HttpContext)` (see "internal/app dispatch chain" below, the deepest single-file change) |
| `internal/app/graphql.go` | `graphqlPostDispatcher`/`graphqlGetDispatcher`/`graphqlHandler` return types |
| `internal/adapter/fiber/fiber.go` | `App.RegisterRoute` implementation -- builds `req, res := execution.New(&fiberResponder{c: c})` then must also build `ctx := execution.NewHttpContext(req, res)` and call `h(ctx)` instead of `h(req, res)` |
| `internal/openapi/swagger.go` | `SetupSwagger`'s 2 `RegisterRoute(...)` closures |
| `internal/graphql/sse_distinct.go`, `sse_single.go`, `ws_protocol.go` | every exported dispatcher func with this shape |

## Filter.Catch reflect validation (real logic change, not just renaming)

Today (`internal/filter/filter.go`): `requestType`/`responseType` package vars, `isValidCatchSignature`
checks `t.NumIn() == 3 && t.In(0) == requestType && t.In(1) == responseType && t.In(2) == excType`.

After: single `contextType = reflect.TypeOf((*execution.HttpContext)(nil))` var, replacing both
`requestType`/`responseType`. `isValidCatchSignature` checks `t.NumIn() == 2 && t.In(0) ==
contextType && t.In(1) == excType`. Panic message
(`"gonest: invalid Filter.Catch handler signature, expected func(req *execution.Request, res
*execution.Response, exc " + excType.String() + ")"`) becomes `"...expected func(c
*execution.HttpContext, exc " + excType.String() + ")"`.

Callsite in `internal/app/app.go`'s `filteredHandler` (the ONLY place that invokes a Catch handler
via reflect): `h.Call([]reflect.Value{reflect.ValueOf(req), reflect.ValueOf(res),
reflect.ValueOf(exc)})` (2 occurrences, controller-level + global lookup) becomes
`h.Call([]reflect.Value{reflect.ValueOf(c), reflect.ValueOf(exc)})`.

## internal/app dispatch chain (deepest single-file change)

`registerRoutes` currently builds a chain of `func(req *execution.Request, res
*execution.Response)` closures, composed innermost-out: route Handler → `interceptedHandler` →
`gatedHandler` → `composeHandler` → `withRoute` → `filteredHandler` (outermost, wraps everything).
Every one of these closures' signature becomes `func(c *execution.HttpContext)`; every internal
call from one layer to the next (`next(req, res)` → `next(c)`) follows the same shape. `withRoute`'s
body currently calls `req.WithRoute(currentRoute)`/`req.WithSources(...)` directly on the `req`
parameter -- becomes `c.Request().WithRoute(currentRoute)`/`c.Request().WithSources(...)`.

`collected.handler`'s field type and `adapter.RegisterRoute(rt.method, rt.path, rt.handler)`'s
3rd-arg type both follow `HttpAdapter.RegisterRoute`'s own signature change (see table above) --
no separate change needed there beyond the interface/struct field type itself updating.

## internal/adapter/fiber -- where HttpContext is actually constructed per request

`App.RegisterRoute`'s `wrapped := func(c fiber.Ctx) error {...}` closure (note: `c` here is
Fiber's OWN context type, an NAME COLLISION with the new `HttpContext`'s conventional variable name
`c` -- rename Fiber's local var to `fc` or similar to avoid confusion, since both will appear in
the same function body) currently does:

```go
req, res := execution.New(&fiberResponder{c: c})
...
h(req, res)
```

becomes:

```go
req, res := execution.New(&fiberResponder{c: fc}) // fc = renamed Fiber ctx param
ctx := execution.NewHttpContext(req, res)
...
h(ctx)
```

The panic-recovery `defer` inside this closure (writes `res.Status(...).Json(...)` on a recovered
`exception.Exception`) updates its `res` reference to `ctx.Response()` (or keep a local `res`
variable alongside `ctx` for its own convenience within this one function -- implementer's call,
both work, no public API impact either way since this is internal-only).

## gonest.go alias block (exact target shape)

```go
// Reply is the WRITE side of an HTTP request/response cycle -- Status/
// StatusCode/SetHeader/Json/Html/Text/Stream/UpgradeWebSocket, plus
// Request() to reach back to the Request that originated it. Reached via
// HttpContext.Response() in every Handler/Guard/Middleware/Interceptor/
// Filter.Catch -- named Reply (not Response) to avoid colliding with the
// OpenAPI documentation builder of the same conceptual area (see Response
// below). Same (request, reply) naming Fastify uses for this exact role,
// a real precedent in the Node ecosystem this framework targets, not a
// neologism.
type Reply = execution.Reply

// Response is the per-status documentation builder passed to
// Route.Response's callback (`route.Response(201, func(response
// *gonest.Response) { response.Schema(m) })`) -- lets a route describe
// that status's documented body schema/description for OpenAPI. Named
// Response (not RouteResponse) to match OpenAPI 3.x's own vocabulary
// ("responses" is the literal spec term for this) -- the write side of an
// actual HTTP request/response cycle uses Reply instead specifically so
// this name is free for its more natural fit here.
type Response = route.Response

// HttpContext is the single parameter every Handler/Guard/Middleware/
// Interceptor/Filter.Catch receives -- exactly 2 methods, Request()
// (read side) and Response() (write side, *Reply) -- everything else is
// reached through one of those two. Replaces the separate (req, res)
// 2-parameter shape request-response-split (AD-030) introduced; unlike
// that split, still keeps Request/Reply as real, separate, independently
// testable types underneath -- HttpContext is a thin wrapper, not a
// re-merge of their fields.
type HttpContext = execution.HttpContext
```

## Tech Decisions

| Decision | Rationale |
| --- | --- |
| `HttpContext` wraps `*Request`+`*Reply` via 2 getter methods, does not re-merge their fields into one struct | Keeps `Request`/`Reply` independently testable (existing test suites for each stay valid almost unchanged), and keeps the door open for a future protocol that only needs one side (unlikely for REST, but avoids coupling). Minimal-diff way to get the 1-parameter ergonomics without touching Request/Reply's own internals. |
| Only `Request()`/`Response()` promoted -- no flat passthrough methods (`c.Param(...)`, `c.Json(...)`) | User's explicit choice: one way to reach any given piece of data. Also sidesteps a real ambiguity a flat passthrough would introduce for `Status`-like concepts that could plausibly exist on either side. |
| `Response()` method name kept (not renamed to `Reply()`) even though it returns `*Reply` | Matches `Route.Response(status, fn)`'s own established precedent (method name ≠ param type name) and reads more naturally at the call site (`c.Response()` vs `c.Reply()` -- "give me the response" is the more natural phrasing even when the underlying type is called Reply for disambiguation reasons unrelated to what a caller conceptually wants). |
| `Filter.Catch`'s reflect-based validation updated in the SAME task as the rest of `internal/filter` (not deferred) | It is the one place where the signature change is genuine logic, not a pure rename -- deferring it would leave `Filter.Catch` silently accepting the OLD 3-arg shape (a real correctness bug, not just a stale doc comment) if done separately from the rest of the migration. |

## Testing Strategy

`go test ./... -race -count=1`, same gate as every prior AD. Every existing test file that
constructs `execution.New(...)`/asserts on `*execution.Response`/passes `(req, res)` to a
Handler/Guard/Middleware/Interceptor/Filter under test needs updating to the new shape --
mechanical but touches essentially every `_test.go` file in `internal/{execution,route,guard,
middleware,interceptor,filter,app,adapter/fiber,openapi,graphql}` plus root `gonest_test.go`. No
NEW test files required (pure signature/shape migration, no new behavior) beyond what the rename
already needs for `Filter.Catch`'s changed reflect validation (confirm the new 2-arg shape is
accepted and the old 3-arg shape now panics with the updated message -- likely already covered by
an existing "invalid signature" test that just needs its fixture updated, but confirm one exists).
`.examples/*` (all 5, including `full-text-search` which isn't in the pre-push hook's example
list) must all build clean; README.md's every code sample recompiled by hand (copy into a scratch
file, `go build`) before considering the feature done, same standard AD-030 itself used.
