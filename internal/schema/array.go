package schema

// ArraySchema is the branch-specific builder returned by
// PropertyBuilder.Array (schema.go) -- the first DUAL-STATE branch this
// package has needed. Every prior branch type (StringSchema,
// NumericSchema) embeds a SINGLE *PropertyBuilder and adds constraints
// that all describe the SAME field. Array is different: `Tags []string` has
// its own field-level constraints (Required/Nullable/Description/Examples,
// plus how many items the slice may hold) AND its elements have their OWN,
// entirely separate constraints (a string item's own Min/Max length). One
// linear builder can't route two different scopes without ambiguity (see
// spec.md's Problem Statement) -- ArraySchema resolves this by holding
// TWO *PropertyBuilder-shaped things and fixing, by definition rather than
// runtime inspection, which methods touch which:
//
//   - PropertyBuilder (embedded, the FIELD, e.g. Tags itself) --
//     Required()/Nullable()/Description()/Examples() ALWAYS mutate this one.
//   - item (unexported, SYNTHETIC -- zero reflect.StructField, never
//     registered in any Schema.properties map) -- String()/Email()/.../
//     Integer()/.../Boolean()/DateTime()/Date() ALWAYS mutate this one,
//     reusing StringSchema/NumericSchema completely unmodified to get
//     the item's own Min/Max/Pattern for free.
//   - itemRef (set instead of item's own format when the element is a
//     nested struct, via Object(ref) -- equivalent to an OpenAPI $ref).
//   - min/max (unexported *int pair) -- the ARRAY's own item-count bounds,
//     entirely separate storage from the item's own Min/Max (which lives on
//     the *StringSchema/*NumericSchema wrapper String()/Integer()/etc
//     return) -- see Min/Max's own doc comments below.
//
// The callback shape of Items(fn func(m *ArraySchema)) is what makes this
// unambiguous at the call site (design.md's Architecture Overview): every
// method inside the callback is called on the SAME *ArraySchema, so which
// state a method mutates is fixed by which method name you wrote, never by
// chain position.
type ArraySchema struct {
	*PropertyBuilder
	// item is a construction-time SNAPSHOT of PropertyBuilder.item (the
	// canonical, promoted-access copy every item-branch method below also
	// mutates through this same pointer). Kept as ArraySchema's own field
	// -- rather than relying purely on promoted access to
	// PropertyBuilder.item -- so that an EARLIER *ArraySchema returned by
	// a first Array() call keeps observing ITS OWN item builder even after
	// a second Array() call on the same PropertyBuilder reassigns
	// PropertyBuilder.item to a fresh instance (json-body-validation
	// feature's P0 relocation, reconciled against the pre-existing
	// TestArray_CalledTwice_ProducesIndependentItemState regression test:
	// both *ArraySchema values still share the same embedded
	// *PropertyBuilder -- required by TestArray_SetsFormatAndReturnsNewArraySchema
	// -- but each captures its OWN item pointer at the moment Array() was
	// called, exactly mirroring the pre-P0 behavior where item lived only
	// on this wrapper).
	item *PropertyBuilder
}

// Items invokes fn with am itself (pointer identity -- the SAME
// *ArraySchema the caller already holds, not a new type), then returns am
// so the caller can chain quantity Min()/Max() calls after the callback
// (INSIGHT.md's `Items(func(m *gonest.ArraySchema) {...}).Min(1)` shape).
// Everything fn does to its own parameter is indistinguishable from doing it
// to am directly -- Items exists purely to give the callback body a name to
// call methods on, and to hand the return value back for post-callback
// chaining.
func (am *ArraySchema) Items(fn func(m *ArraySchema)) *ArraySchema {
	fn(am)
	return am
}

