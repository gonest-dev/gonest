package schema

// ObjectSchema is the branch-specific builder returned by
// PropertyBuilder.Object (schema.go). Unlike ArraySchema
// (array.go), it is a SINGLE-STATE builder: it embeds the field's own
// *PropertyBuilder directly, the SAME shared object already held in
// Schema.properties[offset], with no synthetic secondary builder. Array
// needed a dual-state split because an element's own type/format
// (e.g. each string's own Min/Max) is genuinely distinct from the FIELD's
// own type/format -- Object has no such split: the field itself IS the
// nested object (e.g. `Address AddressEntity`), so there is no separate
// "element" to describe. Its callback shape (Object(fn func(om
// *ObjectSchema))) exists purely to mirror Items(fn)'s API surface (per
// INSIGHT.md), not because it resolves a real dual-scope ambiguity the way
// AD-011 did for Array -- Required()/Nullable()/Description()/Examples()
// called INSIDE the callback and chained OUTSIDE it (on Object(fn)'s own
// return value) mutate the exact same *PropertyBuilder either way.
type ObjectSchema struct {
	*PropertyBuilder
}

// Schema reuses an already-registered *Schema as om's schema
// (equivalent to an OpenAPI $ref -- INSIGHT.md's
// `om.Schema(addressSchema)`), storing ref rather than duplicating
// Property, and returns om so calls can chain.
func (om *ObjectSchema) Schema(ref *Schema) *ObjectSchema {
	om.ref = ref
	return om
}

// SchemaRef returns the *Schema set via Schema(ref), and whether
// Schema was ever called -- same "never called" distinction as
// ArraySchema's ItemRef/MinValue/MaxValue.
func (om *ObjectSchema) SchemaRef() (*Schema, bool) {
	if om.ref == nil {
		return nil, false
	}
	return om.ref, true
}

// AdditionalProperties marks om's schema as open/free-form (no ref
// associated -- typically for a field with no Go struct to reflect, e.g.
// `Schema map[string]any`, INSIGHT.md's `om.AdditionalProperties()`) and
// returns om so calls can chain.
func (om *ObjectSchema) AdditionalProperties() *ObjectSchema {
	om.additionalProperties = true
	return om
}

// IsAdditionalProperties reports whether AdditionalProperties was called.
func (om *ObjectSchema) IsAdditionalProperties() bool {
	return om.additionalProperties
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED FIELD object -- the only state ObjectSchema has,
// unlike ArraySchema's field-vs-item split), then returns om (not the
// embedded PropertyBuilder) so calls stay chainable as *ObjectSchema --
// see ArraySchema's own doc comment (array.go) for why this manual
// re-declaration is necessary rather than relying on Go's automatic method
// promotion.
func (om *ObjectSchema) Required() *ObjectSchema {
	om.PropertyBuilder.Required()
	return om
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable, then
// returns om. See Required's doc comment for why this manual re-declaration
// exists.
func (om *ObjectSchema) Nullable() *ObjectSchema {
	om.PropertyBuilder.Nullable()
	return om
}

// Description delegates to the embedded PropertyBuilder's own Description,
// then returns om. See Required's doc comment for why this manual
// re-declaration exists.
func (om *ObjectSchema) Description(word string, words ...string) *ObjectSchema {
	om.PropertyBuilder.Description(word, words...)
	return om
}

// Examples delegates to the embedded PropertyBuilder's own Examples, then
// returns om. See Required's doc comment for why this manual re-declaration
// exists.
func (om *ObjectSchema) Examples(examples ...any) *ObjectSchema {
	om.PropertyBuilder.Examples(examples...)
	return om
}
