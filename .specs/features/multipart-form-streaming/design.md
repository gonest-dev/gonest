# Multipart Form Streaming Design

**Spec**: `.specs/features/multipart-form-streaming/spec.md`
**Status**: Draft

---

## Architecture Overview

```mermaid
graph TD
    A[NewApp/MustNewApp opts AppOptions] --> B["adapter.Init(opts)<br/>(HttpAdapter.Init signature CHANGES)"]
    B --> C["FiberApp.Init: fiber.New(fiber.Config{<br/>StreamRequestBody: opts.EnableFormStreaming,<br/>DisablePreParseMultipartForm: opts.EnableFormStreaming})"]

    D[Route Handler] --> E["gonest.MustParseRestFormBody[T](ctx, schema, onFile)"]
    E --> F["validate.ParseFormBody[T]"]
    F --> G["ctx.res.BodyStream()<br/>(Responder, NEW method)"]
    G --> H["fiberResponder.BodyStream():<br/>c.RequestCtx().RequestBodyStream() + boundary"]
    F --> I["mime/multipart.NewReader(stream, boundary)"]
    I --> J{"NextPart()"}
    J -->|"filename == \"\" (form field)"| K["collect into presence map<br/>(same shape as MustParams/MustQuery)"]
    J -->|"filename != \"\" (file)"| L["onFile(*FormFile) -- SYNCHRONOUS,<br/>caller pipes file.Reader() to S3/etc NOW"]
    J -->|io.EOF| M["validateStruct-style walk over presence<br/>(Required/Custom(fn), form: tag)"]
    M --> N{violations?}
    N -->|yes| O["*BadRequestException (same shape as JSON/Params/Query)"]
    N -->|no| P["populate T, return (T, nil)"]
```

