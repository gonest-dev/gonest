# Enum Naming Convention ✅

## Standard Applied to All Enums

To avoid naming conflicts and improve clarity, all enums in GoNest follow this pattern:

```go
type EnumName string

const (
    // EnumNameVALUE1 - description
    EnumNameVALUE1 EnumName = "value1"
    
    // EnumNameVALUE2 - description
    EnumNameVALUE2 EnumName = "value2"
)
```

**Pattern:**
- Type name: `EnumName` (PascalCase)
- Constant prefix: `EnumName` (same as type)
- Constant suffix: `VALUE` (UPPERCASE)
- Full constant: `EnumNameVALUE`

## Examples

### ✅ Correct (New Convention)

```go
// Scope enum
type Scope string

const (
    ScopeSINGLETON  Scope = "singleton"
    ScopeTRANSIENT  Scope = "transient"
    ScopeREQUEST    Scope = "request"
)

// MetadataKey enum
type MetadataKey string

const (
    MetadataKeyCONTROLLER   MetadataKey = "controller"
    MetadataKeyROUTE        MetadataKey = "route"
    MetadataKeyGUARD        MetadataKey = "guard"
)
```

### ❌ Incorrect (Old Convention - Conflicts)

```go
// BAD: Can conflict with functions
type Scope string

const (
    SingletonScope  Scope = "singleton"  // ❌ Can clash with Singleton()
    TransientScope  Scope = "transient"  // ❌ Can clash with Transient()
    RequestScope    Scope = "request"    // ❌ Conflicts!
)
```

## Benefits

### 1. **No Naming Conflicts**
```go
// Enum constant
const value = ScopeSINGLETON

// Helper function - NO CONFLICT!
func Singleton() ProviderOption {
    return WithScope(ScopeSINGLETON)
}
```

### 2. **Clear Namespace**
```go
// Clear that this is the Scope enum's SINGLETON value
ScopeSINGLETON

// Clear that this is the MetadataKey enum's CONTROLLER value
MetadataKeyCONTROLLER
```

### 3. **IDE Autocomplete**
Type `Scope` and IDE shows all scope values:
- `ScopeSINGLETON`
- `ScopeTRANSIENT`
- `ScopeREQUEST`

### 4. **Grep-Friendly**
```bash
# Find all Scope enum usage
grep "Scope[A-Z]" *.go

# Find all MetadataKey usage
grep "MetadataKey[A-Z]" *.go
```

## Updated Files

### DI Module
- ✅ `di/types.go` - `Scope` enum
  - `ScopeSINGLETON`
  - `ScopeTRANSIENT`
  - `ScopeREQUEST`

### Core Module
- ✅ `core/metadata.go` - `MetadataKey` enum
  - `MetadataKeyCONTROLLER`
  - `MetadataKeyROUTE`
  - `MetadataKeyGUARD`
  - `MetadataKeyINTERCEPTOR`
  - `MetadataKeyPIPE`
  - `MetadataKeyPARAM`
  - `MetadataKeySWAGGER`

## Migration Guide

When creating new enums:

1. **Define the type:**
   ```go
   type Status string
   ```

2. **Create constants with pattern:**
   ```go
   const (
       StatusACTIVE   Status = "active"
       StatusINACTIVE Status = "inactive"
       StatusPENDING  Status = "pending"
   )
   ```

3. **Helper functions are free to use natural names:**
   ```go
   func Active() Option {
       return WithStatus(StatusACTIVE)
   }
   
   func Inactive() Option {
       return WithStatus(StatusINACTIVE)
   }
   ```

## Complete Example

```go
// HTTP Method enum
type HTTPMethod string

const (
    HTTPMethodGET    HTTPMethod = "GET"
    HTTPMethodPOST   HTTPMethod = "POST"
    HTTPMethodPUT    HTTPMethod = "PUT"
    HTTPMethodPATCH  HTTPMethod = "PATCH"
    HTTPMethodDELETE HTTPMethod = "DELETE"
)

// Helper functions - no conflicts!
func Get(path string) RouteBuilder {
    return NewRoute(HTTPMethodGET, path)
}

func Post(path string) RouteBuilder {
    return NewRoute(HTTPMethodPOST, path)
}
```

## Why This Matters

**Before (with conflicts):**
```go
// ❌ Ambiguous - is this a constant or function?
RequestScope

// Solution: rename one of them
// But which one? Both are valid names!
```

**After (no conflicts):**
```go
// ✅ Clear - this is the enum value
ScopeREQUEST

// ✅ Clear - this is the helper function
Request()

// ✅ No ambiguity anywhere!
```

## Standard Applied Going Forward

**All new enums MUST follow this convention.**

This is now documented and will be enforced in code reviews.

---

**Status:** Convention established and applied to all existing enums ✅