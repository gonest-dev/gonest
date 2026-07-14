package validate

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/metadata"
	"github.com/gonest-dev/gonest/internal/route"
)

// MustParams is the real implementation behind the public root
// gonest.MustParams[T] wrapper (Go cannot re-export a generic function via
// var, see AD-004). T is a pointer type at the call site (e.g.
// MustParams[*UserIdParams]).
//
// Unlike MustJsonBody (which validates a single JSON object payload),
// MustParams validates every path param declared on T's registered
// *metadata.Metadata (via a `param:"name"` struct tag, same tag-resolution
// convention MustJsonBody uses for `json:"..."`) against the CURRENT
// route's actual ":name" segments -- reusing validateValue/populate
// UNCHANGED once each param's raw string is coerced into the same any-shape
// (string/float64/bool) JSON decoding already produces (design.md's Tech
// Decisions: "coerce-then-reuse, not a parallel validation path").
//
// Steps (design.md's Architecture Overview, "MustParams" section):
//  1. Look up T's (dereferenced) registered *metadata.Metadata via the
//     global registry -- panics immediately, BEFORE reading any param at
//     all, if T was never registered via NewMetadata[T] (spec.md's Edge
//     Cases, same precedent as MustJsonBody).
//  2. Resolve ctx's currently-attached *route.Route (via ctx.Route(), an
//     `any` type-asserted back to *route.Route -- see execution.Context's
//     WithRoute/Route doc comment for why the link is untyped at the
//     execution layer).
//  3. For each of T's own registered properties: resolve its key via
//     tagKey(field, "param"), check presence via route.HasParam(key) (a
//     field whose key has no ":name" segment on the route's declared
//     pattern at all is treated as absent, spec.md's P2.2), and if
//     Required and absent, record a violation (same convention
//     validateStruct already uses for JSON body Required checks).
//  4. If present: read the raw string via ctx.Param(key). If Custom(fn) is
//     set on the field, hand the RAW STRING straight to validateValue (its
//     own Custom short-circuit calls fn(raw) directly, never coercing --
//     spec.md's P2.3). Otherwise, coerce the raw string into the any-shape
//     validateValue already expects via coerceParamString, then validate
//     via validateValue (reused unchanged) -- a coercion failure is
//     recorded as a violation directly, validateValue is not even called
//     (the value couldn't become the right shape to check Min/Max/Pattern
//     against).
//  5. Collect ALL violations across every field (same collect-all behavior
//     as MustJsonBody, context.md's Decision 2 -- never stop early).
//  6. If any violations were collected: panic
//     exception.NewBadRequestException(violations).
//  7. Otherwise: populate a fresh *structType field-by-field via the shared
//     populate core (tag="param"), using the SAME raw/coerced value already
//     produced during validation as the presence map's value.
func MustParams[T any](ctx *execution.Context) T {
	var zero T
	structType := reflect.TypeOf(zero).Elem()

	m, ok := metadata.Lookup(structType)
	if !ok {
		panic(fmt.Sprintf("gonest: no metadata registered for type %s (call NewMetadata[%s] first)", structType, structType))
	}

	r, hasRoute := ctx.Route().(*route.Route)

	var violations []violation
	presence := map[string]any{}

	for _, p := range m.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), "param")
		if !visible {
			continue
		}

		if !hasRoute || !r.HasParam(key) {
			if p.IsRequired() {
				violations = append(violations, violation{Field: key, Message: "required"})
			}
			continue
		}

		raw := ctx.Param(key)

		if _, isCustom := p.CustomFunc(); isCustom {
			violations = append(violations, validateValue(raw, p, key)...)
			presence[key] = raw
			continue
		}

		coerced, err := coerceParamString(raw, p.KindValue())
		if err != nil {
			violations = append(violations, violation{Field: key, Message: err.Error()})
			continue
		}

		violations = append(violations, validateValue(coerced, p, key)...)
		presence[key] = coerced
	}

	if len(violations) > 0 {
		panic(exception.NewBadRequestException(violations))
	}

	out := reflect.New(structType)
	if err := populate(out.Elem(), presence, m, "param"); err != nil {
		// Should be unreachable in practice, same rationale as
		// MustJsonBody's own equivalent panic: the validate pass above
		// already proved every present field's shape matches what T
		// expects. Panicking here keeps failures loud instead of masking a
		// genuine bug in the validation pass.
		panic(exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate params: %v", err)},
		}))
	}

	return out.Interface().(T)
}

// coerceParamString converts a raw path/query param string into the same
// any-shape validateValue/validatePrimitive already consume (string stays
// string; JSON numbers decode to float64, so "integer"/"number" parse via
// strconv.ParseFloat to match; "boolean" via strconv.ParseBool) -- letting
// MustParams/MustQuery reuse validateValue/validatePrimitive UNCHANGED once
// the string is coerced (design.md's Tech Decisions).
//
// Runs only when Custom(fn) is NOT set on the field (Custom always receives
// the raw string directly, spec.md's P2.3/P3.3) -- MustParams checks for
// Custom BEFORE ever calling this function.
func coerceParamString(raw string, kind string) (any, error) {
	switch kind {
	case "string":
		return raw, nil
	case "integer", "number":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("expected a number, got %q", raw)
		}
		return f, nil
	case "boolean":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected a boolean, got %q", raw)
		}
		return b, nil
	default:
		// No branch method was ever called on this PropertyBuilder (kind ==
		// ""), or an unrecognized kind -- fall back to the raw string
		// unchanged, mirroring validatePrimitive's own "nothing to check
		// against" no-op for an unrecognized kind.
		return raw, nil
	}
}
