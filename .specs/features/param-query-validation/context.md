# Param, Query & Custom Validation Context

User decisions captured via `AskUserQuestion` (multiple rounds) and iterative INSIGHT.md editing, 2026-07-14. This feature's scope grew substantially across the conversation -- this file is the definitive record of WHY, in the order decisions were made, since each one changed the shape of the next question.

## Decision 1: Param/Query API shape -- whole-object, mirroring MustJsonBody

**Chosen** (via INSIGHT.md metacode, "Opção C"): `MustParams[T](ctx)` (path params) and `MustQuery[T](ctx)` (query string params) -- BOTH struct-based, validated against `NewMetadata[T]`, same shape as `MustJsonBody[T](ctx)`. NOT per-field `MustParam[T](ctx, name)` calls, NOT a `*PropertyBuilder` passed explicitly to a single-param call.

**Rejected alternatives** (Options A/B in INSIGHT.md, both struck): (A) passing a `*PropertyBuilder` explicitly to `MustParam` as a 3rd argument -- rejected, no clean way to say "which field of a multi-field schema does this ONE param correspond to" without inventing new indirection; (B) a `ValidatedPipe(propertyBuilder)` built-in Pipe wrapping an existing `*PropertyBuilder` -- rejected once Decision 2 (below) removed Pipe as a concept entirely, made moot.

## Decision 2: `MustParams`/`MustQuery` REPLACE `MustParam[T](ctx, name)` entirely -- not additive

**Chosen**: the existing singular `gonest.MustParam[T](ctx, name)` (used throughout INSIGHT.md's prior examples, e.g. `/:user_id`) is REMOVED. Every path param access becomes struct-based via `MustParams[T]`, even for a route with only ONE path param.

**Rejected**: keeping both side by side (singular for simple 1-field cases, plural for multi-field) -- user explicitly chose full replacement over coexistence, accepting the blast radius of rewriting every prior INSIGHT.md example that used `MustParam[int64](ctx, "user_id")`.

**Impact**: `internal/route/param.go`'s current `MustParam[T]` implementation (defaultCoerce + custom-Pipe fallback) is fully superseded. This does NOT mean `internal/route` package disappears -- `Route` itself (path/method/handler registration) stays; only the per-param access mechanism changes.

## Decision 3: Pipe (the whole concept) is REMOVED, not just `MustParam`'s Pipe-fallback path

**Chosen**: `internal/pipe` package, `gonest.Pipe`/`gonest.NewPipe` root exports, `Route.Param(name, pipe)`/`Route.PipeFor` -- ALL removed. "Pipe" as a feature (Milestone 3, shipped and marked COMPLETE in ROADMAP.md) is retroactively superseded by this feature.

**Reason**: user's own framing -- Pipe's original intent was "allow custom param transforms," and once `MustParams`/`MustQuery`/`MustJsonBody` all validate against `Metadata` uniformly, a SEPARATE object-based mechanism for the same concern (custom per-field logic) is redundant, UNLESS Metadata itself can't express what Pipe could (see Decision 4).

**Rejected**: keeping Pipe alongside the new Metadata-based validation for cases Metadata doesn't cover -- superseded once Decision 4 gave Metadata its own escape hatch, closing the capability gap that would have otherwise justified keeping Pipe.

## Decision 4: `PropertyBuilder.Custom(fn)` -- escape hatch preserving Pipe's original intent, INSIDE Metadata

**Chosen**: `PropertyBuilder` gains `Custom(fn func(raw any) (any, error)) *PropertyBuilder` (Go disallows generic methods, so `any`-typed signature is the only option, mirroring how `Property(fieldPtr any)` itself is already untyped). When set, the validator calls `fn(raw)` INSTEAD OF the built-in kind/format/Min/Max/Pattern checks for that field -- `fn`'s returned `error` becomes a violation if non-nil, and `fn`'s returned `any` value is what gets written into `T`'s field (via reflect, see Decision 5) if `fn` succeeds.

**Reason**: without this, removing Pipe (Decision 3) would be a genuine capability LOSS -- Pipe's whole purpose was arbitrary Go logic (custom format parsing, signed-token decoding, DB-lookup-based coercion) that no fixed vocabulary of `Min`/`Max`/`Pattern` could express. `Custom(fn)` is the direct replacement, living inside the same declarative surface instead of a separate object type.

**Rejected**: keeping Pipe as a separate mechanism specifically for this case -- once `Custom(fn)` exists, Pipe's remaining justification evaporates; two ways to do the same thing (arbitrary per-field transform) would be worse than one.

## Decision 5: Unify `T` population across `MustJsonBody`/`MustParams`/`MustQuery` -- ALL reflect-based, field-by-field

**Chosen**: `MustJsonBody[T]` (already shipped this session, commits `25ab1e3`/`a9bbda9`) gets REFACTORED. Its current second pass (`json.Unmarshal(body, result)`, a single opaque call) is replaced with a reflect-based walk that populates `T`'s fields ONE AT A TIME from the already-validated presence map, using each field's `PropertyBuilder` (applying `Custom(fn)`'s returned value when set, or falling back to the JSON-decoded value's natural Go type otherwise). `MustParams[T]`/`MustQuery[T]` (new, built from scratch) use the SAME population mechanism from the start, sourcing raw values from path/query strings instead of a JSON body.

**Reason**: discovered while designing `Custom(fn)` -- `MustJsonBody`'s existing two-pass design (generic-any pass for presence/type validation, THEN a completely separate `json.Unmarshal` call for the final typed value) has NO way for `Custom(fn)`'s transformed value to reach `T`, since the second pass never consults `PropertyBuilder`/`Custom` at all, it just re-decodes raw JSON bytes via `encoding/json`'s own rules. Two options were presented: (a) `Custom(fn)` only for `MustParams`/`MustQuery` (new code, built reflect-based from day one), leaving `MustJsonBody` as-is and permanently unable to use `Custom`; (b) unify all three onto one reflect-based population core, giving `MustJsonBody` `Custom(fn)` support too, at the cost of touching already-shipped, evaluator-approved code from earlier this session.

**Rejected**: option (a) above -- user explicitly chose the unified rewrite (b) over leaving `MustJsonBody` as a permanent exception, accepting that this reopens `internal/validate/validate.go`'s `MustJsonBody` function specifically (not the validation/violation-collection logic itself, which stays as-is -- only the FINAL "build `T`" step changes).

**Impact on design**: `internal/validate` needs a NEW shared function, something like `populate[T any](presence map[string]any, m *metadata.Metadata) T` (or an unexported non-generic helper `populateStruct(dest reflect.Value, presence map[string]any, m *metadata.Metadata)` that all three public entry points call after validation passes) -- walks `m.OwnProperties()`, for each field either calls `PropertyBuilder.CustomFunc()` (new getter, mirrors the pattern of `MinValue`/`MaxValue`/etc from AD-012) if set, or performs the appropriate Go-native type conversion (string→int64/float64/bool for Params/Query; the JSON-decoded `any` value for JSON body) and `reflect.Value.Set`s it onto the destination struct field.

## Scope boundary confirmed with user

Everything above is IN SCOPE for this single feature (large, but user confirmed proceeding through 3 escalating `AskUserQuestion` rounds rather than stopping at a smaller cut). Nothing beyond this was raised or implied -- OpenAPI generation (Milestone 7) reading `Custom`/`kind`/etc is explicitly future work, not touched here.
