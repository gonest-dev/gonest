package gonest

// Scope defines the lifetime of a provider instance within the DI container.
type Scope int

const (
	// ScopeSingleton means a single shared instance is created and reused
	// for the lifetime of the application.
	ScopeSingleton Scope = iota
	// ScopeTransient means a new instance is created every time the
	// provider is resolved.
	ScopeTransient
	// ScopeRequest means a single instance is created per incoming request
	// and shared across resolutions within that request.
	ScopeRequest
)

// String implements fmt.Stringer for debug-friendly output.
func (s Scope) String() string {
	switch s {
	case ScopeSingleton:
		return "Singleton"
	case ScopeTransient:
		return "Transient"
	case ScopeRequest:
		return "Request"
	default:
		return "Unknown"
	}
}

// ownerModule is implemented by internal DI graph nodes (e.g. modules,
// providers, controllers) to report which Module they belong to. It is
// used internally by MustResolve to walk the ownership chain.
//
// The return type is intentionally any here: the concrete *Module type
// is introduced in a later task. This interface will be tightened to
// return *Module once that type exists.
type ownerModule interface {
	ownerModule() any
}
