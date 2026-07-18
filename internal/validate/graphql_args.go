package validate

import (
	"reflect"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// mapArgsSource is the Parseable implementation behind a GraphQL Query/
// Mutation's ctx.Args() (graphql-support feature, T7's SPEC_DEVIATION,
// internal/graphqlgen). Unlike jsonBodySource, there is no raw JSON body
// to unmarshal -- graphql-go's own execution engine already decoded the
// request's arguments into a plain Go value (a map[string]any for a
// struct-shaped Args schema, matching graphql.ResolveParams.Args' own
// type) before internal/graphqlgen's Resolve callback ever runs. Reuses
// parseDecoded, the EXACT same validate/populate/Refine pipeline
// jsonBodySource.ParseInto uses after its own json.Unmarshal step.
type mapArgsSource struct {
	parsed any
}

// NewGraphqlArgsSource builds a Parseable over parsed -- an
// already-decoded value (map[string]any for a struct-shaped schema, or a
// bare scalar for a schema-value-support Value-schema), as produced by
// graphql-go's own argument decoding. Exported so internal/graphqlgen
// (which cannot import this package's unexported jsonBodySource) can wire
// gqlresolver.GraphqlContext.Args() to the same validation engine REST
// already uses.
func NewGraphqlArgsSource(parsed any) execution.Parseable {
	return &mapArgsSource{parsed: parsed}
}

// ParseInto implements execution.Parseable.
func (s *mapArgsSource) ParseInto(dst any, schemaArg any) error {
	m := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(m, dstVal.Type())
	return parseDecoded(dst, dstVal, m, normalizeGraphqlValue(s.parsed))
}

// normalizeGraphqlValue converts graphql-go's own decoded arg shapes into
// the SAME shapes encoding/json's decode-into-any produces -- the shape
// validateValue/validatePrimitive/setField (this package's shared
// validate/populate pipeline, reused unchanged from REST) already assume.
//
// SPEC_DEVIATION (real bug found via a live example's Query, not caught by
// unit tests that only ever passed raw string/int64 args by hand):
// graphql-go's own Int/Float scalar coercion (confirmed via Context7 --
// stream.go/coerceInt) parses a GraphQL `Int` literal into a native Go
// `int`, not `float64` -- but validatePrimitive's "integer"/"number" case
// does a hard `raw.(float64)` type assertion, matching ONLY what
// encoding/json's own json.Unmarshal(..., &any{}) produces for a JSON
// number. Left unnormalized, EVERY integer/float GraphQL arg failed
// validation with a confusing, near-silent "expected number" violation
// (surfacing as an EMPTY error message, since NewBadRequestException's
// default Message() is "" when never explicitly set).
func normalizeGraphqlValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalizeGraphqlValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeGraphqlValue(val)
		}
		return out
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	default:
		return v
	}
}