// String selects the bare "string" item format with no format (empty format
// string) on am's SYNTHETIC item builder and returns a *StringSchema view
// onto it -- mechanical repeat of PropertyBuilder.String's own body, see
// that method's doc comment for the shared branch-method behavior
// (last-call-wins, no panic), applied here to am.item instead of a
// registered field.
func (am *ArraySchema) String() *StringSchema {
	am.item.format = ""
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Email selects OpenAPI's "email" string format on the item. See String's
// doc comment for the shared branch-method behavior.
func (am *ArraySchema) Email() *StringSchema {
	am.item.format = "email"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Uuid selects OpenAPI's "uuid" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArraySchema) Uuid() *StringSchema {
	am.item.format = "uuid"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Uri selects OpenAPI's "uri" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArraySchema) Uri() *StringSchema {
	am.item.format = "uri"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Hostname selects OpenAPI's "hostname" string format on the item. See
// String's doc comment for the shared branch-method behavior.
func (am *ArraySchema) Hostname() *StringSchema {
	am.item.format = "hostname"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Ipv4 selects OpenAPI's "ipv4" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArraySchema) Ipv4() *StringSchema {
	am.item.format = "ipv4"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Ipv6 selects OpenAPI's "ipv6" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArraySchema) Ipv6() *StringSchema {
	am.item.format = "ipv6"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Password selects OpenAPI's "password" string format on the item. See
// String's doc comment for the shared branch-method behavior.
func (am *ArraySchema) Password() *StringSchema {
	am.item.format = "password"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Byte selects OpenAPI's "byte" string format (base64-encoded) on the item.
// See String's doc comment for the shared branch-method behavior.
func (am *ArraySchema) Byte() *StringSchema {
	am.item.format = "byte"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Binary selects OpenAPI's "binary" string format on the item. See String's
// doc comment for the shared branch-method behavior.
func (am *ArraySchema) Binary() *StringSchema {
	am.item.format = "binary"
	am.item.kind = "string"
	return &StringSchema{PropertyBuilder: am.item}
}

// Integer selects OpenAPI's "int64" numeric format on the item and returns
// a *NumericSchema view onto am.item -- mechanical repeat of
// PropertyBuilder.Integer's own body, see that method's doc comment for the
// shared branch-method behavior (last-call-wins, no panic).
func (am *ArraySchema) Integer() *NumericSchema {
	am.item.format = "int64"
	am.item.kind = "integer"
	return &NumericSchema{PropertyBuilder: am.item}
}

// Int32 selects OpenAPI's "int32" numeric format on the item. See Integer's
// doc comment for the shared branch-method behavior.
func (am *ArraySchema) Int32() *NumericSchema {
	am.item.format = "int32"
	am.item.kind = "integer"
	return &NumericSchema{PropertyBuilder: am.item}
}

// Float selects OpenAPI's "float" numeric format on the item. See Integer's
// doc comment for the shared branch-method behavior.
func (am *ArraySchema) Float() *NumericSchema {
	am.item.format = "float"
	am.item.kind = "number"
	return &NumericSchema{PropertyBuilder: am.item}
}

// Double selects OpenAPI's "double" numeric format on the item. See
// Integer's doc comment for the shared branch-method behavior.
func (am *ArraySchema) Double() *NumericSchema {
	am.item.format = "double"
	am.item.kind = "number"
	return &NumericSchema{PropertyBuilder: am.item}
}

// Boolean selects OpenAPI's "boolean" type on the item -- no format, no
// extra validators, so it returns the bare item *PropertyBuilder itself
// rather than constructing a wrapper type, mechanical repeat of
// PropertyBuilder.Boolean's own body. See that method's doc comment for the
// full rationale.
func (am *ArraySchema) Boolean() *PropertyBuilder {
	am.item.format = ""
	am.item.kind = "boolean"
	return am.item
}

// DateTime selects OpenAPI's "date-time" string format on the item. Like
// Boolean, this family has no extra validators, so it returns the bare item
// *PropertyBuilder. Mechanical repeat of PropertyBuilder.DateTime's own
// body.
func (am *ArraySchema) DateTime() *PropertyBuilder {
	am.item.format = "date-time"
	am.item.kind = "string"
	return am.item
}

// Date selects OpenAPI's "date" string format on the item. See DateTime's
// doc comment for why this returns the bare item *PropertyBuilder.
func (am *ArraySchema) Date() *PropertyBuilder {
	am.item.format = "date"
	am.item.kind = "string"
	return am.item
}

// Object sets ref as the item's schema (equivalent to an OpenAPI $ref),
// reusing an already-registered *Schema (INSIGHT.md's
// `addressSchema := gonest.NewSchema[AddressEntity](...)`, reused
// verbatim rather than re-walked) instead of the item's own format string,
// and returns am so calls can chain (matching Items(fn)'s own
// return-am-for-chaining shape, even though nothing in INSIGHT.md's own
// examples chains further off an Object item).
func (am *ArraySchema) Object(ref *Schema) *ArraySchema {
	am.itemRef = ref
	return am
}

// Min sets the ARRAY's own minimum item-count constraint (distinct from any
// item's own Min, which lives on the *StringSchema/*NumericSchema
// wrapper returned by String()/Integer()/etc above) and returns am so calls
// can chain -- INSIGHT.md's `Items(fn).Min(1)` shape, called AFTER the
// callback returns.
func (am *ArraySchema) Min(v int) *ArraySchema {
	am.min = &v
	return am
}

// MinValue returns the array's own minimum item count set via Min, and
// whether Min was ever called -- same "never called" vs "called with 0"
// distinction as StringSchema/NumericSchema's own MinValue.
func (am *ArraySchema) MinValue() (int, bool) {
	if am.min == nil {
		return 0, false
	}
	return *am.min, true
}

// Max sets the ARRAY's own maximum item-count constraint and returns am so
// calls can chain. See Min's doc comment for how this differs from the
// item's own Max.
func (am *ArraySchema) Max(v int) *ArraySchema {
	am.max = &v
	return am
}

// MaxValue returns the array's own maximum item count set via Max, and
// whether Max was ever called -- same "never called" vs "called with 0"
// distinction as MinValue.
func (am *ArraySchema) MaxValue() (int, bool) {
	if am.max == nil {
		return 0, false
	}
	return *am.max, true
}

// ItemBuilder returns am's synthetic item *PropertyBuilder directly --
// needed by a future consumer (Milestone 7's OpenAPI generator) to read the
// item's own FormatValue()/Field()-shaped state. Field() on a synthetic
// item returns a zero reflect.StructField (documented, not fixed here --
// out of scope per spec.md, since the item was never registered via
// Schema.Property).
func (am *ArraySchema) ItemBuilder() *PropertyBuilder {
	return am.item
}

// ItemRef returns the *Schema set via Object(ref), and whether Object was
// ever called -- same "never called" distinction as MinValue/MaxValue.
func (am *ArraySchema) ItemRef() (*Schema, bool) {
	if am.itemRef == nil {
		return nil, false
	}
	return am.itemRef, true
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED FIELD object, e.g. Tags itself -- NEVER am.item),
// then returns am (not the embedded PropertyBuilder) so Items/Min/Max stay
// chainable afterward -- see StringSchema's own doc comment (string.go)
// for why this manual re-declaration is necessary rather than relying on Go's
// automatic method promotion.
func (am *ArraySchema) Required() *ArraySchema {
	am.PropertyBuilder.Required()
	return am
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArraySchema) Nullable() *ArraySchema {
	am.PropertyBuilder.Nullable()
	return am
}

// Description delegates to the embedded PropertyBuilder's own Description
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArraySchema) Description(d string) *ArraySchema {
	am.PropertyBuilder.Description(d)
	return am
}

// Examples delegates to the embedded PropertyBuilder's own Examples
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArraySchema) Examples(examples ...any) *ArraySchema {
	am.PropertyBuilder.Examples(examples...)
	return am
}
