// Package validate is the real implementation behind the public root
// gonest.MustJsonBody[T] wrapper (Go cannot re-export a generic function via
// var, see AD-004 -- same reasoning as inject.MustInject). It reads
// *execution.Request's raw request body, validates it against T's
// registered *schema.Schema (internal/schema), and panics
// *exception.BadRequestException on any violation -- this package is a thin
// cross-cutting layer over internal/execution + internal/schema +
// internal/exception + internal/route (for MustParams/MustQuery's
// route.HasParam lookups), none of which import each other back (design.md's
// Architecture Overview).
package validate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// violation is one field-level validation failure -- the shape every
// recursive core function below collects into (context.md's Decision 2:
// collect ALL violations, not fail-fast).
//
// Kept unexported (not exported as "Violation"): BadRequestException's
// details is a plain `any` (internal/exception/builtin.go), and every
// consumer of MustJsonBody's failure path (the fiber-adapter's panic
// recovery in RegisterRoute, an end user's own recover()) only ever needs
// to json.Marshal Details() back to the client, not type-assert its
// concrete Go type -- the two struct tags below (`json:"field"`,
// `json:"message"`) are what actually matters to a caller, not the Go type
// name. Exporting it would suggest external packages are meant to
// construct or type-assert []validate.Violation directly, which nothing in
// spec.md/design.md asks for; keeping it package-private matches this
// package's own narrow "internal cross-cutting layer" role (see the
// package doc comment above) and can be exported later without a breaking
// change if a real consumer needs it.
type violation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// resolveSchema verifies m was built for structType (m.StructType() ==
// structType) before any of Must*'s reflect-heavy work runs, panicking with
// a clear message otherwise -- m's field offsets are measured against ITS
// OWN struct's memory layout (internal/schema.New's own doc comment), so
// populating/reading against a mismatched T would silently misinterpret
// memory instead of failing loudly. Shared by MustJsonBody/MustParams/
// MustQuery, all 3 of which now take m explicitly (AD-019) instead of
// looking it up in a global registry keyed by T -- this is the one
// remaining safety net for "the schema I passed doesn't actually describe
// the T I asked for".
func resolveSchema(m *schema.Schema, structType reflect.Type) *schema.Schema {
	if m.StructType() != structType {
		panic(fmt.Sprintf("gonest: schema mismatch -- schema built for %s, but T is %s", m.StructType(), structType))
	}
	return m
}

// jsonBodySource is the Parseable implementation behind ctx.Body().Json()
// (unified-parse-api feature). Reuses the exact validate/populate pipeline
// the old ParseRestJsonBody[T] had, adapted to write into a caller-supplied
// *T (dst) instead of allocating+returning one, since gonest.Parse[T]/
// MustParse[T] already own that `var zero T` allocation.
type jsonBodySource struct {
	req *execution.Request
}

// NewJSONBodySource builds a Parseable for ctx's JSON body. Exported so
// internal/app (the package that bridges execution and validate) can wire
// one into a Context's BodySource per-request.
func NewJSONBodySource(req *execution.Request) execution.Parseable {
	return &jsonBodySource{req: req}
}