Two concerns interleave: this is why the design settles on a callback, not
an iterator (spec.md's confirmed decision) -- a single pass over
`multipart.Reader.NextPart()` handles BOTH form fields and files in
whatever order the client actually sent them, with files handled the
instant they're seen (true streaming) and fields validated once the walk
is fully done (`io.EOF`), reusing the exact same violation-collection shape
`ParseJsonBody`/`ParseParams`/`ParseQuery` already established.

---

## Code Reuse Analysis

### Existing Components to Leverage

| Component                                | Location                                | How to Use                                                                                                  |
| ----------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `resolveSchema`                          | `internal/validate/validate.go`         | Unchanged -- same schema-type-mismatch guard, called first, same as the other 3 `Parse*` functions.        |
| `violation`, `tagKeyVisible`              | `internal/validate/validate.go`         | Reused as-is -- `tagKeyVisible(p.Field(), "form")` is the ONLY new call shape (new tag name, same function). |
| `validateValue`                          | `internal/validate/validate.go`         | Reused unchanged for each collected form-field value (same as `ParseParams`/`ParseQuery`'s own use).       |
| `populate`                                | `internal/validate/populate.go` (or wherever it lives) | Reused unchanged, `tag="form"`, to build the final `T` from the collected presence map.                     |
| `coerceParamString`                       | `internal/validate/params.go`           | Reused unchanged -- form field raw strings coerce into the same any-shape as param/query string values.    |
| `exception.NewBadRequestException`        | `internal/exception/builtin.go`         | Same violation-collecting error shape, unchanged.                                                            |
| `AD-021`'s Parse/Must pair convention     | `gonest.go`                             | `ParseRestFormBody`/`MustParseRestFormBody` follow the exact same shape as the other 3 pairs.               |

### Integration Points

| System                          | Integration Method                                                                                                             |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `internal/execution.Responder`  | Gains ONE new method (`BodyStream`) -- every existing implementer (11 files, listed in Components below) needs the addition. |
| `internal/app.HttpAdapter`      | `Init()` signature changes to `Init(opts AppOptions)` -- the ONLY way `EnableFormStreaming` can reach the adapter's own `fiber.Config` at construction time (confirmed: `newAdapter[T,PT]()`/`Init()` take zero params today, and `AppOptions` is only ever seen by `NewApp` itself, never threaded further -- this is a REAL gap discovered during design, not previously visible in spec.md's Required Adjustments list). |
| `internal/adapter/fiber.FiberApp` | `Init` implements the new signature, builds `fiber.New(fiber.Config{...})` conditionally instead of `fiber.New()` bare.       |

---

## Components

### `internal/app.HttpAdapter` (MODIFIED interface)

- **Purpose**: Thread `AppOptions` into adapter construction, so `EnableFormStreaming` can reach `fiber.Config` before the underlying `*fiber.App` is built (config is immutable after `fiber.New()` returns).
- **Location**: `internal/app/app.go`
- **Interfaces**:
  - `Init(opts AppOptions)` -- CHANGED from `Init()`. Every implementer must accept `opts`, even if (like every adapter but Fiber, today) it ignores most of it.
- **Dependencies**: none new.
- **Reuses**: `AppOptions` (already exists, `internal/app/options.go`) -- just gains a new field (`EnableFormStreaming bool`), no new type.
- **Blast radius**: `newAdapter[T,PT]()` (app.go:406) changes to accept and forward `opts`; its ONE call site inside `NewApp` (app.go:350) passes the already-in-scope `opts` param. `internal/adapter/fiber.FiberApp.Init` is the only real (non-test) implementer today.

### `internal/adapter/fiber.FiberApp.Init` (MODIFIED)

- **Purpose**: Build the underlying `*fiber.App` with `StreamRequestBody`/`DisablePreParseMultipartForm` both set together when `opts.EnableFormStreaming` is true.
- **Location**: `internal/adapter/fiber/fiber.go`
- **Interfaces**:
  - `Init(opts app.AppOptions)` -- was `Init()`. New import: `internal/app` (confirmed no cycle -- `internal/app`'s own production code, `app.go`, does not import `internal/adapter/fiber` at all; only its OWN test file does, a different compilation unit).
- **Dependencies**: `internal/app` (new), `github.com/gofiber/fiber/v3` (already a dependency).
- **Reuses**: existing lazy-init-once guard (`if f.app == nil`), unchanged.

### `internal/adapter/fiber.fiberResponder.BodyStream` (NEW method)

- **Purpose**: Expose the raw request body as a stream + multipart boundary, when available.
- **Location**: `internal/adapter/fiber/fiber.go`
- **Interfaces**:
  - `BodyStream() (io.Reader, string, bool)` -- reader, boundary (from `Content-Type`'s `boundary=` param), `ok` (false when `EnableFormStreaming` wasn't turned on for this app, OR the request's `Content-Type` isn't `multipart/form-data` at all).
- **Dependencies**: `c.RequestCtx().RequestBodyStream()` (confirmed present on `*fasthttp.RequestCtx`, only non-nil when the SERVER was built with `StreamRequestBody: true` -- see Tech Decisions), `mime.ParseMediaType` (stdlib) to extract the boundary from `Content-Type`.
- **Reuses**: nothing new to build -- pure adapter-layer plumbing over already-vendored `fasthttp`.

### `internal/execution.Responder` (MODIFIED interface)

- **Purpose**: Let `Context`/`internal/validate` reach the raw stream without ever importing Fiber/fasthttp directly (same "HTTP-agnostic core" boundary `Body()`/`Queries()` already established).
- **Location**: `internal/execution/context.go`
- **Interfaces**:
  - `BodyStream() (io.Reader, string, bool)` -- added to the `Responder` interface. Every fake implementer across the test suite needs the addition (11 files confirmed via grep: `gonest_test.go`, `internal/execution/context_test.go`, `internal/filter/filter_test.go`, `internal/guard/guard_test.go`, `internal/interceptor/interceptor_test.go`, `internal/middleware/middleware_test.go`, `internal/route/route_test.go`, `internal/validate/{params,query,validate}_test.go` -- most can just `return nil, "", false`, matching Terminus's own `SendString` precedent exactly).
- **Reuses**: same one-line-delegation pattern as `Body()`/`Queries()`.

### `internal/execution.Context.FormStream` (NEW method)

- **Purpose**: Thin delegation, same shape as `Body()`/`Queries()`.
- **Location**: `internal/execution/context.go`
- **Interfaces**:
  - `FormStream() (io.Reader, string, bool)`
- **Reuses**: `ctx.res.BodyStream()`.

### `internal/validate.FormFile` (NEW type)

- **Purpose**: One file part from the multipart stream, still un-consumed at the moment `onFile` is called.
- **Location**: `internal/validate/form.go` (new file, alongside `params.go`/`query.go`)
- **Interfaces**:
  - `FieldName() string` -- the form field name (`Content-Disposition`'s `name`).
  - `Filename() string` -- the uploaded file's own name (`Content-Disposition`'s `filename`).
  - `ContentType() string` -- the part's own `Content-Type` header, if present.
  - `Reader() io.Reader` -- the LIVE part -- reading from it consumes the underlying multipart stream in real time. Not safe to retain past `onFile`'s own call (same "synchronous use only" precedent as `Context.Body()`'s own doc comment).
- **Dependencies**: wraps a `*multipart.Part` (stdlib `mime/multipart`) directly -- no new abstraction needed, `*multipart.Part` already implements `io.Reader`.
- **Reuses**: nothing -- genuinely new, small type.

### `internal/validate.ParseFormBody`/`MustFormBody` (NEW functions)

- **Purpose**: The real implementation behind `gonest.ParseRestFormBody`/`MustParseRestFormBody` -- walks the multipart stream once, dispatching each part to either the presence map (field) or `onFile` (file), then validates/populates `T` exactly like `ParseParams`/`ParseQuery` do.
- **Location**: `internal/validate/form.go`
- **Interfaces**:
  - `ParseFormBody[T any](ctx *execution.Context, m *schema.Schema, onFile func(*FormFile) error) (T, error)`
  - `MustFormBody[T any](ctx *execution.Context, m *schema.Schema, onFile func(*FormFile) error) T` -- thin panic wrapper, same shape as `MustParams`/`MustQuery`/`MustJsonBody`.
- **Dependencies**: `ctx.FormStream()`, `mime/multipart.NewReader`, `resolveSchema`, `tagKeyVisible(..., "form")`, `coerceParamString`, `validateValue`, `populate(..., "form")`.
- **Reuses**: everything listed in Code Reuse Analysis above -- this function is almost entirely composition of EXISTING internal machinery, plus the multipart walk itself (genuinely new logic, but small: a loop calling `mr.NextPart()` until `io.EOF`).

### `gonest.ParseRestFormBody`/`MustParseRestFormBody`/`FormFile` (NEW root exports)

- **Purpose**: Public API, matching AD-021's Parse/Must pair shape.
- **Location**: `gonest.go`
- **Interfaces**:
  - `func ParseRestFormBody[T any](ctx *RestContext, m *Schema, onFile func(*FormFile) error) (T, error)`
  - `func MustParseRestFormBody[T any](ctx *RestContext, m *Schema, onFile func(*FormFile) error) T`
  - `type FormFile = validate.FormFile`
- **Reuses**: thin wrappers calling `validate.ParseFormBody`/`validate.MustFormBody`, exactly like every other `ParseRestXxx`/`MustParseRestXxx` pair.

### `gonest.AppOptions.EnableFormStreaming` (NEW field)

- **Purpose**: The single, friendly, gonest-level opt-in toggle.
- **Location**: `internal/app/options.go` (re-exported at root via the existing `AppOptions = app.AppOptions` alias, unchanged)
- **Interfaces**: `EnableFormStreaming bool` (default `false`)
- **Reuses**: nothing -- a field addition to an existing struct.

---

## Data Models

### `FormFile` (conceptual shape, see Components above for the real interface)

```go
type FormFile struct {
    part *multipart.Part // unexported, wraps the stdlib type directly
}
```

**Relationships**: constructed internally by `ParseFormBody` for each
file-part encountered, handed to `onFile` by value (a `*FormFile`), never
retained/constructed anywhere else.

---

## Error Handling Strategy

| Error Scenario                                                | Handling                                                                                          | User Impact                                                                    |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `EnableFormStreaming` not set, `ParseRestFormBody` called anyway | `ctx.FormStream()` returns `ok=false` -- `ParseFormBody` returns a plain-string error identifying the missing `AppOptions` field (mirrors `resolveSchema`'s own plain-string-panic precedent) | Clear, actionable message naming the exact `AppOptions` field to flip, not a generic failure. |
| Request `Content-Type` isn't `multipart/form-data`             | `ctx.FormStream()` returns `ok=false` the same way (no boundary to parse)                          | Same as above -- one unified "form stream unavailable" failure mode, 2 different causes. |
| A form field fails validation (Required/Min/Max/Custom(fn))    | Collected into `violations`, same as `ParseParams`/`ParseQuery` -- returned as `*BadRequestException` after the FULL walk completes | Same JSON shape (`{name, message, details}`) every other validation failure already produces. |
| `onFile` returns a non-nil error                                | Walk stops immediately; wrapped into a violation (field name = form field name, message = callback's error) and returned as `*BadRequestException` | Same shape, attributable to the specific file field.                           |
| `mime/multipart.Reader.NextPart()` itself errors (malformed multipart stream) | Treated as ONE violation (field `""`), same "can't collect per-field violations if the container itself is broken" precedent `ParseJsonBody`'s malformed-JSON case already established | Same shape as `ParseJsonBody`'s malformed-JSON case. |

---

## Tech Decisions (only non-obvious ones)

| Decision                                                                 | Choice                                                                                                     | Rationale                                                                                                                                                                                                                     |
| --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| File delivery mechanism                                                 | Synchronous per-file callback (`onFile`), not a manual iterator                                            | Confirmed with user directly (see conversation) -- callback is the only shape that survives arbitrary field/file interleaving in the raw multipart stream without buffering. |
| `HttpAdapter.Init()` signature                                          | Changes to `Init(opts AppOptions)`                                                                          | The ONLY way `EnableFormStreaming` reaches `fiber.Config` before `fiber.New()` is called -- `fiber.Config` is immutable post-construction, and `Init()`/`newAdapter[T,PT]()` take zero params today (confirmed via direct source read, not assumed). This is a genuinely new finding from this design pass, not visible when spec.md was written. |
| `StreamRequestBody` + `DisablePreParseMultipartForm` must BOTH be set    | Both flip together under ONE `EnableFormStreaming` toggle, never exposed as 2 separate `AppOptions` fields | Confirmed via `fasthttp@v1.72.0/server.go`/`http.go` source: multipart pre-parsing (which defeats streaming) stays ON by default even with `StreamRequestBody` alone (`!s.DisablePreParseMultipartForm` gates it) -- exposing them separately would be an easy-to-misconfigure footgun for zero benefit (no real use case wants one without the other). |
| New struct tag: `form:"..."`                                            | Distinct tag, not reusing `json:"..."`                                                                    | Matches the existing "one tag family per source" precedent (`param`/`query`/`json`) -- considered and set aside the "everything is `json:` tag" idea during the earlier `MustParse`-consolidation brainstorm (see STATE.md). |
| `Responder.BodyStream()` return shape: `(io.Reader, string, bool)`      | 3-value return (reader, boundary, ok) instead of a single `(io.Reader, error)`                             | `ok=false` cleanly covers 2 DIFFERENT non-error conditions (streaming not enabled at all vs. request isn't multipart) without inventing a sentinel error value to distinguish them -- matches `schema.Lookup`'s own `(value, bool)` convention already used elsewhere in this codebase (never `(value, error)` for a "not applicable" case). |
| Testing strategy                                                        | Real `httptest` dispatch only, no lightweight fake-responder unit style for the streaming-proof tests      | A fake in-memory `Responder` cannot meaningfully prove "bytes were available before the rest of the body arrived" -- that specific claim needs a REAL stream (confirmed already in spec.md's Independent Test). |

---

## Open Question Surfaced During Design (needs explicit sign-off before Tasks)

`HttpAdapter.Init()`'s signature change was NOT in spec.md's original
"Required Adjustments" list (spec.md only anticipated `AppOptions` gaining
a field and `internal/app.NewApp` "translating" it -- it did not yet know
`Init()` itself needed to change shape to carry that translation through).
This is the single biggest surprise this design pass found: it's a real,
if small, breaking change to `internal/app`'s own adapter contract
(`HttpAdapter` interface), not just additive API. Confirmed safe (no import
cycle, single call site) via direct source reading above -- flagging
prominently per the user's own "deixe bem óbvio o que for necessário a ser
ajustado" instruction, not because it's risky, but because it's exactly the
kind of adjustment that request asked to be surfaced loudly.
