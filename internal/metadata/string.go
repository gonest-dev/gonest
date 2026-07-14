package metadata

// StringMetadata is the branch-specific builder returned by all 10
// string-family branch methods (PropertyBuilder.String/Email/Uuid/Uri/
// Hostname/Ipv4/Ipv6/Password/Byte/Binary, declared in metadata.go). ONE
// type serves all 10 branches because they share identical extra
// validators (Min/Max for length, Pattern for an additional regex
// constraint) -- INSIGHT.md's own comment block lists these once for the
// whole string family, not per-branch. Only the format VALUE differs
// between branches, and that's already resolved by the branch METHOD
// setting PropertyBuilder.format before construction (see
// PropertyBuilder.FormatValue's doc comment), not by this type needing to
// vary.
//
// StringMetadata embeds *PropertyBuilder (a POINTER, the exact same object
// already sitting in Metadata.properties[offset]) rather than copying it --
// this is what makes Required()/Nullable()/Description()/Examples() calls
// made through a StringMetadata value mutate the SAME underlying builder a
// future consumer (Metadata.OwnProperties()) will see.
//
// The 4 common constraint methods below are DELIBERATELY, MANUALLY
// re-declared here instead of relying on Go's automatic method promotion
// through the embedded field. Go WOULD promote PropertyBuilder.Required()
// etc onto StringMetadata for free if left alone -- but a promoted method
// keeps the EMBEDDED type's own return type (*PropertyBuilder), which has
// no Min/Max/Pattern methods, so a chain like `.Required().Min(5)` would
// fail to compile the moment it crossed back from the common method to a
// string-specific one. Re-declaring each method as a one-line wrapper that
// delegates to the embedded PropertyBuilder's own method and then returns
// the *StringMetadata receiver instead is the only way to keep the fluent
// chain working across this base-vs-branch-specific boundary in Go. This is
// the exact mechanical pattern the next branch-family feature (Numeric &
// Boolean Branches) will repeat for its own branch-specific type(s).
type StringMetadata struct {
	*PropertyBuilder
}

// Min sets this field's own minimum length constraint and returns s so
// calls can chain.
func (s *StringMetadata) Min(n int) *StringMetadata {
	s.min = &n
	return s
}

// MinValue returns the minimum length set via Min, and whether Min was ever
// called -- the bool return distinguishes "never called" from "called with
// 0", since 0 is itself a valid minimum length.
func (s *StringMetadata) MinValue() (int, bool) {
	if s.min == nil {
		return 0, false
	}
	return *s.min, true
}

// Max sets this field's own maximum length constraint and returns s so
// calls can chain.
func (s *StringMetadata) Max(n int) *StringMetadata {
	s.max = &n
	return s
}

// MaxValue returns the maximum length set via Max, and whether Max was ever
// called -- same "never called" vs "called with 0" distinction as MinValue.
func (s *StringMetadata) MaxValue() (int, bool) {
	if s.max == nil {
		return 0, false
	}
	return *s.max, true
}

// Pattern sets an additional regex constraint on this field's value and
// returns s so calls can chain. The regex syntax itself is never validated
// here (spec.md's Out of Scope / Edge Cases: "trust the caller", matching
// this metadata system's registration-only stance -- a future runtime
// validation consumer, Milestone 6/7, is the one that would ever compile
// this string).
func (s *StringMetadata) Pattern(p string) *StringMetadata {
	s.pattern = p
	return s
}

// PatternValue returns the regex pattern set via Pattern, or "" if it was
// never called.
func (s *StringMetadata) PatternValue() string {
	return s.pattern
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED object), then returns s (not the embedded
// PropertyBuilder) so Min/Max/Pattern stay chainable afterward -- see this
// type's own doc comment for why this manual re-declaration is necessary
// rather than relying on Go's automatic method promotion.
func (s *StringMetadata) Required() *StringMetadata {
	s.PropertyBuilder.Required()
	return s
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringMetadata) Nullable() *StringMetadata {
	s.PropertyBuilder.Nullable()
	return s
}

// Description delegates to the embedded PropertyBuilder's own Description
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringMetadata) Description(d string) *StringMetadata {
	s.PropertyBuilder.Description(d)
	return s
}

// Examples delegates to the embedded PropertyBuilder's own Examples
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringMetadata) Examples(examples ...any) *StringMetadata {
	s.PropertyBuilder.Examples(examples...)
	return s
}
