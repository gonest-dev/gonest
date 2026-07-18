// Package schema is the first package of an entirely new domain
// introduced by Milestone 4 (Schema Builder -- Primitivos): schema/
// reflection-shaped declaration, distinct from every package before it in
// this codebase (Milestones 1-3, all DI graph or HTTP dispatch). It holds
// the foundation every later type+format branch feature (String(),
// Integer(), Boolean(), DateTime(), etc. -- see ROADMAP.md) will build on
// top of: a way to declare, per struct field, a set of OpenAPI-3.1-shaped
// constraints WITHOUT struct tags, identified purely by the field's own
// pointer address (INSIGHT.md's `m.Property(&t.Id)` call shape).
//
// This package only builds the REGISTRATION side (a *Schema value that
// HOLDS what was declared, inspectable via accessors) -- it does nothing
// with the registered data yet (no OpenAPI generation, no runtime
// validation; those are Milestones 6-7, see spec.md's Out of Scope).
package schema

import "reflect"

// Schema holds the whole-type description plus every field registered via
// Property, for a single NewSchema[T] call (root package, out of scope
// here -- see design.md's "Architecture Overview": the generic wrapper
// lives at gonest.go, since Go disallows type parameters on methods -- L-001
// in STATE.md -- so this internal type is type-erased, built from a plain
// reflect.Type/uintptr pair rather than a generic T).
type Schema struct {
	structType  reflect.Type
	baseAddr    uintptr
	title       string
	description string
	properties  map[uintptr]*PropertyBuilder // keyed by field offset from baseAddr

	// isValue distinguishes a Schema built via NewValue (schema-value-support
	// feature, value.go) from one built via New (struct-shaped) -- read by
	// internal/validate to route ParseInto down a value-shaped path instead
	// of the struct-field-walking one. See IsValue's own doc comment.
	isValue bool
}

// New constructs a *Schema for structType, whose zero value's address is
// baseAddr. Panics if structType is not a struct Kind -- Property
// fundamentally requires addressable struct fields (spec.md's Edge Cases),
// so a non-struct T can never be usefully registered against.
func New(structType reflect.Type, baseAddr uintptr) *Schema {
	if structType.Kind() != reflect.Struct {
		panic("gonest: NewSchema requires a struct type, got " + structType.Kind().String())
	}
	m := &Schema{
		structType: structType,
		baseAddr:   baseAddr,
		properties: map[uintptr]*PropertyBuilder{},
	}
	Register(structType, m)
	return m
}

// Title sets the whole-type title (the struct itself, not any individual
// field -- same tier as Description) and returns m so calls can chain. Used
// as the components.schemas key + schema "title" field by the OpenAPI
// generator; when never called, TitleText returns "" and the generator falls
// back to the Go type's own name (spec.md AC1 -- that fallback is the
// generator's job, not Schema's).
func (m *Schema) Title(s string) *Schema {
	m.title = s
	return m
}

// TitleText returns the whole-type title set via Title, or "" if it was
// never called. Named differently from the setter for the same reason as
// Description/DescriptionText -- Go has no method overloading.
func (m *Schema) TitleText() string {
	return m.title
}

// StructType returns the reflect.Type m was constructed for (New's
// structType argument). SPEC_DEVIATION (schema-generation feature, T2):
// design.md's registerSchema component needs a default components.schemas
// name (the Go type's own name) when TitleText() is "" -- Schema had no
// existing accessor exposing structType, only using it internally
// (Property's findFieldByOffset). Adding this minimal read-only getter
// (same defensive shape as every other Schema accessor) is the smallest
// change that unblocks that requirement, rather than deriving the type name
// indirectly through a PropertyBuilder's Field().
func (m *Schema) StructType() reflect.Type {
	return m.structType
}

// Description sets the whole-type description (the struct itself, not any
// individual field -- see PropertyBuilder.Description for the field-level
// equivalent) and returns m so calls can chain.
func (m *Schema) Description(s string) *Schema {
	m.description = s
	return m
}

