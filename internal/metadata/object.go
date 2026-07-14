package metadata

// ObjectMetadata is the branch-specific builder returned by
// PropertyBuilder.Object (metadata.go). Unlike ArrayMetadata
// (array.go), it is a SINGLE-STATE builder: it embeds the field's own
// *PropertyBuilder directly, the SAME shared object already held in
// Metadata.properties[offset], with no synthetic secondary builder. Array
// needed a dual-state split because an element's own type/format
// (e.g. each string's own Min/Max) is genuinely distinct from the FIELD's
// own type/format -- Object has no such split: the field itself IS the
// nested object (e.g. `Address AddressEntity`), so there is no separate
// "element" to describe. Its callback shape (Object(fn func(om
// *ObjectMetadata))) exists purely to mirror Items(fn)'s API surface (per
// INSIGHT.md), not because it resolves a real dual-scope ambiguity the way
// AD-011 did for Array -- Required()/Nullable()/Description()/Examples()
// called INSIDE the callback and chained OUTSIDE it (on Object(fn)'s own
// return value) mutate the exact same *PropertyBuilder either way.
type ObjectMetadata struct {
	*PropertyBuilder
}

// Metadata reuses an already-registered *Metadata as om's schema
// (equivalent to an OpenAPI $ref -- INSIGHT.md's
// `om.Metadata(addressMetadata)`), storing ref rather than duplicating
// Property, and returns om so calls can chain.
func (om *ObjectMetadata) Metadata(ref *Metadata) *ObjectMetadata {
	om.ref = ref
	return om
}

// MetadataRef returns the *Metadata set via Metadata(ref), and whether
// Metadata was ever called -- same "never called" distinction as
// ArrayMetadata's ItemRef/MinValue/MaxValue.
func (om *ObjectMetadata) MetadataRef() (*Metadata, bool) {
	if om.ref == nil {
		return nil, false
	}
	return om.ref, true
}

// AdditionalProperties marks om's schema as open/free-form (no ref
// associated -- typically for a field with no Go struct to reflect, e.g.
// `Metadata map[string]any`, INSIGHT.md's `om.AdditionalProperties()`) and
// returns om so calls can chain.
func (om *ObjectMetadata) AdditionalProperties() *ObjectMetadata {
	om.additionalProperties = true
	return om
}

// IsAdditionalProperties reports whether AdditionalProperties was called.
func (om *ObjectMetadata) IsAdditionalProperties() bool {
	return om.additionalProperties
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED FIELD object -- the only state ObjectMetadata has,
// unlike ArrayMetadata's field-vs-item split), then returns om (not the
// embedded PropertyBuilder) so calls stay chainable as *ObjectMetadata --
// see ArrayMetadata's own doc comment (array.go) for why this manual
// re-declaration is necessary rather than relying on Go's automatic method
// promotion.
func (om *ObjectMetadata) Required() *ObjectMetadata {
	om.PropertyBuilder.Required()
	return om
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable, then
// returns om. See Required's doc comment for why this manual re-declaration
// exists.
func (om *ObjectMetadata) Nullable() *ObjectMetadata {
	om.PropertyBuilder.Nullable()
	return om
}

// Description delegates to the embedded PropertyBuilder's own Description,
// then returns om. See Required's doc comment for why this manual
// re-declaration exists.
func (om *ObjectMetadata) Description(s string) *ObjectMetadata {
	om.PropertyBuilder.Description(s)
	return om
}

// Examples delegates to the embedded PropertyBuilder's own Examples, then
// returns om. See Required's doc comment for why this manual re-declaration
// exists.
func (om *ObjectMetadata) Examples(examples ...any) *ObjectMetadata {
	om.PropertyBuilder.Examples(examples...)
	return om
}