// ParseInto reads s.req's raw JSON body into dst (a *T), validating against
// schema (a *schema.Schema) first.
//
// Steps (design.md's Components/"internal/validate" section):
//  1. resolveSchema confirms schema describes T, panicking immediately,
//     BEFORE touching the body at all, otherwise.
//  2. Unmarshal the raw body into `any` -- ground truth for BOTH JSON-key
//     presence (Required checks) and JSON value TYPE checking
//     (context.md's Decision 1). A parse failure here returns
//     *exception.BadRequestException immediately with ONE violation (can't
//     collect per-field violations if the JSON itself doesn't parse --
//     spec.md's P4.1). A body that parses but isn't a JSON object at the
//     top level degrades to "every Required field missing" (design.md's
//     Error Handling Strategy table) rather than crashing.
//  3. validateStruct walks every registered property recursively,
//     collecting every violation (context.md's Decision 2 -- never stops
//     at the first).
//  4. If any violations were collected: return
//     exception.NewBadRequestException(violations) as the error.
//  5. Otherwise: populate dst field-by-field via the shared populate core
//     (param-query-validation feature's P0) instead of a second opaque
//     json.Unmarshal call, so Custom(fn)'s transformed value can actually
//     reach the final result.
func (s *jsonBodySource) ParseInto(dst any, schemaArg any) error {
	m := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(m, dstVal.Type())

	bodyRaw := s.req.Body().Raw()

	var parsed any
	if err := json.Unmarshal(bodyRaw, &parsed); err != nil {
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("invalid JSON: %v", err)},
		})
	}

	// presence may be nil if the top-level JSON value isn't an object (e.g.
	// a bare array or scalar) -- validateStruct treats a nil map the same
	// as "every key absent", which is exactly the graceful degradation
	// design.md's Error Handling Strategy describes.
	presence, _ := parsed.(map[string]any)

	violations := validateStruct(presence, m, "")
	if len(violations) > 0 {
		return exception.NewBadRequestException(violations)
	}

	if err := populate(dstVal, presence, m, "json"); err != nil {
		// Should be unreachable in practice: pass 1 already proved body is
		// valid JSON, and validation already proved every field's shape
		// matches what T expects. Returning an error here (rather than
		// silently leaving dst at its zero value) keeps failures loud
		// instead of masking a genuine bug in the validation pass above.
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to decode request body: %v", err)},
		})
	}

	return nil
}

// tagKey returns the key p's field would be (un)marshaled under for the
// given struct tag name, and whether the field can ever appear at all
// (false for a `<tag>:"-"` value, which encoding/json always skips -- kept
// for every tag this is used with, "json"/"param"/"query", for consistent
// behavior even though only "json" gives that convention any real meaning
// today).
//
// Generalizes what used to be this package's own jsonKey helper (P0 of
// "JSON Body Validation") to take the tag name as a parameter, so
// MustParams/MustQuery (param-query-validation feature's P2/P3) can reuse
// the exact same resolution logic against "param"/"query" tags instead of
// duplicating it. tag="json" produces IDENTICAL results to the old jsonKey
// for every existing call site (zero regression requirement).
//
// Mirrors encoding/json's own tag-parsing rules just enough for this
// package's needs: split on the first comma (dropping options like
// `,omitempty`), fall back to the Go field name if no tag or an empty tag
// name is present.
func tagKey(field reflect.StructField, tag string) string {
	key, _ := tagKeyVisible(field, tag)
	return key
}

// tagKeyVisible is tagKey's full form, also reporting whether the field is
// visible under this tag at all (false for a `<tag>:"-"` value). Kept
// separate from tagKey (which validateStruct doesn't need the bool for --
// see below) purely so tagKey's own signature stays a plain string return,
// matching design.md's Data Models declaration literally
// (`func tagKey(field reflect.StructField, tag string) string`).
func tagKeyVisible(field reflect.StructField, tag string) (string, bool) {
	raw := field.Tag.Get(tag)
	if raw == "-" {
		return "", false
	}
	if raw == "" {
		return field.Name, true
	}
	name := raw
	for i, c := range raw {
		if c == ',' {
			name = raw[:i]
			break
		}
	}
	if name == "" {
		name = field.Name
	}
	return name, true
}

// validateStruct iterates m's own registered properties, checking each
// one's presence in presence (a decoded JSON object, or nil if the parent
// value wasn't an object at all -- see MustJsonBody's step 2 doc comment)
// and recursing into validateValue for whichever ones ARE present.
// pathPrefix is prepended to every violation's Field (e.g. "address." for
// a nested Object, so a violation reads "address.zip" -- spec.md's P5,
// AC3).
func validateStruct(presence map[string]any, m *schema.Schema, pathPrefix string) []violation {
	var violations []violation

	for _, p := range m.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), "json")
		if !visible {
			continue
		}
		path := pathPrefix + key

		raw, present := presence[key]
		if !present {
			if p.IsRequired() {
				violations = append(violations, violation{Field: path, Message: "required"})
			}
			continue
		}

		violations = append(violations, validateValue(raw, p, path)...)
	}

	return violations
}

