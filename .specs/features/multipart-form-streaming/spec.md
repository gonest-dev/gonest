# Multipart Form Streaming Specification

## Problem Statement

gonest has no multipart/form-data support at all today -- only JSON body
(`ParseRestJsonBody`/`MustParseRestJsonBody`, AD-021), path params, and query
string. There is no way to accept a file upload. Most frameworks (NestJS's
own `multer`/`FileInterceptor` included) that DO support file upload buffer
the entire file to memory or a local temp file before handler code ever
runs, forcing an extra disk/memory round-trip before the file can be
forwarded to real storage (S3, etc). Confirmed via direct inspection of the
vendored `gofiber/fiber/v3@v3.4.0` and `valyala/fasthttp@v1.72.0` source
(no assumption, no fabrication -- see Design Constraints below) that TRUE
streaming (handler starts forwarding a file's bytes to storage as they
arrive, never buffering the whole file locally) is achievable with the
dependencies already in `go.mod`, no new dependency needed.

## Goals

- [x] `gonest.ParseRestFormBody[T]`/`MustParseRestFormBody[T]` -- same
      Parse/Must pair shape as `ParseRestJsonBody`/`MustParseRestJsonBody`
      (AD-021), for multipart/form-data bodies specifically.
- [x] Non-file form fields validated via the SAME `*Schema`/`NewSchema[T]`
      mechanism as JSON body/params/query (Required/Min/Max/Custom(fn),
      etc) -- no second declaration style to learn.
- [x] File parts delivered via a synchronous per-file callback, invoked THE
      MOMENT each file-part is encountered while walking the raw multipart
      stream -- never buffered locally first. A handler can pipe
      `file.Reader()` directly into an S3 upload (or any `io.Writer`-based
      destination) from inside the callback.
- [x] Zero behavioral change for every EXISTING route (JSON body, params,
      query) when this feature is enabled at the app level.

## Out of Scope

