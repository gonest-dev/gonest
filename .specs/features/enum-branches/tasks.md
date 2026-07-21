# Enum Branches Tasks

**Spec**: `.specs/features/enum-branches/spec.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T4, todos evaluator PASS -- `go test ./... -race -count=1` green, 24 pacotes core; `.examples/full-text-search` builds/vets/curl-verifies clean)

---

## Execution Plan

```
T1 (internal/schema: PropertyBuilder enum storage + StringSchema.Enum/NumericSchema.Enum)
  → T2 [P] (internal/validate: reject out-of-list values)
  → T3 [P] (internal/openapi: emit "enum" array)
  → T4 (examples/full-text-search: migrate FieldsSchemaFor off Custom(fn))
```

T2 and T3 both only need T1's storage/getters, are independent of each other (different packages,
neither imports the other), and run in parallel. T4 needs BOTH T1 (the real `Enum` method) and T2
(so the migration is validated by the real mechanism, not just compiling) before it makes sense.

---

## Task Breakdown

### T1: `PropertyBuilder` enum storage + `StringSchema.Enum`/`NumericSchema.Enum`

**What**: `internal/schema/schema.go`'s `PropertyBuilder` struct gains 2 new fields, next to the
existing `min, max *int`/`pattern string` (same "only one family active per PropertyBuilder instance"
comment already covers this):
- `enumString []string`
- `enumInt    []int64`

`internal/schema/string.go`'s `StringSchema` gains:
- `func (s *StringSchema) Enum(items ...string) *StringSchema` -- stores `items` into
  `s.enumString`, returns `s` (same chain-return pattern as `Min`/`Max`/`Pattern`)
- `func (s *StringSchema) EnumValues() ([]string, bool)` -- `(nil, false)` if `Enum` was never
  called, same "never called" vs "called with zero items" distinction `MinValue`/`MaxValue` already
  use (getter distinguishes via a SEPARATE `bool` flag or by checking `items != nil` post-construction
  -- do NOT use `len(items) == 0` as the "never called" signal, since Edge Cases in spec.md requires
  telling "never called" apart from "called with zero items")

`internal/schema/numeric.go`'s `NumericSchema` gains the same 2 methods, `int64` instead of `string`:
- `func (n *NumericSchema) Enum(items ...int64) *NumericSchema`
- `func (n *NumericSchema) EnumValues() ([]int64, bool)`

**Where**: `internal/schema/schema.go` (existing, extended), `internal/schema/string.go` (existing,
extended), `internal/schema/numeric.go` (existing, extended), `internal/schema/string_test.go`
(existing, extended), `internal/schema/numeric_test.go` (existing, extended)

**Depends on**: none
**Reuses**: `PropertyBuilder`'s existing `min`/`max`/`pattern` field precedent (same struct, same
"last-call-wins, no panic" branch-method convention every prior branch method in this package follows)
**Requirement**: ENUM-01, ENUM-02

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `.Enum("a","b")` on a `*StringSchema` stores the list, `.EnumValues()` returns `([]string{"a","b"}, true)`
- [ ] `.Enum()` never called on a `*StringSchema` → `.EnumValues()` returns `(nil, false)`
- [ ] `.Enum(1,2,3)` on a `*NumericSchema` stores the list, `.EnumValues()` returns `([]int64{1,2,3}, true)`
- [ ] `.Enum()` never called on a `*NumericSchema` → `.EnumValues()` returns `(nil, false)`
- [ ] Both `Enum` methods return the SAME receiver (`s`/`n`), so `.Required().Enum(...).Description(...)`
  chains in any order (mirrors `TestPropertyBuilder_Custom_LastCallWins`-style test already in this
  package for the "last-call-wins, no panic" convention -- Enum called twice keeps only the LAST list)
- [ ] `go vet ./...` clean, no new lint surface
- [ ] Gate check passes
- [ ] Test count: 8+ (String Enum stores/never-called/chain-return/last-call-wins, same 4 for Numeric)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(schema): add Enum to StringSchema and NumericSchema`

---

### T2: `internal/validate` rejects out-of-list values [P]