// validateValue's FIRST check is Custom(fn) (param-query-validation
// feature's P0 -- context.md's Decision 4): when set, fn(raw) FULLY
// REPLACES every built-in kind/format/Min/Max/Pattern check below for this
// field -- a non-nil error becomes this field's one violation, a nil error
// means Custom succeeded and validateValue returns immediately, NEVER
// falling through to the null/kind dispatch that follows. Only when Custom
// was never set does validateValue handle the one concern shared by every
// kind (null handling), then dispatch on p.KindValue() to the
// kind-specific validator.
func validateValue(raw any, p *schema.PropertyBuilder, path string) []violation {
	if fn, ok := p.CustomFunc(); ok {
		if _, err := fn(raw); err != nil {
			return []violation{{Field: path, Message: err.Error()}}
		}
		return nil
	}

	if raw == nil {
		if p.IsNullable() {
			return nil
		}
		return []violation{{Field: path, Message: "cannot be null"}}
	}

	switch p.KindValue() {
	case "array":
		return validateArray(raw, p, path)
	case "object":
		return validateObject(raw, p, path)
	default: // "string", "integer", "number", "boolean"
		return validatePrimitive(raw, p, path)
	}
}

// validatePrimitive checks raw's Go type (as decoded by encoding/json into
// `any`: JSON strings -> string, JSON numbers -> float64, JSON booleans ->
// bool) against what p.KindValue() expects, then applies
// Min/Max/Pattern when the type matches. A type mismatch is recorded as a
// SINGLE "wrong type" violation and stops there -- Min/Max/Pattern are not
// attempted against a value of the wrong Go-level shape (design.md's Error
// Handling Strategy: "does NOT attempt further format-specific checks on a
// value of the wrong Go-level shape").
//
// SPEC_DEVIATION (design.md's own "your call how strict to be" note for
// kind=="integer"): a JSON number with a non-zero fractional part (e.g.
// 1.5) posted for an Integer()/Int32() field is treated as a type
// violation, not silently truncated. Rationale: encoding/json's own
// decode-into-T pass (MustJsonBody's second unmarshal) would itself fail
// or truncate inconsistently depending on the field's exact Go type
// (int64 vs float64), so rejecting non-integral floats during validation
// keeps the two passes' notion of "valid" consistent, and matches
// OpenAPI's own integer format (a fractional value is not a valid
// "integer").
func validatePrimitive(raw any, p *schema.PropertyBuilder, path string) []violation {
	kind := p.KindValue()

	switch kind {
	case "string":
		s, ok := raw.(string)
		if !ok {
			return []violation{{Field: path, Message: "expected string"}}
		}
		var violations []violation
		if min, ok := p.MinValue(); ok && len(s) < min {
			violations = append(violations, violation{Field: path, Message: fmt.Sprintf("length must be >= %d", min)})
		}
		if max, ok := p.MaxValue(); ok && len(s) > max {
			violations = append(violations, violation{Field: path, Message: fmt.Sprintf("length must be <= %d", max)})
		}
		if pattern := p.PatternValue(); pattern != "" {
			// A dev-supplied Pattern() string that fails to compile as a
			// regexp is treated as "no constraint" rather than panicking a
			// request over a declaration-time mistake (Min>Max is handled
			// the same "trust the caller" way elsewhere in this codebase --
			// see design.md's Tech Decisions). This is a SPEC_DEVIATION in
			// the sense that no Done-when item exercises an invalid
			// pattern, but it is the only sane behavior available: a
			// regexp.MustCompile here would turn a bad Pattern() call at
			// declaration time into a 500-shaped panic at request time,
			// far from where the mistake was made.
			if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(s) {
				violations = append(violations, violation{Field: path, Message: fmt.Sprintf("must match pattern %s", pattern)})
			}
		}
		return violations

	case "integer", "number":
		f, ok := raw.(float64)
		if !ok {
			return []violation{{Field: path, Message: "expected number"}}
		}
		if kind == "integer" && f != float64(int64(f)) {
			return []violation{{Field: path, Message: "expected integer, got non-integer number"}}
		}
		var violations []violation
		if min, ok := p.MinValue(); ok && f < float64(min) {
			violations = append(violations, violation{Field: path, Message: fmt.Sprintf("must be >= %d", min)})
		}
		if max, ok := p.MaxValue(); ok && f > float64(max) {
			violations = append(violations, violation{Field: path, Message: fmt.Sprintf("must be <= %d", max)})
		}
		return violations

	case "boolean":
		if _, ok := raw.(bool); !ok {
			return []violation{{Field: path, Message: "expected boolean"}}
		}
		return nil

	default:
		// No branch method was ever called on this PropertyBuilder (kind ==
		// ""), or an unrecognized kind -- nothing to check against, treat
		// as a no-op rather than crashing on a value the framework itself
		// never declared a shape for.
		return nil
	}
}

