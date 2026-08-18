package validate

import (
	"fmt"
	"reflect"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// querySource is the Parseable implementation behind ctx.Query()
// (unified-parse-api feature).
//
// It is the SAME shape as paramsSource (internal/validate/params.go), except
// presence/raw values come from ctx.Queries() (a plain map[string]string
// sourced from the request's query string, see execution.Request.Queries)
// instead of route.HasParam/ctx.Param. Reuses coerceParamString/populate/
// validateValue/tagKey exactly, unchanged -- design.md's Tech Decisions:
// "coerce-then-reuse, not a parallel validation path".
type querySource struct {
	req *execution.Request
}

// NewQuerySource builds a Parseable for ctx's query string. Exported so
// internal/app can wire one into a Context per-request.
func NewQuerySource(req *execution.Request) execution.Parseable {
	return &querySource{req: req}
}

// ParseInto reads s.req's query string into dst (a *T) via T's
// `query:"name"` struct tags, validating against schema (a *schema.Schema)
// first.
//
//  1. resolveSchema confirms schema describes T, panicking immediately,
//     BEFORE reading any query value at all, otherwise.
//  2. Read ctx.Queries() once (a plain map[string]string).
//  3. For each of T's own registered properties: resolve its key via
//     tagKey(field, "query"), check presence via `_, ok :=
//     ctx.Queries()[key]`, and if Required and absent, record a violation
//     (same convention validateStruct/paramsSource already use).
//  4. If present: read the raw string. If Custom(fn) is set on the field,
//     hand the RAW STRING straight to validateValue (its own Custom
//     short-circuit calls fn(raw) directly, never coercing -- spec.md's
//     P3.3). Otherwise, coerce the raw string into the any-shape
//     validateValue already expects via coerceParamString, then validate
//     via validateValue (reused unchanged) -- a coercion failure is
//     recorded as a violation directly.
//  5. Collect ALL violations across every field (same collect-all behavior
//     as jsonBodySource/paramsSource, context.md's Decision 2 -- never stop
//     early).
//  6. If any violations were collected: return
//     exception.NewBadRequestException(violations) as the error.
//  7. Otherwise: populate dst field-by-field via the shared populate core
//     (tag="query"), using the SAME raw/coerced value already produced
//     during validation as the presence map's value.
func (src *querySource) ParseInto(dst any, schemaArg any) error {
	s := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(s, dstVal.Type())

	queries := src.req.Queries()

	var violations []violation
	presence := map[string]any{}

	for _, p := range s.OwnProperties() {
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
		return exception.NewBadRequestException(violations)
	}

	if err := populate(dstVal, presence, s, "query"); err != nil {
		// Should be unreachable in practice, same rationale as
		// paramsSource's/jsonBodySource's own equivalent case: the validate
		// pass above already proved every present field's shape matches
		// what T expects. Returning an error here keeps failures loud
		// instead of masking a genuine bug in the validation pass.
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate query: %v", err)},
		})
	}

	return nil
}
