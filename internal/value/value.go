// Package value provides Value[T], a generic field wrapper that tracks
// whether its value was explicitly set (dirty-tracking).
//
// The primary use case is PATCH-style handlers: decode a JSON body into a
// struct whose fields are Value[T]; only fields that were present in the
// JSON payload are marked dirty. The handler can then apply only the dirty
// fields to the entity, leaving untouched fields unchanged without needing
// any pointer-based optional semantics.
//
// JSON integration is transparent: MarshalJSON emits T's value directly
// (no extra wrapper in the wire format) and UnmarshalJSON sets dirty=true
// whenever the field appears in the payload -- including explicit null.
package value

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Value[T] tracks a value of type T and whether it was explicitly set.
// The zero value of Value[T] is valid: dirty=false, value=zero(T).
type Value[T any] struct {
	dirty bool
	value T
}

// New creates a Value[T]. If an initial value is provided, it starts dirty.
func New[T any](val ...T) Value[T] {
	if len(val) > 0 {
		return Value[T]{dirty: true, value: val[0]}
	}
	return Value[T]{}
}

// Get returns the stored value regardless of dirty state.
func (v *Value[T]) Get() T {
	return v.value
}

// IsDirty reports whether this field was explicitly set.
func (v *Value[T]) IsDirty() bool {
	return v.dirty
}

// Set stores value and marks the field as dirty.
func (v *Value[T]) Set(value T) {
	v.dirty = true
	v.value = value
}

// OnDirty calls then with the stored value only if the field is dirty.
func (v *Value[T]) OnDirty(then func(T)) {
	if v.dirty {
		then(v.value)
	}
}

// Apply writes the stored value into *ptr only if the field is dirty.
func (v *Value[T]) Apply(ptr *T) {
	if v.dirty {
		*ptr = v.value
	}
}

// GetAny returns the stored value as any. Used internally by ToMap.
func (v *Value[T]) GetAny() any {
	return v.value
}

// MarshalJSON emits the inner value directly (transparent wire format).
func (v Value[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

// UnmarshalJSON deserialises data into the inner value and marks it dirty.
// A field whose JSON value is explicit null is still considered dirty.
func (v *Value[T]) UnmarshalJSON(data []byte) error {
	v.dirty = true
	return json.Unmarshal(data, &v.value)
}

// valueable is the internal interface ToMap uses via reflection to detect
// Value fields without importing the concrete generic type.
type valueable interface {
	GetAny() any
	IsDirty() bool
}

// ToDirtyMap converts a struct (or pointer to struct) into a map[string]any
// containing only the dirty Value[T] fields. The map key is resolved from
// struct tags in priority order: json → file → form → param → query → field
// name. This covers every binding tag the framework's validate package uses,
// so a single struct can carry Value[T] fields bound from any source (JSON
// body, multipart file, form field, path param, or query string) and still
// produce the correct key in the dirty map. Non-Value fields are ignored.
// Returns an empty map for non-struct input.
func ToDirtyMap(obj any) map[string]any {
	out := make(map[string]any)
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return out
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		var ptr reflect.Value
		if field.CanAddr() {
			ptr = field.Addr()
		} else {
			c := reflect.New(field.Type())
			c.Elem().Set(field)
			ptr = c
		}
		m, ok := ptr.Interface().(valueable)
		if !ok || !m.IsDirty() {
			continue
		}
		out[fieldKey(t.Field(i))] = m.GetAny()
	}
	return out
}

// fieldKey resolves the map key for a struct field by checking struct tags
// in priority order: json → file → form → param → query → field name.
// The first non-empty, non-"-" tag value wins. This matches the set of tags
// the rest of the framework uses for binding (validate package).
func fieldKey(f reflect.StructField) string {
	for _, tag := range []string{"json", "file", "form", "param", "query"} {
		v, _, _ := strings.Cut(f.Tag.Get(tag), ",")
		if v != "" && v != "-" {
			return v
		}
	}
	return f.Name
}
