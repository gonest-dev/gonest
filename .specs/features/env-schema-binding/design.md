# Env → Schema Binding Design

**Spec**: `.specs/features/env-schema-binding/spec.md`
**Context**: `.specs/features/env-schema-binding/context.md`
**Status**: Complete

---

## Architecture Overview

```mermaid
graph TD
    A["gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)"] --> B["*dotenv.Dotenv.ParseInto(dst any, schema any) error -- NEW, satisfies execution.Parseable"]
    B --> C["resolveSchema(m, dstVal.Type()) -- reused unchanged"]
    B --> D["loop m.OwnProperties(): tagKeyVisible(p.Field(), \"env\") -- NEW tag name, same helper"]
    D --> E{"os.LookupEnv(key)"}
    E -- absent + Default set --> F["presence[key] = p.DefaultValue() (NEW) -- skips Required check entirely"]
    E -- absent + no Default --> G["p.IsRequired()? -> violation : skip (zero-value)"]
    E -- present --> H["Custom(fn)? -> validateValue(raw, p, key) direct : coerceParamString(raw, p.KindValue()) -- REUSED unchanged"]
    H --> I["validateValue(coerced, p, key) -- REUSED unchanged"]
    F --> J["populate(dstVal, presence, m, \"env\") -- REUSED unchanged"]
    I --> J
    G --> J
    J --> K["*DatabaseConfig fully populated"]
```

Mirrors `paramsSource.ParseInto` (`internal/validate/params.go:63-120`) almost exactly -- same shape
as `querySource`/`headersSource`. The ONLY genuinely new pieces are: (1) `envSource` itself (new file,
same package), (2) `PropertyBuilder.Default`/`DefaultValue` (new, `internal/schema`), (3) `Dotenv.
ParseInto` (new method on the SAME type `dotenv-loading` already built `Load`/`MustLoad` on). Zero
change to `validateValue`/`populate`/`coerceParamString`/`resolveSchema`/`tagKeyVisible`.

---

## Open Edge Cases Resolved (spec.md left these to Design)

| Edge Case (spec.md) | Resolution | Reasoning |
| --------------------- | ---------- | --------- |
| Env var set but EMPTY (`DB_HOST=`) -- absent for `Default` purposes, or present-with-empty-value? | **Present** (NOT absent) -- `envSource` uses `os.LookupEnv(key)`, which distinguishes unset (`ok=false`) from set-to-empty (`ok=true, value=""`). `Default` only applies when `ok=false`. An empty-but-set value goes through the normal `coerceParamString`/`validateValue` path like any other value (may fail `Required`'s sibling checks -- e.g. `Min(1)` on a string -- same as it would for REST today) | Matches POSIX `${VAR-default}` semantics already implemented in `dotenv-loading` (unset triggers, empty-but-set does not) -- one consistent "absent" definition across both features, not two |
| `Schema` used by `env-schema-binding` has a field with NO `env:"..."` tag | `tagKeyVisible(p.Field(), "env")` returns `visible=false` for that field -- `envSource`'s loop simply `continue`s, same as `paramsSource`/`querySource` already do for a field missing THEIR governing tag. No build-time error, field is left at its Go zero-value | Consistent with existing sources' behavior -- introducing a NEW rule (build-time error) only for `env` would be an unexplained inconsistency, and REST fields legitimately mix tags today (a struct can have both `param` and `json` fields side by side, each source only reads its own) |

---

## Components

### `PropertyBuilder.Default` (new -- `internal/schema/schema.go`)

- **Purpose**: Declare a fallback value used when a field's source data is ABSENT (not merely
  invalid) -- today no branch of `PropertyBuilder` has this concept.
- **Location**: `internal/schema/schema.go`, right beside `Custom`/`CustomFunc` (same file, same
  bare-return/last-call-wins/bool-presence conventions)
