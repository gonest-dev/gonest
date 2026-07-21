package schema

// StringSchema is the branch-specific builder returned by all 10
// string-family branch methods (PropertyBuilder.String/Email/Uuid/Uri/
// Hostname/Ipv4/Ipv6/Password/Byte/Binary, declared in schema.go). ONE
// type serves all 10 branches because they share identical extra
// validators (Min/Max for length, Pattern for an additional regex
// constraint) -- INSIGHT.md's own comment block lists these once for the
// whole string family, not per-branch. Only the format VALUE differs
// between branches, and that's already resolved by the branch METHOD
// setting PropertyBuilder.format before construction (see
// PropertyBuilder.FormatValue's doc comment), not by this type needing to
// vary.
//
// StringSchema embeds *PropertyBuilder (a POINTER, the exact same object
// already sitting in Schema.properties[offset]) rather than copying it --
// this is what makes Required()/Nullable()/Description()/Examples() calls
// made through a StringSchema value mutate the SAME underlying builder a
// future consumer (Schema.OwnProperties()) will see.
//
// The 4 common constraint methods below are DELIBERATELY, MANUALLY
// re-declared here instead of relying on Go's automatic method promotion
// through the embedded field. Go WOULD promote PropertyBuilder.Required()
// etc onto StringSchema for free if left alone -- but a promoted method
// keeps the EMBEDDED type's own return type (*PropertyBuilder), which has
// no Min/Max/Pattern methods, so a chain like `.Required().Min(5)` would
// fail to compile the moment it crossed back from the common method to a
// string-specific one. Re-declaring each method as a one-line wrapper that
// delegates to the embedded PropertyBuilder's own method and then returns
// the *StringSchema receiver instead is the only way to keep the fluent
// chain working across this base-vs-branch-specific boundary in Go. This is
// the exact mechanical pattern the next branch-family feature (Numeric &
// Boolean Branches) will repeat for its own branch-specific type(s).
type StringSchema struct {
	*PropertyBuilder
}

// Min sets this field's own minimum length constraint and returns s so
// calls can chain.
func (s *StringSchema) Min(n int) *StringSchema {
	s.min = &n
	return s
}

// MinValue returns the minimum length set via Min, and whether Min was ever
// called -- the bool return distinguishes "never called" from "called with
// 0", since 0 is itself a valid minimum length.
func (s *StringSchema) MinValue() (int, bool) {
	if s.min == nil {
		return 0, false
	}
	return *s.min, true
}

// Max sets this field's own maximum length constraint and returns s so
// calls can chain.
func (s *StringSchema) Max(n int) *StringSchema {
	s.max = &n
	return s
}

// MaxValue returns the maximum length set via Max, and whether Max was ever
// called -- same "never called" vs "called with 0" distinction as MinValue.
func (s *StringSchema) MaxValue() (int, bool) {
	if s.max == nil {
		return 0, false
	}
	return *s.max, true
}

// Pattern sets an additional regex constraint on this field's value and
// returns s so calls can chain. The regex syntax itself is never validated
// here (spec.md's Out of Scope / Edge Cases: "trust the caller", matching
// this schema system's registration-only stance -- a future runtime
// validation consumer, Milestone 6/7, is the one that would ever compile
// this string).
func (s *StringSchema) Pattern(p string) *StringSchema {
	s.pattern = p
	return s
}

// PatternValue returns the regex pattern set via Pattern, or "" if it was
// never called.
func (s *StringSchema) PatternValue() string {
	return s.pattern
}

// Enum restricts this field to the given allowed values and returns s so
// calls can chain. Last-call-wins, no panic, same as every other branch
// method (Min/Max/Pattern) -- calling Enum a second time keeps only the
// latest list. items is stored via a separate enumStringSet flag rather than
// a nil check because a variadic call with zero arguments (`.Enum()`)
// produces a nil slice in Go, indistinguishable from "never called" without
// that flag -- see EnumValues' own doc comment.
func (s *StringSchema) Enum(items ...string) *StringSchema {
	s.enumString = items
	s.enumStringSet = true
	return s
}

// EnumValues returns the allowed-value list set via Enum, and whether Enum
// was ever called -- the bool return distinguishes "never called" from
// "called with 0 items", same "never called" vs "called with 0" distinction
// MinValue/MaxValue already establish for this family.
func (s *StringSchema) EnumValues() ([]string, bool) {
	if !s.enumStringSet {
		return nil, false
	}
	return s.enumString, true
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED object), then returns s (not the embedded
// PropertyBuilder) so Min/Max/Pattern stay chainable afterward -- see this
// type's own doc comment for why this manual re-declaration is necessary
// rather than relying on Go's automatic method promotion.
func (s *StringSchema) Required() *StringSchema {
	s.PropertyBuilder.Required()
	return s
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringSchema) Nullable() *StringSchema {
	s.PropertyBuilder.Nullable()
	return s
}

// Description delegates to the embedded PropertyBuilder's own Description
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringSchema) Description(word string, words ...string) *StringSchema {
	s.PropertyBuilder.Description(word, words...)
	return s
}

// Examples delegates to the embedded PropertyBuilder's own Examples
// (mutating the SHARED object), then returns s. See Required's doc comment
// for why this manual re-declaration exists.
func (s *StringSchema) Examples(examples ...any) *StringSchema {
	s.PropertyBuilder.Examples(examples...)
	return s
}
