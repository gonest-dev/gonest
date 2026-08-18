package validate

import (
	"fmt"
	"os"
	"reflect"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/schema"
)

// ParseEnvInto reads OS environment variables into dst (a *T) via T's
// `env:"NAME"` struct tags, validating against schema (a *schema.Schema)
// first -- the env-schema-binding feature's Parseable implementation behind
// *dotenv.Dotenv.ParseInto (internal/dotenv), which just delegates here.
//
// A FREE FUNCTION, deliberately NOT a struct holding a *execution.Request
// like paramsSource/querySource/headersSource: there is no per-request
// state in a config-loading context (env vars are read once, at boot, not
// per-HTTP-request -- design.md's Tech Decisions).
//
// Mirrors paramsSource.ParseInto (internal/validate/params.go) almost
// exactly:
//
//  1. resolveSchema confirms schema describes T, panicking immediately,
//     BEFORE reading any env var at all, otherwise.
//  2. For each of T's own registered properties: resolve its key via
//     tagKeyVisible(p.Field(), "env"), `continue` if the field isn't
//     visible under that tag (only an explicit `env:"-"` is invisible --
//     a MISSING tag falls back to the Go field name, same as every other
//     tag this helper resolves -- see tagKeyVisible's own doc comment).
//  3. Presence is checked via os.LookupEnv(key), NOT os.Getenv alone --
//     LookupEnv's second return distinguishes "unset" (ok=false) from
//     "set to empty string" (ok=true, value="") -- design.md's Open Edge
//     Cases table: an empty-but-set env var is PRESENT, not absent, and
//     does NOT trigger Default.
//  4. Absent (ok=false):
//     - if p.DefaultValue() reports presence (its own bool return),
//     presence[key] is set DIRECTLY to that value, bypassing
//     coerceParamString/validateValue entirely -- a Default value is a Go
//     literal the dev wrote in code (e.g. .Default(5432) for an
//     Integer() field), already the correct Go type, not a string
//     needing coercion (design.md's Tech Decisions).
//     - otherwise, if p.IsRequired(), a violation{Field: key, Message:
//     "required"} is collected; if not required, simply `continue`
//     (dst's field stays its Go zero value).
//  5. Present (ok=true, even if raw == ""): same Custom-first-else-coerce
//     branching paramsSource already uses, verbatim -- Custom(fn) set on
//     the field receives the RAW STRING directly via validateValue;
//     otherwise raw is coerced via coerceParamString(raw, p.KindValue())
//     (REUSED unchanged) before validateValue.
//  6. Collect ALL violations across every field (collect-all, never stop
//     early -- same convention every other *Source in this package uses).
//  7. If any violations were collected: return
//     exception.NewBadRequestException(violations).
//  8. Otherwise: populate dst field-by-field via the shared populate core
//     (tag="env", REUSED unchanged).
func ParseEnvInto(dst any, schemaArg any) error {
	s := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(s, dstVal.Type())

	var violations []violation
	presence := map[string]any{}

	for _, p := range s.OwnProperties() {
		key, visible := tagKeyVisible(p.Field(), "env")
		if !visible {
			continue
		}

		raw, ok := os.LookupEnv(key)
		if !ok {
			if defaultValue, hasDefault := p.DefaultValue(); hasDefault {
				presence[key] = defaultValue
				continue
			}
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

	if err := populate(dstVal, presence, s, "env"); err != nil {
		// Should be unreachable in practice, same rationale as
		// paramsSource/jsonBodySource's own equivalent case: the validate
		// pass above already proved every present field's shape matches
		// what T expects. Returning an error here keeps failures loud
		// instead of masking a genuine bug in the validation pass.
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate env: %v", err)},
		})
	}

	return nil
}
