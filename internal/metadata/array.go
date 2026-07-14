package metadata

// ArrayMetadata is the branch-specific builder returned by
// PropertyBuilder.Array (metadata.go) -- the first DUAL-STATE branch this
// package has needed. Every prior branch type (StringMetadata,
// NumericMetadata) embeds a SINGLE *PropertyBuilder and adds constraints
// that all describe the SAME field. Array is different: `Tags []string` has
// its own field-level constraints (Required/Nullable/Description/Examples,
// plus how many items the slice may hold) AND its elements have their OWN,
// entirely separate constraints (a string item's own Min/Max length). One
// linear builder can't route two different scopes without ambiguity (see
// spec.md's Problem Statement) -- ArrayMetadata resolves this by holding
// TWO *PropertyBuilder-shaped things and fixing, by definition rather than
// runtime inspection, which methods touch which:
//
//   - PropertyBuilder (embedded, the FIELD, e.g. Tags itself) --
//     Required()/Nullable()/Description()/Examples() ALWAYS mutate this one.
//   - item (unexported, SYNTHETIC -- zero reflect.StructField, never
//     registered in any Metadata.properties map) -- String()/Email()/.../
//     Integer()/.../Boolean()/DateTime()/Date() ALWAYS mutate this one,
//     reusing StringMetadata/NumericMetadata completely unmodified to get
//     the item's own Min/Max/Pattern for free.
//   - itemRef (set instead of item's own format when the element is a
//     nested struct, via Object(ref) -- equivalent to an OpenAPI $ref).
//   - min/max (unexported *int pair) -- the ARRAY's own item-count bounds,
//     entirely separate storage from the item's own Min/Max (which lives on
//     the *StringMetadata/*NumericMetadata wrapper String()/Integer()/etc
//     return) -- see Min/Max's own doc comments below.
//
// The callback shape of Items(fn func(m *ArrayMetadata)) is what makes this
// unambiguous at the call site (design.md's Architecture Overview): every
// method inside the callback is called on the SAME *ArrayMetadata, so which
// state a method mutates is fixed by which method name you wrote, never by
// chain position.
type ArrayMetadata struct {
	*PropertyBuilder
	item    *PropertyBuilder
	itemRef *Metadata
	min     *int
	max     *int
}

// Items invokes fn with am itself (pointer identity -- the SAME
// *ArrayMetadata the caller already holds, not a new type), then returns am
// so the caller can chain quantity Min()/Max() calls after the callback
// (INSIGHT.md's `Items(func(m *gonest.ArrayMetadata) {...}).Min(1)` shape).
// Everything fn does to its own parameter is indistinguishable from doing it
// to am directly -- Items exists purely to give the callback body a name to
// call methods on, and to hand the return value back for post-callback
// chaining.
func (am *ArrayMetadata) Items(fn func(m *ArrayMetadata)) *ArrayMetadata {
	fn(am)
	return am
}

// String selects the bare "string" item format with no format (empty format
// string) on am's SYNTHETIC item builder and returns a *StringMetadata view
// onto it -- mechanical repeat of PropertyBuilder.String's own body, see
// that method's doc comment for the shared branch-method behavior
// (last-call-wins, no panic), applied here to am.item instead of a
// registered field.
func (am *ArrayMetadata) String() *StringMetadata {
	am.item.format = ""
	return &StringMetadata{PropertyBuilder: am.item}
}

// Email selects OpenAPI's "email" string format on the item. See String's
// doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Email() *StringMetadata {
	am.item.format = "email"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Uuid selects OpenAPI's "uuid" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArrayMetadata) Uuid() *StringMetadata {
	am.item.format = "uuid"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Uri selects OpenAPI's "uri" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArrayMetadata) Uri() *StringMetadata {
	am.item.format = "uri"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Hostname selects OpenAPI's "hostname" string format on the item. See
// String's doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Hostname() *StringMetadata {
	am.item.format = "hostname"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Ipv4 selects OpenAPI's "ipv4" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArrayMetadata) Ipv4() *StringMetadata {
	am.item.format = "ipv4"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Ipv6 selects OpenAPI's "ipv6" string format on the item. See String's doc
// comment for the shared branch-method behavior.
func (am *ArrayMetadata) Ipv6() *StringMetadata {
	am.item.format = "ipv6"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Password selects OpenAPI's "password" string format on the item. See
// String's doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Password() *StringMetadata {
	am.item.format = "password"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Byte selects OpenAPI's "byte" string format (base64-encoded) on the item.
// See String's doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Byte() *StringMetadata {
	am.item.format = "byte"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Binary selects OpenAPI's "binary" string format on the item. See String's
// doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Binary() *StringMetadata {
	am.item.format = "binary"
	return &StringMetadata{PropertyBuilder: am.item}
}

// Integer selects OpenAPI's "int64" numeric format on the item and returns
// a *NumericMetadata view onto am.item -- mechanical repeat of
// PropertyBuilder.Integer's own body, see that method's doc comment for the
// shared branch-method behavior (last-call-wins, no panic).
func (am *ArrayMetadata) Integer() *NumericMetadata {
	am.item.format = "int64"
	return &NumericMetadata{PropertyBuilder: am.item}
}

