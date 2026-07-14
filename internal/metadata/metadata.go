// Package metadata is the first package of an entirely new domain
// introduced by Milestone 4 (Metadata Builder -- Primitivos): schema/
// reflection-shaped declaration, distinct from every package before it in
// this codebase (Milestones 1-3, all DI graph or HTTP dispatch). It holds
// the foundation every later type+format branch feature (String(),
// Integer(), Boolean(), DateTime(), etc. -- see ROADMAP.md) will build on
// top of: a way to declare, per struct field, a set of OpenAPI-3.1-shaped
// constraints WITHOUT struct tags, identified purely by the field's own
// pointer address (INSIGHT.md's `m.Property(&t.Id)` call shape).
//
// This package only builds the REGISTRATION side (a *Metadata value that
// HOLDS what was declared, inspectable via accessors) -- it does nothing
// with the registered data yet (no OpenAPI generation, no runtime
// validation; those are Milestones 6-7, see spec.md's Out of Scope).
package metadata

import "reflect"

// Metadata holds the whole-type description plus every field registered via
// Property, for a single NewMetadata[T] call (root package, out of scope
// here -- see design.md's "Architecture Overview": the generic wrapper
// lives at gonest.go, since Go disallows type parameters on methods -- L-001
// in STATE.md -- so this internal type is type-erased, built from a plain
// reflect.Type/uintptr pair rather than a generic T).
type Metadata struct {
	structType  reflect.Type
	baseAddr    uintptr
	description string
	properties  map[uintptr]*PropertyBuilder // keyed by field offset from baseAddr
}

// New constructs a *Metadata for structType, whose zero value's address is
// baseAddr. Panics if structType is not a struct Kind -- Property
// fundamentally requires addressable struct fields (spec.md's Edge Cases),
// so a non-struct T can never be usefully registered against.
func New(structType reflect.Type, baseAddr uintptr) *Metadata {
	if structType.Kind() != reflect.Struct {
		panic("gonest: NewMetadata requires a struct type, got " + structType.Kind().String())
	}
	return &Metadata{
		structType: structType,
		baseAddr:   baseAddr,
		properties: map[uintptr]*PropertyBuilder{},
	}
}

// Description sets the whole-type description (the struct itself, not any
// individual field -- see PropertyBuilder.Description for the field-level
// equivalent) and returns m so calls can chain.
func (m *Metadata) Description(s string) *Metadata {
	m.description = s
	return m
}

// DescriptionText returns the whole-type description set via Description,
// or "" if it was never called. Named differently from the setter because
// Go has no method overloading -- same setter/getter split already
// established by internal/route/route.go's HttpCode(status)/Code().
func (m *Metadata) DescriptionText() string {
	return m.description
}

// Property identifies WHICH field of the type m was built for is being
// referenced by fieldPtr, using the offset between fieldPtr's own address
// and m.baseAddr (design.md's "Field identification algorithm" -- the core,
// non-obvious mechanism this whole feature depends on, empirically
// confirmed by this package's own test suite per design.md's Tech
// Decisions table: "not independently re-verified via external docs this
// session ... T1's own test suite is what proves it actually works").
//
// Panics if fieldPtr's offset does not match any field of the type (a
// pointer that does not belong to the value m was built for -- spec.md
// AC3), or if that offset was already registered by an earlier Property
// call (spec.md's Edge Cases: panic chosen over silent merge, since merge
// semantics -- does a second Required() call OVERRIDE or ADD to the
// first's -- are genuinely ambiguous and INSIGHT.md never demonstrates
// this case).
func (m *Metadata) Property(fieldPtr any) *PropertyBuilder {
	fieldAddr := reflect.ValueOf(fieldPtr).Pointer()
	offset := fieldAddr - m.baseAddr

	if _, exists := m.properties[offset]; exists {
		panic("gonest: field already registered via Property")
	}

	field, ok := findFieldByOffset(m.structType, offset)
	if !ok {
		panic("gonest: Property(...) pointer does not belong to the type passed to NewMetadata")
	}

	pb := &PropertyBuilder{field: field}
	m.properties[offset] = pb
	return pb
}