// validateArray type-asserts raw to []any, checks the ARRAY's own quantity
// Min/Max (spec.md's P5 AC2 -- a quantity violation is recorded against
// the FIELD itself, not any specific item), then recurses into every item:
// either validateValue against the item's own *PropertyBuilder
// (p.ItemBuilder()), or, if the item is Object(ref)-typed (p.ItemRef()),
// validateStruct against the referenced *Schema. Each item's path
// includes its index (e.g. "tags[2]", "addresses[0].zip") -- spec.md's P5
// AC1.
func validateArray(raw any, p *schema.PropertyBuilder, path string) []violation {
	items, ok := raw.([]any)
	if !ok {
		return []violation{{Field: path, Message: "expected array"}}
	}

	var violations []violation

	if min, ok := p.MinValue(); ok && len(items) < min {
		violations = append(violations, violation{Field: path, Message: fmt.Sprintf("must have at least %d items", min)})
	}
	if max, ok := p.MaxValue(); ok && len(items) > max {
		violations = append(violations, violation{Field: path, Message: fmt.Sprintf("must have at most %d items", max)})
	}

	itemRef, hasItemRef := p.ItemRef()

	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if hasItemRef {
			itemMap, ok := item.(map[string]any)
			if !ok {
				violations = append(violations, violation{Field: itemPath, Message: "expected object"})
				continue
			}
			violations = append(violations, validateStruct(itemMap, itemRef, itemPath+".")...)
			continue
		}
		violations = append(violations, validateValue(item, p.ItemBuilder(), itemPath)...)
	}

	return violations
}

// validateObject handles a field's own Object()-typed value: if
// p.IsAdditionalProperties() is set (an open/free-form schema, e.g.
// `Schema map[string]any` -- INSIGHT.md's `om.AdditionalProperties()`),
// structural validation is skipped entirely (spec.md's Out of Scope: "no
// fixed shape exists to check against by definition"). Otherwise, if
// p.SchemaRef() is set, raw is type-asserted to map[string]any and
// recursed into via validateStruct, with path prefixed by "." so a nested
// violation reads e.g. "address.zip" (spec.md's P5 AC3). If neither is
// set, there is nothing to validate against -- treated as a no-op (a dev
// called Object(fn) but never called Schema(ref) or
// AdditionalProperties() inside fn, an edge case design.md doesn't assign
// behavior to beyond "nothing to validate against").
func validateObject(raw any, p *schema.PropertyBuilder, path string) []violation {
	if p.IsAdditionalProperties() {
		return nil
	}

	ref, ok := p.SchemaRef()
	if !ok {
		return nil
	}

	objMap, ok := raw.(map[string]any)
	if !ok {
		return []violation{{Field: path, Message: "expected object"}}
	}

	return validateStruct(objMap, ref, path+".")
}

