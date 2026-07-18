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
	return parseDecoded(dst, dstVal, m, s.parsed)
}
