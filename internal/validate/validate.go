// Package validate is the real implementation behind the public root
// gonest.MustJsonBody[T] wrapper (Go cannot re-export a generic function via
// var, see AD-004 -- same reasoning as inject.MustInject/route.MustParam).
// It reads *execution.Context's raw request body, validates it against T's
// registered *metadata.Metadata (internal/metadata), and panics
// *exception.BadRequestException on any violation -- mirroring how
// internal/route is a thin cross-cutting layer over internal/execution +
// internal/pipe, this package is a thin cross-cutting layer over
// internal/execution + internal/metadata + internal/exception, none of
// which import each other back (design.md's Architecture Overview).
package validate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/metadata"
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

// MustJsonBody is the real implementation behind gonest.MustJsonBody[T] (T
// is a pointer type at the call site, e.g. MustJsonBody[*UserProperties]).
//
// Steps (design.md's Components/"internal/validate" section):
//  1. Look up T's (dereferenced) registered *metadata.Metadata via the
//     global registry -- panics immediately, BEFORE touching the body at
//     all, if T was never registered via NewMetadata[T] (spec.md's Edge
//     Cases).
//  2. Unmarshal the raw body into `any` -- ground truth for BOTH JSON-key
//     presence (Required checks) and JSON value TYPE checking
//     (context.md's Decision 1). A parse failure here panics
//     BadRequestException immediately with ONE violation (can't collect
//     per-field violations if the JSON itself doesn't parse -- spec.md's
//     P4.1). A body that parses but isn't a JSON object at the top level
//     degrades to "every Required field missing" (design.md's Error
//     Handling Strategy table) rather than crashing.
//  3. validateStruct walks every registered property recursively,
//     collecting every violation (context.md's Decision 2 -- never stops
//     at the first).
//  4. If any violations were collected: panic
//     exception.NewBadRequestException(violations).
//  5. Otherwise: unmarshal the body a SECOND time, this time into a fresh
//     *structType, and return it as T.
func MustJsonBody[T any](ctx *execution.Context) T {
	var zero T
	structType := reflect.TypeOf(zero).Elem()

	m, ok := metadata.Lookup(structType)
	if !ok {
		panic(fmt.Sprintf("gonest: no metadata registered for type %s (call NewMetadata[%s] first)", structType, structType))
	}

	body := ctx.Body()

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		panic(exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("invalid JSON: %v", err)},
		}))
	}

	// presence may be nil if the top-level JSON value isn't an object (e.g.
	// a bare array or scalar) -- validateStruct treats a nil map the same
	// as "every key absent", which is exactly the graceful degradation
	// design.md's Error Handling Strategy describes.
	presence, _ := parsed.(map[string]any)

	violations := validateStruct(presence, m, "")
	if len(violations) > 0 {
		panic(exception.NewBadRequestException(violations))
	}

	out := reflect.New(structType).Interface()
	if err := json.Unmarshal(body, out); err != nil {
		// Should be unreachable in practice: pass 1 already proved body is
		// valid JSON, and validation already proved every field's shape
		// matches what T expects. Panicking here (rather than silently
		// returning a zero T) keeps failures loud instead of masking a
		// genuine bug in the validation pass above.
		panic(exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to decode request body: %v", err)},
		}))
	}

	return out.(T)
}

// jsonKey returns the JSON key p's field would be (un)marshaled under by
// encoding/json itself, and whether the field can ever appear in JSON at
// all (false for a `json:"-"` tag, which encoding/json always skips).
//
// Mirrors encoding/json's own tag-parsing rules just enough for this
// package's needs: split on the first comma (dropping options like
// `,omitempty`), fall back to the Go field name if no tag or an empty tag
// name is present.
func jsonKey(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if tag == "" {
		return f.Name, true
	}
	name := tag
	for i, c := range tag {
		if c == ',' {
			name = tag[:i]
			break
		}
	}
	if name == "" {
		name = f.Name
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
func validateStruct(presence map[string]any, m *metadata.Metadata, pathPrefix string) []violation {
	var violations []violation

	for _, p := range m.OwnProperties() {
		key, visible := jsonKey(p.Field())
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

// validateValue handles the one concern shared by every kind (null
// handling), then dispatches on p.KindValue() to the kind-specific
// validator.
func validateValue(raw any, p *metadata.PropertyBuilder, path string) []violation {
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
func validatePrimitive(raw any, p *metadata.PropertyBuilder, path string) []violation {
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
// validateStruct against the referenced *Metadata. Each item's path
// includes its index (e.g. "tags[2]", "addresses[0].zip") -- spec.md's P5
// AC1.
func validateArray(raw any, p *metadata.PropertyBuilder, path string) []violation {
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
// `Metadata map[string]any` -- INSIGHT.md's `om.AdditionalProperties()`),
// structural validation is skipped entirely (spec.md's Out of Scope: "no
// fixed shape exists to check against by definition"). Otherwise, if
// p.MetadataRef() is set, raw is type-asserted to map[string]any and
// recursed into via validateStruct, with path prefixed by "." so a nested
// violation reads e.g. "address.zip" (spec.md's P5 AC3). If neither is
// set, there is nothing to validate against -- treated as a no-op (a dev
// called Object(fn) but never called Metadata(ref) or
// AdditionalProperties() inside fn, an edge case design.md doesn't assign
// behavior to beyond "nothing to validate against").
func validateObject(raw any, p *metadata.PropertyBuilder, path string) []violation {
	if p.IsAdditionalProperties() {
		return nil
	}

	ref, ok := p.MetadataRef()
	if !ok {
		return nil
	}

	objMap, ok := raw.(map[string]any)
	if !ok {
		return []violation{{Field: path, Message: "expected object"}}
	}

	return validateStruct(objMap, ref, path+".")
}
