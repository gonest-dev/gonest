# Accessor Sync Design

## Architecture Overview

`internal/accessor` will be extended with `sync.go` containing `SyncAccessorFields(dst any, src any)`.

```
gonest.SyncAccessorFields(dst, src)
   └── accessor.SyncAccessorFields(dst, src)
          ├── Traverse src fields (including embedded structs)
          ├── Detect Accessor[T] fields where IsDirty() == true
          └── Match & Apply to dst fields (Accessor[T], T, or *T)
```

## Reflection Strategy & Matching

1. **Unwrapping**: Dereference pointers on `src` and `dst` until reaching `reflect.Struct`.
2. **Accessor Detection**: Check if field implements `interface { IsDirty() bool }` or has method `IsDirty() bool`.
3. **Value Extraction**: Extract value via `Get()` method (`reflect.Value.MethodByName("Get")`).
4. **Target Matching**:
   - Match by exact struct field name (e.g. `Name`).
   - Fallback: match by `json` tag name (e.g. `json:"name,omitempty"` -> `"name"`).
5. **Target Assignment**:
   - If target field is an `Accessor[T]`, call `Set(val)`.
   - If target field type matches `val.Type()`, call `Set(val)`.
   - If target field is pointer `*T` matching `val.Type()`, allocate if nil and set `*ptr = val`.

## API Exposure

In `gonest.go`:

```go
// SyncAccessorFields copies dirty Accessor[T] fields from src into matching fields of dst.
func SyncAccessorFields(dst any, src any) {
	accessor.SyncAccessorFields(dst, src)
}
```
