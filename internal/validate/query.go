package validate

import (
	"fmt"
	"reflect"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/schema"
)

// MustQuery is the real implementation behind the public root
// gonest.MustQuery[T] wrapper (Go cannot re-export a generic function via
// var, see AD-004). T is a pointer type at the call site (e.g.
// MustQuery[*ListUsersQuery]), and m must be the *schema.Schema built via
// NewSchema[T] for that same T (AD-019) -- resolveSchema panics if it
// describes a different type.
//
// It is the SAME shape as MustParams (internal/validate/params.go), except
// presence/raw values come from ctx.Queries() (a plain map[string]string
// sourced from the request's query string, see execution.Context.Queries)
// instead of route.HasParam/ctx.Param. Reuses coerceParamString/populate/
// validateValue/tagKey exactly, unchanged -- design.md's Tech Decisions:
// "coerce-then-reuse, not a parallel validation path".
//
// Steps (design.md's Architecture Overview, "MustQuery" section):
//  1. resolveSchema confirms m describes T, panicking immediately, BEFORE
//     reading any query value at all, otherwise.
//  2. Read ctx.Queries() once (a plain map[string]string).
//  3. For each of T's own registered properties: resolve its key via
//     tagKey(field, "query"), check presence via `_, ok :=
//     ctx.Queries()[key]`, and if Required and absent, record a violation
//     (same convention validateStruct/MustParams already use).
//  4. If present: read the raw string. If Custom(fn) is set on the field,
//     hand the RAW STRING straight to validateValue (its own Custom
//     short-circuit calls fn(raw) directly, never coercing -- spec.md's
//     P3.3). Otherwise, coerce the raw string into the any-shape
//     validateValue already expects via coerceParamString, then validate
//     via validateValue (reused unchanged) -- a coercion failure is
//     recorded as a violation directly.
//  5. Collect ALL violations across every field (same collect-all behavior
//     as MustJsonBody/MustParams, context.md's Decision 2 -- never stop
//     early).
//  6. If any violations were collected: panic
//     exception.NewBadRequestException(violations).
//  7. Otherwise: populate a fresh *structType field-by-field via the shared
//     populate core (tag="query"), using the SAME raw/coerced value already
//     produced during validation as the presence map's value.
func MustQuery[T any](ctx *execution.Context, m *schema.Schema) T {
	var zero T
	structType := reflect.TypeOf(zero).Elem()
	resolveSchema(m, structType)

	queries := ctx.Queries()

	var violations []violation
	presence := map[string]any{}

	for _, p := range m.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), "query")
		if !visible {
			continue
		}

		raw, present := queries[key]
		if !present {
			if p.IsRequired() {
				violations = append(violations, violation{Field: key, Message: "required"})
			}
			continue
		}

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
	if err := populate(out.Elem(), presence, m, "query"); err != nil {
		// Should be unreachable in practice, same rationale as
		// MustParams'/MustJsonBody's own equivalent panic: the validate pass
		// above already proved every present field's shape matches what T
		// expects. Panicking here keeps failures loud instead of masking a
		// genuine bug in the validation pass.
		panic(exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate query: %v", err)},
		}))
	}

	return out.Interface().(T)
}
