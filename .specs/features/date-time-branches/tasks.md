# Date/Time Branches Tasks

**Spec**: `.specs/features/date-time-branches/spec.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1, commit `558e587`)

---

## Execution Plan

```
T1 (internal/metadata: DateTime() + Date(), both wrapper-less, root re-export test coverage)
```

Single task -- both branches follow the exact `Boolean()` precedent (no wrapper type needed), mechanical to add.

---

## Task Breakdown

### T1: `DateTime()` + `Date()`, both wrapper-less ✅ DONE (commit `558e587`)

**What**: extends `internal/metadata/metadata.go`'s `PropertyBuilder` with:
- `func (p *PropertyBuilder) DateTime() *PropertyBuilder` -- `p.format = "date-time"`, returns `p` direct (no wrapper, same reasoning as `Boolean()`)
- `func (p *PropertyBuilder) Date() *PropertyBuilder` -- `p.format = "date"`, returns `p` direct

No new type, no new file for a metadata wrapper -- both live as doc-commented methods directly on `PropertyBuilder` in `metadata.go`.

**Where**: `internal/metadata/metadata.go` (extended), `internal/metadata/datetime_test.go` (new), `gonest_test.go` (extended -- root-level reproduction of INSIGHT.md's `CreatedAt`/`UpdatedAt`/`DeletedAt` chains)

**Done when**:
- [x] `DateTime()` sets `format == "date-time"` and returns the SAME `*PropertyBuilder` (pointer identity, `got == p`)
- [x] `Date()` sets `format == "date"` and returns the SAME `*PropertyBuilder`
- [x] `Required`/`Nullable`/`Description`/`Examples` work normally after either branch (they're just `PropertyBuilder`'s own methods, no redeclaration needed since no wrapper exists)
- [x] INSIGHT.md's `CreatedAt`/`UpdatedAt` (`DateTime().Required()...`) and `DeletedAt` (`DateTime().Nullable()...`, `Examples(nil, time.Now())`) chains reproduced verbatim and pass
- [x] One `Date()` case (not shown in INSIGHT.md) exercised to prove it isn't missed
- [x] Calling `DateTime()` then `Date()` (or vice versa) on the same `*PropertyBuilder` doesn't panic -- last-write-wins on `format`
- [x] Gate check passes (`go test ./...`, all packages green)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add wrapper-less DateTime/Date branches`

---

## Parallel Execution Map

```
Single task, no parallelism needed.
```

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: DateTime + Date, wrapper-less | 1 extended file + 2 test files, mechanical repeat of `Boolean()` precedent | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Methods on existing type, no new type | unit | unit | ✅ OK |

Nenhuma violação.
