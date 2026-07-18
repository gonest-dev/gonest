package schema

import "reflect"

// Value is the builder handed to NewValue's callback -- an alias of
// PropertyBuilder since a standalone value needs none of PropertyBuilder's
// struct-field bookkeeping (Field()/its tag lookup are simply never read
// for a Value-schema's own scalar), only its type+format branch methods
// (String/Integer/Boolean/Array/Object) and Min/Max/Pattern/Custom -- all
// already implemented, reused with zero duplication.
type Value = PropertyBuilder

// NewValue constructs a *Schema for a standalone value of type valueType --
// no struct required, unlike New (whose structType.Kind() != reflect.Struct
// panic stays byte-for-byte unchanged; NewValue is a PARALLEL entry point,
// not a flag inside New -- design.md's Tech Decisions, "menor superfície de
// mudança em código já testado"). The returned *PropertyBuilder is m's
// single, implicit property (registered at offset 0), exposed directly to
// the caller so branch methods (String()/Integer()/etc) apply to it exactly
// as Property(&t.Field) already does for a struct field.
//
// Registered in the SAME process-wide registry New uses (Register), keyed
// by valueType -- MustParse[T]/resolveSchema's existing type-comparison
// safety net (internal/validate) needs no change to keep working for a
// Value-schema, since reflect.Type equality is Kind-agnostic.
func NewValue(valueType reflect.Type) (*Schema, *PropertyBuilder) {
	pb := &PropertyBuilder{}
	m := &Schema{
		structType: valueType,
		isValue:    true,
		properties: map[uintptr]*PropertyBuilder{0: pb},
	}
	Register(valueType, m)
	return m, pb
}

// IsValue reports whether m was constructed via NewValue (a standalone
// value schema) rather than New (a struct schema). internal/validate uses
// this to route ParseInto down a value-shaped path instead of the
// struct-field-walking one (validateStruct/populate), since a Value-schema
// has no JSON object wrapping it, no struct tag, and no reflect.StructField
// to key off of -- the raw decoded body IS the value itself.
func (m *Schema) IsValue() bool {
	return m.isValue
}

// ValueProperty returns the single implicit *PropertyBuilder registered by
// NewValue. Panics if m was not built via NewValue -- same defensive shape
// as Property's own mismatch panics, since calling this against a
// struct-shaped Schema is a programmer error (there is no single "the"
// property on a multi-field struct).
func (m *Schema) ValueProperty() *PropertyBuilder {
	if !m.isValue {
		panic("gonest: ValueProperty called on a struct-shaped Schema (built via New, not NewValue)")
	}
	return m.properties[0]
}