// populate is the shared reflect-based "build T" core used by ALL public
// entry points (MustJsonBody today; MustParams/MustQuery in later tasks of
// this feature -- param-query-validation's context.md Decision 5). It walks
// m's own registered properties, resolves each field's key via tagKey(...,
// tag), and writes that field's final value into dest via setField.
//
// Called ONLY after the caller's own validate pass already confirmed zero
// violations (mirrors the two-step "validate everything, THEN build" order
// MustJsonBody already had before this refactor) -- populate itself does
// NOT re-validate, it trusts presence's values are already known-good
// (or, for Custom fields, known to succeed when called again -- see below).
//
// If a field's key is absent from presence, it is skipped entirely: either
// it was optional and legitimately missing (nothing to populate, dest
// field stays its Go zero value), or it was Required and missing, in which
// case validateStruct already recorded a violation earlier and the caller
// would have panicked before ever reaching populate.
//
// If p.CustomFunc() is set, fn(raw) is called AGAIN here (it already ran
// once inside validateValue during the validate pass) and its returned
// value is what gets written -- deliberately not cached/deduplicated (see
// design.md's Tech Decisions: fn must be idempotent, called up to 2x per
// field per request, documented on PropertyBuilder.Custom's own doc
// comment). Since a failing Custom already short-circuited validateValue
// with a violation earlier, and MustJsonBody/MustParams/MustQuery all check
// violations BEFORE calling populate, fn is guaranteed to succeed here too
// (same raw input, same idempotent fn).
func populate(dest reflect.Value, presence map[string]any, m *schema.Schema, tag string) error {
	for _, p := range m.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), tag)
		if !visible {
			continue
		}

		raw, ok := presence[key]
		if !ok {
			continue
		}

		value := raw
		if fn, isCustom := p.CustomFunc(); isCustom {
			v, err := fn(raw)
			if err != nil {
				// Unreachable in practice (see doc comment above), but
				// treated as a structured error rather than silently
				// skipping the field, consistent with this package's "never
				// crash, always structured error" stance.
				return fmt.Errorf("field %q: Custom(fn) failed on population pass: %w", key, err)
			}
			value = v
		}

		fieldVal := dest.FieldByIndex(p.Field().Index)
		if err := setField(fieldVal, value); err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
	}

	return nil
}