**What**: `internal/validate/validate.go`'s `validatePrimitive` (the function already dispatching on
`p.KindValue()` for `"string"`/`"integer"`/`"number"`) gains an enum membership check AFTER the
existing type/Min/Max/Pattern checks for each kind, so it never short-circuits a check already there
(spec.md's Edge Cases: "collect every violation, never short-circuit"):
- `case "string"`: if `p.EnumValues()` returns `(list, true)` and the decoded string is not in `list`,
  append a violation (message format: `"must be one of [a b c]"`, matching this codebase's existing
  violation message style -- check `validatePrimitive`'s own current messages for the exact tone/format
  to mirror, e.g. `"expected string"`)
- `case "integer", "number"`: same check against `p.EnumValues()`'s `[]int64` (JSON numbers decode to
  `float64` in this pipeline already -- convert for comparison, do not change the decode path itself)

Nullable/null handling is UNCHANGED (an explicit JSON `null` on a `Nullable()` field is accepted before
`validatePrimitive` ever runs the kind-specific checks -- confirm this by reading the existing call
site in `validateValue`, do not re-derive it).

**Where**: `internal/validate/validate.go` (existing, extended), `internal/validate/*_test.go` (existing
test file covering `validatePrimitive` -- extend, do not create a new file unless none currently covers
this function directly)

**Depends on**: T1
**Reuses**: `validatePrimitive`'s existing violation-collection pattern (same `violation{Field, Message}`
shape, same "append and continue" loop structure) -- do not introduce a new violation type or bypass
mechanism
**Requirement**: ENUM-03, ENUM-04

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] A field with `.Enum("a","b")` set: value `"a"` passes, value `"c"` produces exactly one violation
  naming the field
- [ ] A field with `.Enum(1,2,3)` set: value `2` passes, value `5` produces exactly one violation
- [ ] A field with NO `Enum()` call: any value of the right primitive type still passes exactly as
  before this task (explicit regression assertion, not just "no test failure")
- [ ] A field with `Enum` AND `Min`/`Max`/`Pattern` all set, given a value that violates BOTH: violation
  list contains entries for EACH broken check, not just the first one hit
- [ ] `Nullable()` + explicit JSON `null` on an Enum'd field: still accepted, zero violations (proves the
  null short-circuit still runs before the new check, unchanged)
- [ ] Gate check passes (`go test ./... -race`, full suite -- this package is imported by `internal/app`,
  a regression here could break request dispatch, not just this package's own tests)
- [ ] Test count: 6+ covering the bullets above

**Tests**: unit
**Gate**: full

**Commit**: `feat(validate): reject values outside a field's registered Enum`

---

### T3: `internal/openapi` emits `"enum"` array [P]

**What**: `internal/openapi/generate.go`'s `schemaFor` function (the `switch p.KindValue()` block
already emitting `minLength`/`maxLength`/`pattern` for `"string"` and `minimum`/`maximum` for
`"integer","number"`) gains, in EACH of those 2 branches, right after the existing Min/Max/Pattern
lines:
- `case "string"`: `if items, ok := p.EnumValues(); ok { schema["enum"] = items }` (using
  `StringSchema.EnumValues`'s underlying `[]string`, reached the same way `MinValue`/`PatternValue`
  already are -- via the shared `*PropertyBuilder`, confirm the exact accessor path by reading how
  `MinValue`/`PatternValue` are currently called in this same function before assuming `EnumValues` is
  reachable identically)
- `case "integer", "number"`: same for the `[]int64` variant

No `"enum"` key at all when `Enum` was never called (mirrors how `pattern`'s key is only set when
`pattern != ""` today -- same "omit key entirely when unset" convention, not an empty array).

**Where**: `internal/openapi/generate.go` (existing, extended), `internal/openapi/generate_test.go`
(existing, extended)

**Depends on**: T1
**Reuses**: `schemaFor`'s existing `switch p.KindValue()` dispatch, `MinValue`/`MaxValue`/`PatternValue`
call-site precedent immediately above each insertion point
**Requirement**: ENUM-05, ENUM-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Generated schema for a field with `.Enum("a","b")` has `"enum": ["a","b"]` at
  `components.schemas.<Title>.properties.<field>.enum`