| Feature                                                                    | Reason                                                                                                                                                             |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Per-route toggle of streaming mode                                        | `StreamRequestBody`/`DisablePreParseMultipartForm` are `fasthttp.Server`-level (whole-process) settings -- Fiber has no per-route override. App-wide only (P1's Edge Cases documents the consequence).      |
| Nested/repeated form fields (arrays, `foo[]=1&foo[]=2`, nested objects)    | Params/Query (the closest existing precedent) are flat single-value only; multipart form fields follow the same flat precedent for P1. Revisit only if a real need appears. |
| OpenAPI/Swagger documentation of `multipart/form-data` request bodies     | Needs its own `internal/openapi` design (different media type, `type: string, format: binary` for file fields) -- separate follow-up feature, not blocking runtime behavior (same graceful-degradation precedent as `Custom(fn)` fields today). |
| Multiple files under the SAME field name (`<input multiple>`)             | P1 ships one callback invocation per file PART regardless of field name repetition (the callback already fires once per file naturally) -- no special "grouping by field name" API. Revisit only if requested. |
| Automatic upload-size enforcement per file                                | `fasthttp`'s existing `maxRequestBodySize`/`Server.MaxRequestBodySize` already caps the WHOLE request; a per-file cap would need to be enforced inside the dev's own `onFile` callback (they already control the `io.Reader`) -- no framework-level primitive added in P1. |

---

## User Stories

### P1: Stream an uploaded file straight to storage ⭐ MVP

**User Story**: As a gonest user, I want to accept a `multipart/form-data`
upload (file + a few regular form fields) and start forwarding the file's
bytes to my own storage backend (S3, disk, anything `io.Writer`) as they
arrive, without gonest ever buffering the whole file itself.

**Why P1**: This is the entire point of the feature -- without true
streaming, this is just "yet another multipart parser," a solved problem
elsewhere. The streaming guarantee is what's actually new.

**Acceptance Criteria**:

1. WHEN `AppOptions.EnableFormStreaming` is `true` at `NewApp`/`MustNewApp`
   time THEN the underlying Fiber app SHALL be configured with
   `StreamRequestBody: true` AND `DisablePreParseMultipartForm: true` (both
   required together -- confirmed via `fasthttp` source that pre-parsing
   stays ON by default even with `StreamRequestBody` alone, which would
   silently defeat streaming).
2. WHEN a handler calls `gonest.MustParseRestFormBody[T](ctx, schema,
   onFile)` (or `ParseRestFormBody` for the non-panicking form) on a request
   whose `Content-Type` is `multipart/form-data; boundary=...` THEN the
   framework SHALL walk the raw request body as a stream (via
   `*fasthttp.RequestCtx.RequestBodyStream()` wrapped in
   `mime/multipart.NewReader`), never fully buffering it into memory itself.
3. WHEN the stream walk encounters a part with `Content-Disposition`'s
   `filename` set (a file part) THEN the framework SHALL invoke `onFile`
   synchronously, passing a `*FormFile` whose `Reader() io.Reader` gives
   access to THAT PART'S bytes only, still un-consumed at call time.
4. WHEN the stream walk encounters a part with no `filename` (a regular
   form field) THEN its value SHALL be collected and validated against `m`
   exactly like `ParseRestJsonBody` validates JSON keys (Required, type
   coercion, `Custom(fn)`), using a NEW `form:"..."` struct tag (distinct
   from `json`/`param`/`query`, matching each existing source's own tag
   convention).
5. WHEN every part has been walked (`io.EOF`) AND no violation was
   collected THEN `ParseRestFormBody` SHALL return a populated `T` (built
   from the collected form-field values) and a `nil` error.
6. WHEN a required form field was absent, or present but invalid, THEN
   `ParseRestFormBody` SHALL return a non-nil `*BadRequestException`
   (collecting every violation, exact same collect-all convention as
   `ParseRestJsonBody`) and a zero `T` -- `onFile` may already have fired
   for file parts encountered BEFORE the invalid field was reached (a
   streaming-inherent trade-off: violations can only be known after the
   fact, whereas the callback already ran in real time -- see Edge Cases).

**Independent Test**: A real HTTP dispatch (`httptest`, matching
`TestMustJsonBody_RealHTTPDispatch_HappyPath`'s own precedent, NOT the
lightweight in-memory `fakeResponder` -- multipart streaming needs a REAL
byte-stream to prove anything) posts a multipart body with 1 text field + 1
small file; the test's `onFile` callback copies `file.Reader()` into an
in-memory `bytes.Buffer` (standing in for "upload to S3") and the test
asserts: the buffer's final content matches the uploaded file's bytes, the
returned `T`'s text field is populated, and (via an instrumented Reader
wrapper) that the file's bytes were available to `onFile` BEFORE the
request's underlying connection had delivered the rest of the multipart
body -- proving no full-body buffering happened first.

---

### P2: Reject an invalid/oversized file from inside the callback

**User Story**: As a gonest user, I want to reject a file upload (wrong
content-type, too large, whatever domain-specific check I need) FROM INSIDE
my own `onFile` callback, and have that turn into the same
`*BadRequestException` shape every other validation failure produces.

**Why P2**: File-level validation (size, real content sniffing, virus scan
hook, etc) is inherently freeform/domain-specific -- Schema's fixed
vocabulary (Integer/String/Min/Max) has no way to express it, same
rationale as `PropertyBuilder.Custom(fn)` already established for
param/query/JSON fields.

**Acceptance Criteria**:

1. WHEN `onFile` returns a non-nil `error` THEN `ParseRestFormBody` SHALL
   stop walking further parts immediately and return a zero `T` plus a
   `*BadRequestException` whose `Details()` includes the field name (from
   `Content-Disposition`) and the callback's own error message.
2. WHEN `onFile` returns `nil` for every file part encountered THEN file
   validation contributes ZERO violations (silence = accepted).

**Independent Test**: `onFile` returns `fmt.Errorf("file too large")` for a
deliberately-oversized test file; assert the panic/error is
`*BadRequestException` with that message reachable via `Details()`.