// setField converts raw into fieldVal's own Go type and Sets it, returning
// an error instead of letting reflect panic when the types are
// incompatible (spec.md's Edge Cases: "never panic a raw reflect error").
//
// raw is either: (a) a value already in the shape encoding/json's own
// generic decode produces (string/float64/bool/nil/map[string]any/[]any --
// the same shape validateValue/validatePrimitive already consume), or (b)
// whatever a Custom(fn) call returned, which can be ANY Go value including
// one that already matches fieldVal's type exactly (e.g. Custom parsing
// "v1:42" into a plain int).
//
// Strategy: if raw is already directly assignable/convertible to fieldVal's
// type via reflect (covers Custom's common case of returning the exact
// target type, plus simple numeric/string/bool coercions), do that
// directly. Otherwise -- covering the JSON-decoded generic shapes for
// nested struct/slice/map fields, where a naive reflect assignment can
// never work since `map[string]any` is never assignable to e.g.
// AddressEntity -- fall back to a JSON round-trip (marshal raw back to
// bytes, unmarshal into fieldVal's address), reusing encoding/json's own
// well-tested conversion rules rather than reimplementing them by hand.
func setField(fieldVal reflect.Value, raw any) error {
	if !fieldVal.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	fieldType := fieldVal.Type()

	if raw == nil {
		switch fieldType.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice:
			fieldVal.Set(reflect.Zero(fieldType))
			return nil
		default:
			// A nil raw value reaching setField for a non-nilable Go type
			// (e.g. plain string/int) should already have been rejected by
			// the validate pass (Nullable check) before populate ever runs
			// -- but treat as a no-op (leave the Go zero value) rather than
			// panicking, matching this package's "never crash" stance.
			return nil
		}
	}

	rawVal := reflect.ValueOf(raw)

	// Pointer/nilable destination (e.g. *string for a Nullable field with a
	// non-nil value present): allocate the pointee and recurse into it.
	if fieldType.Kind() == reflect.Ptr {
		ptr := reflect.New(fieldType.Elem())
		if err := setField(ptr.Elem(), raw); err != nil {
			return err
		}
		fieldVal.Set(ptr)
		return nil
	}

	// Directly assignable (identical types, e.g. Custom returning exactly
	// the field's own type, or an interface{}/any-typed field).
	if rawVal.Type().AssignableTo(fieldType) {
		fieldVal.Set(rawVal)
		return nil
	}

	// Numeric family: JSON numbers decode to float64; Custom(fn) may also
	// return any numeric Go type. Convert via the appropriate reflect
	// Set*, rather than a blanket Convert (which would silently succeed for
	// nonsensical combinations reflect itself considers "convertible",
	// e.g. string->int would panic at Convert time for non-numeric
	// strings -- restricting this branch to KNOWN numeric kinds keeps that
	// panic surface closed).
	if isNumericKind(fieldType.Kind()) {
		f, ok := toFloat64(raw)
		if !ok {
			return fmt.Errorf("cannot convert %T to %s", raw, fieldType)
		}
		switch {
		case fieldType.Kind() >= reflect.Int && fieldType.Kind() <= reflect.Int64:
			fieldVal.SetInt(int64(f))
		case fieldType.Kind() >= reflect.Uint && fieldType.Kind() <= reflect.Uintptr:
			fieldVal.SetUint(uint64(f))
		case fieldType.Kind() == reflect.Float32 || fieldType.Kind() == reflect.Float64:
			fieldVal.SetFloat(f)
		}
		return nil
	}

	if fieldType.Kind() == reflect.String {
		s, ok := raw.(string)
		if !ok {
			return fmt.Errorf("cannot convert %T to string", raw)
		}
		fieldVal.SetString(s)
		return nil
	}

	if fieldType.Kind() == reflect.Bool {
		b, ok := raw.(bool)
		if !ok {
			return fmt.Errorf("cannot convert %T to bool", raw)
		}
		fieldVal.SetBool(b)
		return nil
	}

	// Struct/slice/map/array destinations that aren't directly assignable
	// from raw's own Go type (e.g. raw is a JSON-decoded map[string]any but
	// fieldVal is a concrete AddressEntity struct, or raw is []any but
	// fieldVal is []AddressEntity) -- reuse encoding/json's own conversion
	// rules via a round-trip rather than hand-rolling nested reflect
	// construction, since MustJsonBody's validate pass already proved raw's
	// SHAPE matches what T expects.
	switch fieldType.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("cannot convert %T to %s: %w", raw, fieldType, err)
		}
		target := reflect.New(fieldType)
		if err := json.Unmarshal(encoded, target.Interface()); err != nil {
			return fmt.Errorf("cannot convert %T to %s: %w", raw, fieldType, err)
		}
		fieldVal.Set(target.Elem())
		return nil
	}

	return fmt.Errorf("cannot convert %T to %s", raw, fieldType)
}

// isNumericKind reports whether k is any of Go's integer/unsigned/float
// kinds -- the set setField's numeric branch handles uniformly via
// SetInt/SetUint/SetFloat.
func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// toFloat64 extracts a float64 from raw regardless of its concrete Go
// numeric type -- JSON numbers always decode to float64, but Custom(fn) may
// return any numeric type (int, int64, float32, ...), so this normalizes
// before handing off to SetInt/SetUint/SetFloat's own int64/uint64/float64
// parameter types.
func toFloat64(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
