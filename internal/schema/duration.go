package schema

import "time"

// DurationSchema is the branch-specific builder returned by
// PropertyBuilder.Duration (declared in schema.go). Same embed/redeclare
// pattern as NumericSchema (see numeric.go's doc comment): Min/Max/Enum are
// numeric under the hood -- nanoseconds, stored via the SAME p.min/p.max/
// p.enumInt fields NumericSchema already uses -- but spelled in
// time.Duration for callers, since a Duration()-branch field's external
// representation is a Go-formatted duration STRING ("5s", "1h30m"), not a
// JSON number, unlike the numeric family.
type DurationSchema struct {
	*PropertyBuilder
}

// Min sets this field's own minimum duration constraint and returns d so
// calls can chain.
func (d *DurationSchema) Min(v time.Duration) *DurationSchema {
	n := int(v)
	d.min = &n
	return d
}

// MinValue returns the minimum duration set via Min, and whether Min was
// ever called -- same "never called" vs "called with 0" distinction as
// NumericSchema.MinValue.
func (d *DurationSchema) MinValue() (time.Duration, bool) {
	if d.min == nil {
		return 0, false
	}
	return time.Duration(*d.min), true
}

// Max sets this field's own maximum duration constraint and returns d so
// calls can chain.
func (d *DurationSchema) Max(v time.Duration) *DurationSchema {
	n := int(v)
	d.max = &n
	return d
}

// MaxValue returns the maximum duration set via Max, and whether Max was
// ever called -- same "never called" vs "called with 0" distinction as
// NumericSchema.MaxValue.
func (d *DurationSchema) MaxValue() (time.Duration, bool) {
	if d.max == nil {
		return 0, false
	}
	return time.Duration(*d.max), true
}

// Enum restricts this field to the given allowed durations and returns d so
// calls can chain. Last-call-wins, no panic, same as every other branch
// method. Stored via the SAME p.enumInt/p.enumIntSet fields NumericSchema
// already uses.
func (d *DurationSchema) Enum(items ...time.Duration) *DurationSchema {
	ints := make([]int64, len(items))
	for i, v := range items {
		ints[i] = int64(v)
	}
	d.enumInt = ints
	d.enumIntSet = true
	return d
}

// EnumValues returns the allowed-duration list set via Enum, and whether
// Enum was ever called -- same "never called" vs "called with 0 items"
// distinction as MinValue/MaxValue.
func (d *DurationSchema) EnumValues() ([]time.Duration, bool) {
	if !d.enumIntSet {
		return nil, false
	}
	out := make([]time.Duration, len(d.enumInt))
	for i, v := range d.enumInt {
		out[i] = time.Duration(v)
	}
	return out, true
}

// Required delegates to the embedded PropertyBuilder's own Required
// (mutating the SHARED object), then returns d (not the embedded
// PropertyBuilder) so Min/Max/Enum stay chainable afterward -- same reason
// NumericSchema redeclares it.
func (d *DurationSchema) Required() *DurationSchema {
	d.PropertyBuilder.Required()
	return d
}

// Nullable delegates to the embedded PropertyBuilder's own Nullable. See
// Required's doc comment for why this manual redeclaration exists.
func (d *DurationSchema) Nullable() *DurationSchema {
	d.PropertyBuilder.Nullable()
	return d
}

// Description delegates to the embedded PropertyBuilder's own Description.
// See Required's doc comment for why this manual redeclaration exists.
func (d *DurationSchema) Description(word string, words ...string) *DurationSchema {
	d.PropertyBuilder.Description(word, words...)
	return d
}

// Examples delegates to the embedded PropertyBuilder's own Examples. See
// Required's doc comment for why this manual redeclaration exists.
func (d *DurationSchema) Examples(examples ...any) *DurationSchema {
	d.PropertyBuilder.Examples(examples...)
	return d
}