- **Interfaces**:
  - `(*PropertyBuilder) Default(value any) *PropertyBuilder` -- stores `value` unexported
    (`p.defaultValue`), sets `p.hasDefault = true`, returns `p` bare (chainable, mirrors `Custom`)
  - `(*PropertyBuilder) DefaultValue() (any, bool)` -- mirrors `CustomFunc`'s `(value, bool)` shape
    exactly (`MinValue`/`MaxValue`/`ItemRef`/`SchemaRef`'s own "never called" convention, per
    `Custom`'s own doc comment referencing the same pattern)
- **Dependencies**: none new.
- **Reuses**: the exact field/method PAIR pattern (`custom`/`Custom`/`CustomFunc`) already established
  -- no new convention invented.
- **Scope**: usable on ANY `PropertyBuilder` (the storage lives on the shared struct, same as every
  other modifier) -- but per spec.md's Out of Scope, ONLY `envSource` reads it in this feature.
  `paramsSource`/`querySource`/`headersSource`/`formBodySource`/`jsonBodySource` could start reading
  it too with a small follow-up change to each (their own presence-check branch), deferred (Out of
  Scope table, `INSIGHT-CONFIG.md`'s "O que fica em aberto").

### `envSource` (new -- `internal/validate/env.go`)

- **Purpose**: The `execution.Parseable` implementation `*dotenv.Dotenv.ParseInto` delegates to --
  same role `paramsSource`/`querySource` play for `ctx.Params()`/`ctx.Query()`.
- **Location**: `internal/validate/env.go` (new file, same package as every other `*Source`)
- **Interfaces**: `func ParseEnvInto(dst any, schemaArg any) error` (a free function, NOT a struct with
  a `req *execution.Request` field like `paramsSource` -- there IS no request in a config-loading
  context; `internal/dotenv.Dotenv.ParseInto` calls this directly). Exported (capital `P`) since
  `internal/dotenv` needs to call it and importing `internal/validate` FROM `internal/dotenv` is a
  valid, acyclic direction (`internal/dotenv` becomes a leaf CONSUMER of `internal/validate`, which
  already has zero dependency on `internal/dotenv`).
- **Behavior**: for each `m.OwnProperties()`: `tagKeyVisible(p.Field(), "env")`; `os.LookupEnv(key)`
  for presence (see Edge Cases table); on absent+no-Default → `Required` check (violation or skip);
  on absent+Default → `presence[key] = p.DefaultValue()` directly (SKIPS `coerceParamString`/
  `validateValue` entirely -- a `Default` value is assumed to already be the correct Go type the dev
  wrote in code, e.g. `.Default(5432)` for an `Integer()` field, not a string needing coercion); on
  present → same `Custom`-first-else-`coerceParamString`-then-`validateValue` branching
  `paramsSource` already uses, verbatim. Ends with `populate(dstVal, presence, m, "env")`.
- **Dependencies**: `internal/schema`, `internal/execution` (for the `violation`/`exception.
  NewBadRequestException` shape, same imports `params.go` already has), `os`.
- **Reuses**: `resolveSchema`, `tagKeyVisible`, `coerceParamString`, `validateValue`, `populate` -- ALL
  unchanged, zero new parameters added to any of them.

### `Dotenv.ParseInto` (new method -- `internal/dotenv/dotenv.go`)

- **Purpose**: Satisfy `execution.Parseable` on the SAME `*Dotenv` singleton `dotenv-loading` already
  built `Load`/`MustLoad` on (`context.md`'s D2 -- one instance, two capabilities).
- **Location**: `internal/dotenv/dotenv.go` (same file `Load`/`MustLoad` already live in, or a new
  `internal/dotenv/parseinto.go` if the file is getting large by Tasks time -- Implementer's call)
- **Interfaces**: `(*Dotenv) ParseInto(dst any, schema any) error { return validate.ParseEnvInto(dst,
  schema) }` -- a ONE-LINE delegation, `Dotenv` itself carries no env-reading logic of its own.
- **Dependencies**: `internal/validate` (new import for this package -- confirmed acyclic: `internal/
  validate` has no dependency on `internal/dotenv` today, and gains none from this change).
- **Reuses**: 100% of `envSource`'s logic -- this method exists purely to satisfy the `Parseable`
  interface's method NAME (`ParseInto`) on the type callers actually hold (`*Dotenv`, returned by
  `gonest.Dotenv()`).

---

## Tech Decisions

| Decision | Choice | Rationale |
| -------- | ------ | --------- |
| `envSource` as a struct (`paramsSource`-style, holding a `*execution.Request`) vs. a free function | Free function `ParseEnvInto(dst, schema)` | No per-request state exists for env-loading (config is read once, at boot, not per-HTTP-request) -- a struct with an unused/nil `req` field would be a lie about what it holds |
| Does `Default`'s value go through `coerceParamString`/`validateValue`? | NO -- used as-is, straight into `presence` | A `Default` value is a Go literal the DEV wrote (`.Default(5432)`, already an `int`) -- coercion is for STRING sources (env vars, query params) that need parsing INTO a type; a `Default` already IS that type. Running it through string-coercion would require the dev to write `.Default("5432")` (a string) for an `Integer` field, which is backwards from how the API reads |
| `internal/dotenv` importing `internal/validate` | Confirmed acyclic, allowed | `internal/validate` already sits below `internal/dotenv` in the dependency graph (validate has zero knowledge of dotenv); same direction `internal/adapter/fiber` importing `internal/app` established in AD-041 (a leaf importing a slightly-less-leaf package it needs a symbol from, never the reverse) |

---

## Error Handling Strategy

| Error Scenario | Handling | Caller Sees |
| --------------- | -------- | ----------- |
| Required field, no `Default`, env var unset | `violation{Field: key, Message: "required"}` collected, same as any other source | `MustParse` panics with `exception.NewBadRequestException([]violation{...})`, ALL missing fields listed (collect-all) |
| Env var present but fails type coercion (e.g. `DB_PORT=notanumber` for an `Integer()` field) | `coerceParamString` returns its own error, recorded as a violation for that field, same as `paramsSource` today | Same `BadRequestException`-shaped panic, field-specific message |
| `Provider.Constructor` calling `MustParse` panics (any reason above) | `internal/resolver/stage3.go`'s `callConstructor` recovers, converts to `error`, cancels the bootstrap `errgroup` -- ZERO new code needed here, already-existing behavior | `NewApp`/`MustNewApp` returns/panics with a wrapped error identifying which provider failed and why |

---

## Traceability to Spec

| Requirement ID | Design Component |
| -------------- | ----------------- |
| ENVCFG-01 | `envSource`/`ParseEnvInto`, `Dotenv.ParseInto` |
| ENVCFG-02 | `envSource`'s Required-check branch (collect-all violations) |
| ENVCFG-03 | `PropertyBuilder.Default`/`DefaultValue`, `envSource`'s absent+Default branch |