// Int32 selects OpenAPI's "int32" numeric format on the item. See Integer's
// doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Int32() *NumericMetadata {
	am.item.format = "int32"
	return &NumericMetadata{PropertyBuilder: am.item}
}

// Float selects OpenAPI's "float" numeric format on the item. See Integer's
// doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Float() *NumericMetadata {
	am.item.format = "float"
	return &NumericMetadata{PropertyBuilder: am.item}
}

// Double selects OpenAPI's "double" numeric format on the item. See
// Integer's doc comment for the shared branch-method behavior.
func (am *ArrayMetadata) Double() *NumericMetadata {
	am.item.format = "double"
	return &NumericMetadata{PropertyBuilder: am.item}
}

// Boolean selects OpenAPI's "boolean" type on the item -- no format, no
// extra validators, so it returns the bare item *PropertyBuilder itself
// rather than constructing a wrapper type, mechanical repeat of
// PropertyBuilder.Boolean's own body. See that method's doc comment for the
// full rationale.
func (am *ArrayMetadata) Boolean() *PropertyBuilder {
	am.item.format = ""
	return am.item
}

// DateTime selects OpenAPI's "date-time" string format on the item. Like
// Boolean, this family has no extra validators, so it returns the bare item
// *PropertyBuilder. Mechanical repeat of PropertyBuilder.DateTime's own
// body.
func (am *ArrayMetadata) DateTime() *PropertyBuilder {
	am.item.format = "date-time"
	return am.item
}

// Date selects OpenAPI's "date" string format on the item. See DateTime's
// doc comment for why this returns the bare item *PropertyBuilder.
func (am *ArrayMetadata) Date() *PropertyBuilder {
	am.item.format = "date"
	return am.item
}

// Object sets ref as the item's schema (equivalent to an OpenAPI $ref),
// reusing an already-registered *Metadata (INSIGHT.md's
// `addressMetadata := gonest.NewMetadata[AddressEntity](...)`, reused
// verbatim rather than re-walked) instead of the item's own format string,
// and returns am so calls can chain (matching Items(fn)'s own
// return-am-for-chaining shape, even though nothing in INSIGHT.md's own
// examples chains further off an Object item).
func (am *ArrayMetadata) Object(ref *Metadata) *ArrayMetadata {
	am.itemRef = ref
	return am
}

// Min sets the ARRAY's own minimum item-count constraint (distinct from any
// item's own Min, which lives on the *StringMetadata/*NumericMetadata
// wrapper returned by String()/Integer()/etc above) and returns am so calls
// can chain -- INSIGHT.md's `Items(fn).Min(1)` shape, called AFTER the
// callback returns.
func (am *ArrayMetadata) Min(v int) *ArrayMetadata {
	am.min = &v
	return am
}

// MinValue returns the array's own minimum item count set via Min, and
// whether Min was ever called -- same "never called" vs "called with 0"
// distinction as StringMetadata/NumericMetadata's own MinValue.
func (am *ArrayMetadata) MinValue() (int, bool) {
	if am.min == nil {
		return 0, false
	}
	return *am.min, true
}

// Max sets the ARRAY's own maximum item-count constraint and returns am so
// calls can chain. See Min's doc comment for how this differs from the
// item's own Max.
func (am *ArrayMetadata) Max(v int) *ArrayMetadata {
	am.max = &v
	return am
}

// MaxValue returns the array's own maximum item count set via Max, and
// whether Max was ever called -- same "never called" vs "called with 0"
// distinction as MinValue.
func (am *ArrayMetadata) MaxValue() (int, bool) {
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
// Metadata.Property).
func (am *ArrayMetadata) ItemBuilder() *PropertyBuilder {
	return am.item
}

// ItemRef returns the *Metadata set via Object(ref), and whether Object was
// ever called -- same "never called" distinction as MinValue/MaxValue.
func (am *ArrayMetadata) ItemRef() (*Metadata, bool) {
	if am.itemRef == nil {
		return nil, false
	}
	return am.itemRef, true
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED FIELD object, e.g. Tags itself -- NEVER am.item),
// then returns am (not the embedded PropertyBuilder) so Items/Min/Max stay
// chainable afterward -- see StringMetadata's own doc comment (string.go)
// for why this manual re-declaration is necessary rather than relying on Go's
// automatic method promotion.
func (am *ArrayMetadata) Required() *ArrayMetadata {
	am.PropertyBuilder.Required()
	return am
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArrayMetadata) Nullable() *ArrayMetadata {
	am.PropertyBuilder.Nullable()
	return am
}

// Description delegates to the embedded PropertyBuilder's own Description
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArrayMetadata) Description(d string) *ArrayMetadata {
	am.PropertyBuilder.Description(d)
	return am
}

// Examples delegates to the embedded PropertyBuilder's own Examples
// (mutating the SHARED FIELD object, never am.item), then returns am. See
// Required's doc comment for why this manual re-declaration exists.
func (am *ArrayMetadata) Examples(examples ...any) *ArrayMetadata {
	am.PropertyBuilder.Examples(examples...)
	return am
}
