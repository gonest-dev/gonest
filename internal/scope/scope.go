// Package scope defines the lifetime semantics of a DI provider instance.
package scope

// Scope defines the lifetime of a provider instance within the DI container.
type Scope int

const (
	// Singleton means a single shared instance is created and reused
	// for the lifetime of the application.
	Singleton Scope = iota
	// Transient means a new instance is created every time the
	// provider is resolved.
	Transient
	// Request means a single instance is created per incoming request
	// and shared across resolutions within that request.
	Request
)

// String implements fmt.Stringer for debug-friendly output.
func (s Scope) String() string {
	switch s {
	case Singleton:
		return "Singleton"
	case Transient:
		return "Transient"
	case Request:
		return "Request"
	default:
		return "Unknown"
	}
}
