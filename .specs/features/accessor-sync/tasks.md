# Accessor Sync Tasks

**Design**: `.specs/features/accessor-sync/design.md`
**Testing**: `internal/accessor/sync_test.go`, `gonest_test.go`
**Status**: COMPLETE

---

## Execution Plan

```
T0 (implement internal/accessor.SyncAccessorFields + unit tests) -- COMPLETE
  → T1 (re-export in gonest.go + integration test in gonest_test.go) -- COMPLETE
```

---

## Task Breakdown

### T0: Implement `internal/accessor.SyncAccessorFields` -- COMPLETE

**What**: Implement `SyncAccessorFields(dst any, src any)` in `internal/accessor/sync.go`. Supports syncing dirty `Accessor[T]` fields into `Accessor[T]`, raw `T`, and `*T` fields in `dst`, including embedded struct traversal and name/json-tag matching.

**Where**: `internal/accessor/sync.go`, `internal/accessor/sync_test.go`

**Depends on**: none

**Requirement**: ACC-01, ACC-02, ACC-03, ACC-04

**Done when**:
- [x] `SyncAccessorFields` syncs dirty `Accessor[T]` to `Accessor[T]` in `dst`.
- [x] `SyncAccessorFields` syncs dirty `Accessor[T]` to raw `T` and `*T` in `dst`.
- [x] Non-dirty fields in `src` are ignored.
- [x] Embedded structs in both `src` and `dst` are correctly traversed.
- [x] `go test ./internal/accessor/...` passes.

---

### T1: Re-export `SyncAccessorFields` in `gonest.go` & verify -- COMPLETE

**What**: Re-export `gonest.SyncAccessorFields(dst, src)` in `gonest.go` and add end-to-end integration test in `gonest_test.go`.

**Where**: `gonest.go`, `gonest_test.go`

**Depends on**: T0

**Requirement**: ACC-05

**Done when**:
- [x] `gonest.SyncAccessorFields` is accessible from the root package.
- [x] `go test ./...` passes completely across all packages.
