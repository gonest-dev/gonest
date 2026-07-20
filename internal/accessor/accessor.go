// Package accessor provides Accessor[T], a generic field wrapper that tracks
// whether its value was explicitly set (dirty-tracking).
//
// The primary use case is PATCH-style handlers: decode a JSON body into a
// struct whose fields are Accessor[T]; only fields that were present in the
// JSON payload are marked dirty. The handler can then apply only the dirty
// fields to the entity, leaving untouched fields unchanged without needing
// any pointer-based optional semantics.
//
// JSON integration is transparent: MarshalJSON emits T's value directly
// (no extra wrapper in the wire format) and UnmarshalJSON sets dirty=true
// whenever the field appears in the payload -- including explicit null.
package accessor

import "encoding/json"

// Accessor[T] tracks a value of type T and whether it was explicitly set.
// The zero value of Accessor[T] is valid: dirty=false, value=zero(T).
type Accessor[T any] struct {
	dirty bool
	value T
}

// New creates an Accessor[T]. If an initial value is provided, it starts dirty.
func New[T any](val ...T) Accessor[T] {
	if len(val) > 0 {
		return Accessor[T]{dirty: true, value: val[0]}
	}
	return Accessor[T]{}
}

// Get returns the stored value regardless of dirty state.
func (v *Accessor[T]) Get() T {
	return v.value
}

// IsDirty reports whether this field was explicitly set.
func (v *Accessor[T]) IsDirty() bool {
	return v.dirty
}

// Set stores value and marks the field as dirty.
func (v *Accessor[T]) Set(value T) {
	v.dirty = true
	v.value = value
}

// OnDirty calls then with the stored value only if the field is dirty.
func (v *Accessor[T]) OnDirty(then func(T)) {
	if v.dirty {
		then(v.value)
	}
}

// Apply writes the stored value into *ptr only if the field is dirty.
func (v *Accessor[T]) Apply(ptr *T) {
	if v.dirty {
		*ptr = v.value
	}
}

// Sync writes the stored value into dest via dest.Set, only if the field is
// dirty. Unlike Apply, dest is itself an Accessor[T], so the write also marks
// dest as dirty -- useful for propagating a dirty field between two Accessor
// structs (e.g. DTO -> entity) without losing dirty-tracking on dest.
func (v *Accessor[T]) Sync(dest *Accessor[T]) {
	if v.dirty {
		dest.Set(v.value)
	}
}

// MarshalJSON emits the inner value directly (transparent wire format).
func (v Accessor[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

// UnmarshalJSON deserialises data into the inner value and marks it dirty.
// A field whose JSON value is explicit null is still considered dirty.
func (v *Accessor[T]) UnmarshalJSON(data []byte) error {
	v.dirty = true
	return json.Unmarshal(data, &v.value)
}
