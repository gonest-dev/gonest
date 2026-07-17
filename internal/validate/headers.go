package validate

import (
	"fmt"
	"reflect"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// headersSource is the Parseable implementation behind ctx.Headers()
// (unified-parse-api feature) -- a net-new capability, no equivalent existed
// before this feature. Reads each of T's own registered properties via its
// `header:"name"` struct tag, using ctx.Header(name) (case-insensitive,
// delegated to the underlying HTTP framework).
//
// Same shape as paramsSource/querySource: coerce the raw header string into
// the any-shape validateValue/validatePrimitive already consume, then reuse
// validateValue/populate unchanged (design.md's Tech Decisions:
// "coerce-then-reuse, not a parallel validation path").
type headersSource struct {
	req *execution.Request
}

// NewHeadersSource builds a Parseable for ctx's HTTP headers. Exported so
// internal/app can wire one into a Context per-request.
func NewHeadersSource(req *execution.Request) execution.Parseable {
	return &headersSource{req: req}
}

// ParseInto reads s.req's headers into dst (a *T) via T's `header:"name"`
// struct tags, validating against schema (a *schema.Schema) first.
//
//  1. resolveSchema confirms schema describes T, panicking immediately,
//     BEFORE reading any header at all, otherwise.
//  2. For each of T's own registered properties: resolve its key via
//     tagKey(field, "header"), check presence via ctx.Header(key) (empty
//     string means absent -- same convention every HTTP framework uses,
//     since a header can't meaningfully distinguish "absent" from "present
//     but empty" without a lower-level API this codebase doesn't expose).
//     If Required and absent, record a violation (same convention
//     validateStruct/paramsSource/querySource already use).
//  3. If present: coerce the raw string into the any-shape validateValue
//     already expects via coerceParamString, then validate via
//     validateValue (reused unchanged) -- a coercion failure is recorded as
//     a violation directly.
//  4. Collect ALL violations across every field (context.md's Decision 2 --
//     never stop early).
//  5. If any violations were collected: return
//     exception.NewBadRequestException(violations).
//  6. Otherwise: populate dst field-by-field via the shared populate core
//     (tag="header").
func (s *headersSource) ParseInto(dst any, schemaArg any) error {
	m := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(m, dstVal.Type())

	var violations []violation
	presence := map[string]any{}

	for _, p := range m.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), "header")
		if !visible {
			continue
		}

		raw := s.req.Header(key)
		if raw == "" {
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

	if err := populate(dstVal, presence, m, "header"); err != nil {
		// Should be unreachable in practice, same rationale as
		// jsonBodySource's own equivalent case.
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate headers: %v", err)},
		})
	}

	return nil
}