// DescriptionText returns the whole-type description set via Description,
// or "" if it was never called. Named differently from the setter because
// Go has no method overloading -- same setter/getter split already
// established by internal/route/route.go's HttpCode(status)/Code().
func (m *Schema) DescriptionText() string {
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
func (m *Schema) Property(fieldPtr any) *PropertyBuilder {
	fieldAddr := reflect.ValueOf(fieldPtr).Pointer()
	offset := fieldAddr - m.baseAddr

	if _, exists := m.properties[offset]; exists {
		panic("gonest: field already registered via Property")
	}

	field, ok := findFieldByOffset(m.structType, offset)
	if !ok {
		panic("gonest: Property(...) pointer does not belong to the type passed to NewSchema")
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
// Schema's internal state (same defensive-copy pattern as
// Controller.OwnMiddleware/Module.OwnProviders).
func (m *Schema) OwnProperties() []*PropertyBuilder {
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

	// P0 additions (json-body-validation feature) -- permanent home for
	// every branch's own constraints, relocated off each disposable
	// wrapper type (StringSchema/NumericSchema/ArraySchema/
	// ObjectSchema) so a future consumer can read them back through
	// PropertyBuilder alone, without needing the original wrapper
	// reference (see design.md's Components table). Reused across
	// branch families since only ONE family is ever active per
	// PropertyBuilder instance (format/kind determine which).
	kind                 string           // OpenAPI "type": string/integer/number/boolean/array/object
	min, max             *int             // String length / Numeric value / Array quantity
	pattern              string           // String only
	item                 *PropertyBuilder // Array's synthetic item builder
	itemRef              *Schema          // Array's Object(ref)-as-item
	ref                  *Schema          // Object's own Schema(ref)
	additionalProperties bool             // Object's own AdditionalProperties()

	// custom is the param-query-validation feature's P0 escape hatch (see
	// that feature's context.md Decision 4): when set via Custom, it fully
	// REPLACES the built-in kind/format/Min/Max/Pattern checks for this
	// field, and its returned value (not the raw decoded value) is what
	// ultimately populates T's field.
	custom func(raw any) (any, error)
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
// Schema.Description, which sets the whole-type description) and returns
// p so calls can chain.
func (p *PropertyBuilder) Description(s string) *PropertyBuilder {
	p.description = s
	return p
}

// DescriptionText returns the field description set via Description, or ""
// if it was never called. Named differently from the setter for the same
// reason as Schema.Description/DescriptionText -- Go has no method
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
// PropertyBuilder that Schema.properties[offset] already holds -- not on
// the disposable *StringSchema wrapper each branch method constructs
// fresh -- because that wrapper is discarded the instant a dev doesn't keep
// chaining off it, while p itself is the one object a future consumer
// (Schema.OwnProperties(), Milestone 7's OpenAPI generator) actually has
// access to (string-family-branches feature's design.md, Tech Decisions:
// "storing format on the shared object is the ONLY way the choice survives
// past the branch call itself").
func (p *PropertyBuilder) FormatValue() string {
	return p.format
}

// String selects the bare "string" OpenAPI type with no format (empty
// format string) and returns a *StringSchema view onto p -- the first of
// the 10 string-family branch methods (string-family-branches feature).
// Calling String()/Email()/etc a second time on the same p simply overwrites
// p.format (last call wins, no panic) -- see this method group's package
// doc / design.md's Error Handling Strategy: branch selection isn't field
// registration, so it isn't held to Property's own stricter
// double-registration panic.
func (p *PropertyBuilder) String() *StringSchema {
	p.format = ""
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Email selects OpenAPI's "email" string format. See String's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Email() *StringSchema {
	p.format = "email"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Uuid selects OpenAPI's "uuid" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Uuid() *StringSchema {
	p.format = "uuid"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Uri selects OpenAPI's "uri" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Uri() *StringSchema {
	p.format = "uri"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Hostname selects OpenAPI's "hostname" string format. See String's doc
// comment for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Hostname() *StringSchema {
	p.format = "hostname"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Ipv4 selects OpenAPI's "ipv4" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Ipv4() *StringSchema {
	p.format = "ipv4"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Ipv6 selects OpenAPI's "ipv6" string format. See String's doc comment for
// the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Ipv6() *StringSchema {
	p.format = "ipv6"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Password selects OpenAPI's "password" string format. See String's doc
// comment for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Password() *StringSchema {
	p.format = "password"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Byte selects OpenAPI's "byte" string format (base64-encoded). See
// String's doc comment for the shared branch-method behavior
// (last-call-wins, no panic).
func (p *PropertyBuilder) Byte() *StringSchema {
	p.format = "byte"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Binary selects OpenAPI's "binary" string format. See String's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Binary() *StringSchema {
	p.format = "binary"
	p.kind = "string"
	return &StringSchema{PropertyBuilder: p}
}

// Integer selects OpenAPI's "int64" numeric format (the default width for a
// bare "integer" per INSIGHT.md's own comment: "format: int64 default") and
// returns a *NumericSchema view onto p -- the first of the 4
// numeric-family branch methods (numeric-boolean-branches feature). Calling
// Integer()/Int32()/etc a second time on the same p simply overwrites
// p.format (last call wins, no panic), same precedent String's doc comment
// already established for the string family.
func (p *PropertyBuilder) Integer() *NumericSchema {
	p.format = "int64"
	p.kind = "integer"
	return &NumericSchema{PropertyBuilder: p}
}

// Int32 selects OpenAPI's "int32" numeric format. See Integer's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Int32() *NumericSchema {
	p.format = "int32"
	p.kind = "integer"
	return &NumericSchema{PropertyBuilder: p}
}

// Float selects OpenAPI's "float" numeric format. See Integer's doc comment
// for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Float() *NumericSchema {
	p.format = "float"
	p.kind = "number"
	return &NumericSchema{PropertyBuilder: p}
}

// Double selects OpenAPI's "double" numeric format. See Integer's doc
// comment for the shared branch-method behavior (last-call-wins, no panic).
func (p *PropertyBuilder) Double() *NumericSchema {
	p.format = "double"
	p.kind = "number"
	return &NumericSchema{PropertyBuilder: p}
}

// Boolean selects OpenAPI's "boolean" type, which per INSIGHT.md's own
// comment ("Boolean() -> sem format") has NO format string and NO extra
// validators at all -- the first branch method this codebase has needed
// that returns the BARE *PropertyBuilder itself rather than constructing a
// branch-specific wrapper type. Every other branch method above (String,
// Email, ..., Integer, Int32, ...) exists to carry a format string plus
// extra validators (Min/Max/Pattern) that PropertyBuilder itself doesn't
// have -- Boolean has neither, so a "BooleanSchema" wrapper with zero
// extra fields or methods would add a type purely for consistency's own
// sake, not because it does anything a caller couldn't already do through
// PropertyBuilder directly. Boolean still explicitly sets p.format = ""
// (rather than leaving whatever was already there) so it "claims" the
// format slot the same way every other branch method does -- this matters
// if a dev calls e.g. `.Integer().Boolean()` by mistake, since a
// last-write-wins Boolean() must leave FormatValue() reporting "", not a
// stale "int64" from the earlier call (see design.md's Tech Decisions for
// numeric-boolean-branches).
func (p *PropertyBuilder) Boolean() *PropertyBuilder {
	p.format = ""
	p.kind = "boolean"
	return p
}

// DateTime selects OpenAPI's "date-time" string format (RFC 3339
// timestamp -- INSIGHT.md's UserEntity example uses this for CreatedAt/
// UpdatedAt/DeletedAt, all time.Time/*time.Time fields). Like Boolean, this
// family has no extra validators beyond Required/Nullable/Description/
// Examples (INSIGHT.md documents none for the date/date-time branches), so
// DateTime returns the BARE *PropertyBuilder itself rather than constructing
// a disposable wrapper type -- same reasoning as Boolean's own doc comment.
func (p *PropertyBuilder) DateTime() *PropertyBuilder {
	p.format = "date-time"
	p.kind = "string"
	return p
}

// Date selects OpenAPI's "date" string format (calendar date only, no time
// component). See DateTime's doc comment for why this returns the bare
// *PropertyBuilder rather than a wrapper type.
func (p *PropertyBuilder) Date() *PropertyBuilder {
	p.format = "date"
	p.kind = "string"
	return p
}

// Array selects OpenAPI's "array" type and returns a brand new
// *ArraySchema (array.go, array-builder feature) -- the first branch
// method whose extra state can't be captured by simply wrapping p itself
// (see String/Integer/Boolean above): an array needs a SEPARATE builder for
// its own items (Tags []string's element is itself a string with its own
// Min/Max), distinct from p's own field-level Required/Nullable/
// Description/Examples. p.format is set to "array" here (same
// last-call-wins precedent as every other branch method), and a fresh
// SYNTHETIC item *PropertyBuilder is allocated every call -- calling
// Array() twice on the same p discards whatever item state the first
// *ArraySchema had (see array.go's own doc comment for the full
// rationale).
func (p *PropertyBuilder) Array() *ArraySchema {
	p.format = "array"
	p.kind = "array"
	p.item = &PropertyBuilder{}
	p.itemRef = nil
	p.min = nil
	p.max = nil
	// NOTE (SPEC_DEVIATION from tasks.md T0 item 3's literal
	// `return &ArraySchema{PropertyBuilder: p}`): also captures p.item
	// into the returned ArraySchema's OWN item field (see array.go's
	// ArraySchema.item doc comment) -- required to keep an EARLIER
	// *ArraySchema's ItemBuilder() independent of a LATER Array() call on
	// the same PropertyBuilder, per the pre-existing
	// TestArray_CalledTwice_ProducesIndependentItemState regression test,
	// which the literal task instruction (no item: in the literal) would
	// otherwise break.
	return &ArraySchema{PropertyBuilder: p, item: p.item}
}

// Object selects OpenAPI's "object" type and returns a brand new
// *ObjectSchema wrapping p itself (object.go, object-builder feature) --
// unlike Array, Object has no separate "item" concept: the field itself IS
// the nested object (e.g. `Address AddressEntity`), so there is no second
// synthetic *PropertyBuilder to allocate here, just p.format = "object" and
// a fresh *ObjectSchema view onto the SAME p (see object.go's own doc
// comment for the full rationale). fn is invoked with that same
// *ObjectSchema (pointer identity, same callback shape as
// ArraySchema.Items(fn) for API-surface consistency per INSIGHT.md), and
// Object returns it so the caller can keep chaining Required()/Nullable()/
// etc after the callback returns (INSIGHT.md's
// `Object(fn).Nullable().Description(...)` shape).
func (p *PropertyBuilder) Object(fn func(om *ObjectSchema)) *ObjectSchema {
	p.format = "object"
	p.kind = "object"
	p.ref = nil
	p.additionalProperties = false
	om := &ObjectSchema{PropertyBuilder: p}
	fn(om)
	return om
}

// KindValue returns the OpenAPI "type" dimension set by whichever branch
// method (String/.../Boolean/DateTime/.../Array/Object) was last called on
// p, or "" if none was ever called. kind is orthogonal to format (see the
// PropertyBuilder.kind field's own doc comment) -- this is what
// distinguishes a String() field from a Boolean() field, both of which
// leave FormatValue() == "" (json-body-validation feature's P0 fix for a
// pre-existing collision).
func (p *PropertyBuilder) KindValue() string {
	return p.kind
}

// MinValue returns the minimum constraint set via a branch's own Min call
// (String length / Numeric value / Array item count -- whichever family is
// active on p), and whether Min was ever called -- the bool return
// distinguishes "never called" from "called with 0", since 0 is itself a
// valid minimum. Relocated here (json-body-validation's P0) so a future
// consumer can read it back without needing the original StringSchema/
// NumericSchema/ArraySchema wrapper reference.
func (p *PropertyBuilder) MinValue() (int, bool) {
	if p.min == nil {
		return 0, false
	}
	return *p.min, true
}

// MaxValue returns the maximum constraint set via a branch's own Max call,
// and whether Max was ever called -- same "never called" vs "called with 0"
// distinction as MinValue. See MinValue's doc comment for why this lives on
// PropertyBuilder now.
func (p *PropertyBuilder) MaxValue() (int, bool) {
	if p.max == nil {
		return 0, false
	}
	return *p.max, true
}

// PatternValue returns the regex pattern set via a String-family branch's
// own Pattern call, or "" if it was never called. See MinValue's doc
// comment for why this lives on PropertyBuilder now.
func (p *PropertyBuilder) PatternValue() string {
	return p.pattern
}

// ItemBuilder returns p's synthetic item *PropertyBuilder (set by Array),
// or nil if Array was never called. See MinValue's doc comment for why this
// lives on PropertyBuilder now.
func (p *PropertyBuilder) ItemBuilder() *PropertyBuilder {
	return p.item
}

// ItemRef returns the *Schema set via ArraySchema.Object(ref), and
// whether it was ever called -- same "never called" distinction as
// MinValue/MaxValue. See MinValue's doc comment for why this lives on
// PropertyBuilder now.
func (p *PropertyBuilder) ItemRef() (*Schema, bool) {
	if p.itemRef == nil {
		return nil, false
	}
	return p.itemRef, true
}

// SchemaRef returns the *Schema set via ObjectSchema.Schema(ref),
// and whether it was ever called -- same "never called" distinction as
// MinValue/MaxValue. See MinValue's doc comment for why this lives on
// PropertyBuilder now.
func (p *PropertyBuilder) SchemaRef() (*Schema, bool) {
	if p.ref == nil {
		return nil, false
	}
	return p.ref, true
}

// IsAdditionalProperties reports whether ObjectSchema.AdditionalProperties
// was ever called. See MinValue's doc comment for why this lives on
// PropertyBuilder now.
func (p *PropertyBuilder) IsAdditionalProperties() bool {
	return p.additionalProperties
}

// Custom sets fn as this field's escape hatch, replacing every built-in
// kind/format/Min/Max/Pattern check the validator would otherwise run for
// this field (param-query-validation feature, context.md's Decision 4 --
// the direct replacement for the removed Pipe mechanism). Returns p bare, no
// wrapper type -- same precedent as Boolean()/DateTime(): Custom has no
// format-specific extra validators of its own to chain off of.
//
// fn may be called MORE THAN ONCE per request for the same field (once
// during validation, once during population -- see internal/validate's
// design doc for why this isn't cached) and must therefore be idempotent:
// calling it twice with the same raw input must produce the same result.
//
// Last-call-wins, no panic, same as every other branch method on
// PropertyBuilder -- calling Custom a second time simply overwrites the
// first fn.
func (p *PropertyBuilder) Custom(fn func(raw any) (any, error)) *PropertyBuilder {
	p.custom = fn
	return p
}

// CustomFunc returns the function set via Custom, and whether Custom was
// ever called -- same "never called" bool-return shape as MinValue/MaxValue/
// ItemRef/SchemaRef.
func (p *PropertyBuilder) CustomFunc() (func(raw any) (any, error), bool) {
	if p.custom == nil {
		return nil, false
	}
	return p.custom, true
}
