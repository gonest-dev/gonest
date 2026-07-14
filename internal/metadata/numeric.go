package metadata

// NumericMetadata is the branch-specific builder returned by all 4
// numeric-family branch methods (PropertyBuilder.Integer/Int32/Float/Double,
// declared in metadata.go). ONE type serves all 4 branches because they
// share identical extra validators (Min/Max only -- INSIGHT.md's own
// comment block lists these once for the whole numeric family, unlike the
// string family there is no regex-equivalent validator here, so this type
// has no Pattern method). Only the format VALUE differs between branches,
// and that's already resolved by the branch METHOD setting
// PropertyBuilder.format before construction (see PropertyBuilder.
// FormatValue's doc comment), not by this type needing to vary.
//
// NumericMetadata embeds *PropertyBuilder (a POINTER, the exact same object
// already sitting in Metadata.properties[offset]) rather than copying it --
// this is what makes Required()/Nullable()/Description()/Examples() calls
// made through a NumericMetadata value mutate the SAME underlying builder a
// future consumer (Metadata.OwnProperties()) will see. This is the exact
// same reasoning StringMetadata's own doc comment already spells out --
// see string.go -- this feature mechanically repeats that settled pattern,
// it does not invent a new one.
//
// The 4 common constraint methods below are DELIBERATELY, MANUALLY
// re-declared here instead of relying on Go's automatic method promotion
// through the embedded field, for the identical reason StringMetadata's own
// doc comment explains: a promoted method keeps the EMBEDDED type's own
// return type (*PropertyBuilder), which has no Min/Max methods, so a chain
// like `.Required().Min(5)` would fail to compile the moment it crossed back
// from the common method to a numeric-specific one.
type NumericMetadata struct {
	*PropertyBuilder
}

// Min sets this field's own minimum value constraint and returns n so calls
// can chain.
func (n *NumericMetadata) Min(v int) *NumericMetadata {
	n.min = &v
	return n
}

// MinValue returns the minimum value set via Min, and whether Min was ever
// called -- the bool return distinguishes "never called" from "called with
// 0", since 0 is itself a valid minimum.
func (n *NumericMetadata) MinValue() (int, bool) {
	if n.min == nil {
		return 0, false
	}
	return *n.min, true
}

// Max sets this field's own maximum value constraint and returns n so calls
// can chain.
func (n *NumericMetadata) Max(v int) *NumericMetadata {
	n.max = &v
	return n
}

// MaxValue returns the maximum value set via Max, and whether Max was ever
// called -- same "never called" vs "called with 0" distinction as MinValue.
func (n *NumericMetadata) MaxValue() (int, bool) {
	if n.max == nil {
		return 0, false
	}
	return *n.max, true
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED object), then returns n (not the embedded
// PropertyBuilder) so Min/Max stay chainable afterward -- see this type's
// own doc comment for why this manual re-declaration is necessary rather
// than relying on Go's automatic method promotion.
func (n *NumericMetadata) Required() *NumericMetadata {
	n.PropertyBuilder.Required()
	return n
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable
// (mutating the SHARED object), then returns n. See Required's doc comment
// for why this manual re-declaration exists.
func (n *NumericMetadata) Nullable() *NumericMetadata {
	n.PropertyBuilder.Nullable()
	return n
}

// Description delegates to the embedded PropertyBuilder's own Description
// (mutating the SHARED object), then returns n. See Required's doc comment
// for why this manual re-declaration exists.
func (n *NumericMetadata) Description(d string) *NumericMetadata {
	n.PropertyBuilder.Description(d)
	return n
}

// Examples delegates to the embedded PropertyBuilder's own Examples
// (mutating the SHARED object), then returns n. See Required's doc comment
// for why this manual re-declaration exists.
func (n *NumericMetadata) Examples(examples ...any) *NumericMetadata {
	n.PropertyBuilder.Examples(examples...)
	return n
}
