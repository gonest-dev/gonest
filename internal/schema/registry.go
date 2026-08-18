package schema

import (
	"fmt"
	"reflect"
	"sync"
)

// registry is the process-wide lookup from a Go struct type to its
// registered *Schema, so MustJsonBody[T] (internal/validate, out of scope
// here) can find T's schema without an explicit argument -- INSIGHT.md's
// own call shape (gonest.MustJsonBody[*UserProperties](ctx)) never passes
// one. Populated automatically by New (NewSchema[T]'s internal call),
// never by hand -- registration becomes impossible to forget.
//
// Guarded by registryMu (not the zero-value sync.Map) because the whole
// registry is a single conceptual unit: Register's duplicate check and the
// write it guards must be atomic together, which sync.Map's own
// LoadOrStore could do too, but a plain map+RWMutex keeps the panic
// message construction (fmt.Sprintf) trivially outside the critical
// section without a second API to learn.
var registryMu sync.RWMutex
var registry = map[reflect.Type]*Schema{}

// Register stores s under t, the struct type NewSchema[T] was built for.
// Panics if t was already registered -- one schema declaration per type,
// same precedent as Property's own double-registration panic (schema.go):
// silently overwriting an earlier declaration would let a typo'd duplicate
// NewSchema[T] call silently corrupt validation behavior instead of
// failing loudly at startup.
func Register(t reflect.Type, s *Schema) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[t]; ok {
		panic(fmt.Sprintf("gonest: NewSchema already registered for type %s", t))
	}
	registry[t] = s
}

// Lookup returns the *Schema registered for t and true, or (nil, false)
// if t was never registered -- the "not found" signal MustJsonBody uses to
// panic a clear "no schema registered for type X" error instead of a
// nil-pointer crash (spec.md's Edge Cases).
func Lookup(t reflect.Type) (*Schema, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[t]
	return s, ok
}

// Deregister removes t's entry from the registry, if any. It is a TEST-ONLY
// escape hatch (unlike internal/inject's Reset, the registry is
// intentionally process-long-lived in production -- see registry_test.go's
// doc comment) -- this package's own test suite builds fresh *Schema
// values for the SAME handful of package-local types across many Test
// functions within one test binary run, which would otherwise trip
// Register's own duplicate panic. Production code has no reason to call
// this: a real process registers each type exactly once, at startup.
func Deregister(t reflect.Type) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(registry, t)
}