// findFieldByOffset searches t's own visible fields (reflect.VisibleFields,
// which also flattens embedded fields) for the one whose Offset matches
// offset.
func findFieldByOffset(t reflect.Type, offset uintptr) (reflect.StructField, bool) {
	for _, f := range reflect.VisibleFields(t) {
		if f.Offset == offset {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// OwnProperties returns a copy of every PropertyBuilder registered so far
// via Property. Read-only: mutating the returned slice does not affect this
// Metadata's internal state (same defensive-copy pattern as
// Controller.OwnMiddleware/Module.OwnProviders).
func (m *Metadata) OwnProperties() []*PropertyBuilder {
	out := make([]*PropertyBuilder, 0, len(m.properties))
	for _, pb := range m.properties {
		out = append(out, pb)
	}
	return out
}

// PropertyBuilder holds one field's own constraints -- Required/Nullable/
// Description/Examples in this feature; future type+format branch features
// (String(), Integer(), etc. -- see ROADMAP.md, explicitly out of scope
// here) add their own methods on top, most likely by embedding
// *PropertyBuilder in a branch-specific type (design.md's Tech Decisions).
type PropertyBuilder struct {
	field       reflect.StructField
	required    bool
	nullable    bool
	description string
	examples    []any
	format      string
}

// Required marks this field as required and returns p so calls can chain.
func (p *PropertyBuilder) Required() *PropertyBuilder {
	p.required = true
	return p
}

// IsRequired reports whether Required was called.
func (p *PropertyBuilder) IsRequired() bool {
	return p.required
}

// Nullable marks this field as nullable and returns p so calls can chain.
func (p *PropertyBuilder) Nullable() *PropertyBuilder {
	p.nullable = true
	return p
}

// IsNullable reports whether Nullable was called.
func (p *PropertyBuilder) IsNullable() bool {
	return p.nullable
}

// Description sets this field's own description (distinct from
// Metadata.Description, which sets the whole-type description) and returns
// p so calls can chain.
func (p *PropertyBuilder) Description(s string) *PropertyBuilder {
	p.description = s
	return p
}

// DescriptionText returns the field description set via Description, or ""
// if it was never called. Named differently from the setter for the same
// reason as Metadata.Description/DescriptionText -- Go has no method
// overloading.
func (p *PropertyBuilder) DescriptionText() string {
	return p.description
}

// Examples stores examples as this field's own example values and returns p
// so calls can chain.
func (p *PropertyBuilder) Examples(examples ...any) *PropertyBuilder {
	p.examples = append([]any(nil), examples...)
	return p
}

// ExamplesList returns a copy of the examples set via Examples. Read-only:
// mutating the returned slice does not affect this PropertyBuilder's
// internal state (same defensive-copy pattern as OwnProperties).
func (p *PropertyBuilder) ExamplesList() []any {
	return append([]any(nil), p.examples...)
}

// Field returns the reflect.StructField this builder was registered for --
// needed by later branch/OpenAPI/validation features to know the field's
// own Go type, json tag, and name.
func (p *PropertyBuilder) Field() reflect.StructField {
	return p.field
}

// FormatValue returns the OpenAPI 3.1 format string set by whichever
// type+format branch method (String/Email/Uuid/... below) was last called on
// p, or "" if none was ever called. format is stored HERE, on the SHARED
// PropertyBuilder that Metadata.properties[offset] already holds -- not on
// the disposable *StringMetadata wrapper each branch method constructs
// fresh -- because that wrapper is discarded the instant a dev doesn't keep
// chaining off it, while p itself is the one object a future consumer
// (Metadata.OwnProperties(), Milestone 7's OpenAPI generator) actually has
// access to (string-family-branches feature's design.md, Tech Decisions:
// "storing format on the shared object is the ONLY way the choice survives
// past the branch call itself").
func (p *PropertyBuilder) FormatValue() string {
	return p.format
}

// String selects the bare "string" OpenAPI type with no format (empty
// format string) and returns a *StringMetadata view onto p -- the first of
// the 10 string-family branch methods (string-family-branches feature).
// Calling String()/Email()/etc a second time on the same p simply overwrites
// p.format (last call wins, no panic) -- see this method group's package
// doc / design.md's Error Handling Strategy: branch selection isn't field
// registration, so it isn't held to Property's own stricter
// double-registration panic.
func (p *PropertyBuilder) String() *StringMetadata {
	p.format = ""
	return &StringMetadata{PropertyBuilder: p}
}

// Email selects OpenAPI's "email" string format. See String's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Email() *StringMetadata {
	p.format = "email"
	return &StringMetadata{PropertyBuilder: p}
}

// Uuid selects OpenAPI's "uuid" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Uuid() *StringMetadata {
	p.format = "uuid"
	return &StringMetadata{PropertyBuilder: p}
}

// Uri selects OpenAPI's "uri" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Uri() *StringMetadata {
	p.format = "uri"
	return &StringMetadata{PropertyBuilder: p}
}

// Hostname selects OpenAPI's "hostname" string format. See String's doc
// comment for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Hostname() *StringMetadata {
	p.format = "hostname"
	return &StringMetadata{PropertyBuilder: p}
}

// Ipv4 selects OpenAPI's "ipv4" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Ipv4() *StringMetadata {
	p.format = "ipv4"
	return &StringMetadata{PropertyBuilder: p}
}

// Ipv6 selects OpenAPI's "ipv6" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Ipv6() *StringMetadata {
	p.format = "ipv6"
	return &StringMetadata{PropertyBuilder: p}
}

// Password selects OpenAPI's "password" string format. See String's doc
// comment for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Password() *StringMetadata {
	p.format = "password"
	return &StringMetadata{PropertyBuilder: p}
}

// Byte selects OpenAPI's "byte" string format (base64-encoded). See
// String's doc comment for the shared branch-method behavior
// (last-call-wins, no panic).
func (p *PropertyBuilder) Byte() *StringMetadata {
	p.format = "byte"
	return &StringMetadata{PropertyBuilder: p}
}

// Binary selects OpenAPI's "binary" string format. See String's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Binary() *StringMetadata {
	p.format = "binary"
	return &StringMetadata{PropertyBuilder: p}
}