- [ ] Generated schema for a field with `.Enum(1,2,3)` has `"enum": [1,2,3]`
- [ ] Generated schema for a field with NO `Enum()` call has NO `"enum"` key at all (explicit assertion
  the key is absent, not present-and-empty)
- [ ] Gate check passes
- [ ] Test count: 3+ covering the bullets above

**Tests**: unit
**Gate**: quick

**Commit**: `feat(openapi): emit enum array for fields with Enum set`

---

### T4: `.examples/full-text-search` migrates off `Custom(fn)`

**What**: `search.FieldsSchemaFor[T]` (`.examples/full-text-search/shared/search/search.go`) currently
validates `Fields[T].Select`/`Remove` entries via a hand-rolled `Custom(fn func(raw any) (any, error))`
checking membership in `FieldNames[T]()`. Replace with the real `Enum(...)`:

```go
func FieldsSchemaFor[T any]() *gonest.Schema {
	allowed := FieldNames[T]()
	return gonest.NewSchema(func(t *Fields[T], s *gonest.Schema) {
		s.Title("search.Fields")
		s.Property(&t.Select).Array().Items(func(m *gonest.ArraySchema) { m.String().Enum(allowed...) })
		s.Property(&t.Remove).Array().Items(func(m *gonest.ArraySchema) { m.String().Enum(allowed...) })
	})
}
```

Remove the now-unused `validate`/`fmt`/`slices` helper function and any now-dead imports this leaves
behind in `search.go` (check with `go vet`/`goimports`, do not leave an unused-import build error).

**Where**: `.examples/full-text-search/shared/search/search.go` (existing, edited)

**Depends on**: T1, T2 (needs the real `Enum` AND real validation behind it -- migrating to a method
that compiles but doesn't actually validate yet would silently regress this example's own already-proven
behavior)
**Reuses**: `FieldNames[T]()` (unchanged), `Fields[T]` (unchanged) -- only the schema-construction body
of `FieldsSchemaFor` changes
**Requirement**: ENUM-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `.examples/full-text-search` builds (`go build ./...`) and `go vet ./...` clean, run from
  `.examples/full-text-search` (this module is intentionally excluded from the repo root's own
  `go build/test/mod tidy ./...`, per its own `go.mod` comment -- verify from inside that directory)
- [ ] Manual `curl` re-verification (same requests already used earlier in this session): `QUERY /person`
  with `fields.select` containing a real Person field name (e.g. `"name"`) → 200; with a typo'd name
  (e.g. `"nome_errado"`) → 400 with a violation listing the real field names
- [ ] No leftover unused `Custom`-only helper/import in `search.go`

**Tests**: manual (this is an example module, not part of the core `go test ./...` suite -- matches how
every other `.examples/*` change in this repo's history has been verified, see STATE.md precedent e.g.
AD-036/AD-037/AD-039's "evidência real" via live `curl`)
**Gate**: quick (build + vet + the curl checks above)

**Commit**: `refactor(full-text-search): use real Enum instead of Custom(fn) workaround`

---

## Parallel Execution Map

```
T1 → T2 [P] ┐
     T3 [P] ┴→ T4
```

**Papéis por task (Subagent workflow convention em STATE.md):** Implementer sub-agent implementa,
Evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: PropertyBuilder enum storage + Enum methods | 3 arquivos existentes, padrão mecânico já resolvido por Min/Max/Pattern | ✅ Granular |
| T2: validate.go rejection | 1 arquivo existente, 1 switch já existente, 2 novos case-branches | ✅ Granular |
| T3: openapi.go emission | 1 arquivo existente, 1 switch já existente, 2 novas linhas | ✅ Granular |
| T4: example migration | 1 arquivo, remove workaround + troca por API real | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Schema/reflection puro (`internal/schema`) | unit | unit | ✅ OK |
| T2 | Validação de request (`internal/validate`, consumido por `internal/app`) | unit (full gate por segurança de regressão cross-package) | unit / full gate | ✅ OK |
| T3 | Geração de documento OpenAPI (`internal/openapi`) | unit | unit | ✅ OK |
| T4 | Example standalone, fora da suite `go test` do repo core | manual (curl) | manual | ✅ OK |

Nenhuma violação.