---

### P3: `Custom(fn)` on form fields (same escape hatch as everywhere else)

**User Story**: As a gonest user, I want `PropertyBuilder.Custom(fn)` to
work on `form:"..."` fields exactly like it already works on
`param:"..."`/`query:"..."`/`json:"..."` fields, for domain-specific
non-file form values (e.g. a prefixed ID submitted as a form field
alongside the upload).

**Why P3**: Consistency completionism, not a new capability -- `Custom(fn)`
already exists and already works identically across every OTHER source;
this just proves the SAME mechanism reaches `form:"..."` too, no new code
path.

**Acceptance Criteria**:

1. WHEN a `form:"..."` field has `Custom(fn)` set THEN `fn` SHALL receive
   the RAW STRING value (same convention as `param`/`query`'s own
   `Custom(fn)`, never pre-coerced).

---

## Edge Cases

- WHEN `AppOptions.EnableFormStreaming` is `false` (default) AND a handler
  calls `ParseRestFormBody`/`MustParseRestFormBody` THEN the framework
  SHALL return/panic a clear plain-string error identifying the missing
  `AppOptions` toggle -- NOT a generic "stream unavailable" failure a dev
  has to guess at.
- WHEN the request's `Content-Type` is NOT `multipart/form-data` (missing
  or wrong boundary) THEN `ParseRestFormBody` SHALL return a
  `*BadRequestException` before touching the body at all (same "fail
  loud, fail early" precedent as `ParseRestJsonBody`'s malformed-JSON case).
- WHEN violations are found in form FIELDS that appear in the multipart
  stream AFTER one or more files THEN those files' `onFile` callbacks will
  ALREADY have run (streaming is inherently forward-only, there is no
  "look ahead and validate everything first" option without buffering,
  which defeats the whole feature) -- this is a DOCUMENTED trade-off, not
  a bug: a dev whose `onFile` starts an S3 multipart upload should design
  their OWN cleanup/rollback for "upload started, but the surrounding
  request turned out invalid" (e.g. an abandoned-upload garbage collector
  on the S3 side), same as any real streaming upload system (this is not
  unique to gonest).
- WHEN a route's Handler never calls `ParseRestFormBody`/
  `MustParseRestFormBody` for a genuinely multipart request THEN the raw
  body stream is simply never consumed by gonest -- Fiber/fasthttp's own
  connection-handling cleans it up same as any unread stream today, no new
  leak introduced.
- WHEN `onFile`'s callback does NOT fully drain `file.Reader()` before
  returning THEN the NEXT part's read will implicitly consume (and discard)
  whatever remains of the current part first -- standard `multipart.Reader`
  behavior (`NextPart()` already does this), not a new mechanism to learn.

---

## Requirement Traceability

| Requirement ID | Story                                     | Phase   | Status   |
| -------------- | ------------------------------------------ | ------- | -------- |
| MPF-01         | P1: AppOptions.EnableFormStreaming wiring   | T1      | Verified |
| MPF-02         | P1: Responder/Context raw-stream access    | T2      | Verified |
| MPF-03         | P1: multipart walk + form-tag validation   | T3      | Verified |
| MPF-04         | P1: per-file synchronous callback          | T3, T5  | Verified |
| MPF-05         | P1: Parse/MustParse root wrappers          | T4      | Verified |
| MPF-06         | P2: onFile error -> BadRequestException    | T3      | Verified |
| MPF-07         | P3: Custom(fn) on form fields              | T3      | Verified |

**ID format:** `MPF-[NUMBER]` (Multipart Form)

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 7 total, 7 mapped to tasks, 0 unmapped -- see `.specs/features/multipart-form-streaming/tasks.md` for the T1-T6 breakdown and `design.md` for architecture.

---

## Required Adjustments (explicit, per user request)

Everything below is a REAL change to existing code/interfaces, not purely
additive -- called out so nothing is a surprise during implementation:

1. **`gonest.AppOptions`** gains a new field, e.g. `EnableFormStreaming
   bool` (default `false`, opt-in). `internal/app.NewApp` must translate
   this into BOTH `fiber.Config.StreamRequestBody: true` AND
   `fiber.Config.DisablePreParseMultipartForm: true` on the adapter's Fiber
   app -- a single friendly gonest-level toggle hides 2 raw fasthttp-level
   flags that must always travel together (confirmed via source: one
   without the other silently defeats streaming).
2. **This is an APP-WIDE, not per-route, setting.** Every route in the
   whole app is affected once turned on (Out of Scope table above) --
   existing JSON body/param/query routes remain functionally unchanged
   (confirmed via `fasthttp` source: `Request.Body()` auto-drains the
   stream into a buffer on first touch, so any EXISTING call site that
   still calls `ctx.Body()` keeps working exactly as today), but this is
   worth the user's explicit sign-off before implementation starts, since
   it's an app-level architectural knob, not a route-level opt-in like
   everything else in gonest today.
3. **`internal/execution.Responder` interface gains a new method** (exact
   shape TBD in design.md, roughly `BodyStream() (io.Reader, string, bool)`
   returning reader/boundary/ok) -- this is a BREAKING interface change:
   EVERY existing fake `Responder` implementation across the test suite
   (`fakeResponder`, `paramFakeResponder`, `queryFakeResponder`,
   `httpFiberResponder`, etc -- same blast radius Terminus's `SendString`
   addition already had, "toda fakeResponder de teste no repo precisou de 1
   método a mais") needs the new method added, even if most just return
   `(nil, "", false)`.
4. **`internal/adapter/fiber.fiberResponder`** implements the new method
   for real, via `c.RequestCtx().RequestBodyStream()` + the boundary parsed
   from the `Content-Type` header.
5. **New internal code**: a home for the multipart-walking core (proposed:
   `internal/validate/form.go`, alongside `params.go`/`query.go`, reusing
   `resolveSchema`/`violation`/`validateValue`/`populate` unchanged) plus
   the `FormFile` type itself (proposed: `internal/validate.FormFile`,
   re-exported at root as `gonest.FormFile`).
6. **Root `gonest.go`** gains `ParseRestFormBody[T]`/`MustParseRestFormBody[T]`
   (AD-021's pair shape) + the `FormFile` type alias.
7. **Testing strategy for this feature must use REAL HTTP dispatch**
   (`httptest`), not the lightweight fake-responder unit-test style most of
   `internal/validate`'s existing suite uses -- a fake in-memory responder
   cannot meaningfully prove "bytes were available before the rest of the
   body arrived," which is the entire point being tested.
8. **New struct tag namespace**: `form:"..."` (distinct from
   `json`/`param`/`query`), following the same "one tag family per source"
   precedent already established, not the "everything is `json:` tag" idea
   discussed (and set aside) during MustParse-consolidation brainstorming.

---

## Success Criteria

- [x] A file uploaded via `multipart/form-data` is fully forwarded to a
      test in-memory destination via `onFile`'s callback, with a real,
      instrumented assertion that it happened WITHOUT the whole file being
      buffered by gonest first. (`TestParseRestFormBody_RealHTTPDispatch_StreamsFileWithoutFullBuffering`, `gonest_test.go`)
- [x] Every pre-existing test in the repo still passes unmodified in
      BEHAVIOR (only the mechanical `Responder` interface addition touches
      existing fake responders, zero assertion changes).
- [x] `go test ./... -race` green. SPEC_DEVIATION from the literal wording
      below: only P1 got a real-HTTP-dispatch test (T5) -- P2/onFile-error
      and P3/Custom(fn) are proven at the `ParseFormBody` unit level
      instead (`internal/validate/form_test.go`, written in T3), a
      deliberate call made during T6 (see tasks.md): they exercise the
      exact same validation logic P1's real dispatch already proves reaches
      the network correctly, so a second real-dispatch test per case would
      re-prove the transport path, not add new coverage.
