package graphql

import (
	gql "github.com/graphql-go/graphql"
)

// Execute runs a single GraphQL operation (query/mutation) against an
// already-built *gql.Schema and returns the {data, errors} pair the
// GraphQL-over-HTTP convention expects. This is the exact gql.Do(gql.Params{...})
// call plus the result.Errors -> []map[string]any{"message": ...} mapping
// that used to live inline in internal/app/graphql.go's graphqlHandler --
// extracted here so WS/SSE transports (later tasks) can reuse the identical
// dispatch instead of copying it and risking divergence. Subscription
// dispatch is NOT handled here -- gql.Do only ever resolves Query/Mutation
// root fields; Subscriptions connect through SSEHandler/WSHandler instead.
func Execute(sch *gql.Schema, query string, variables map[string]any, operationName string) (data any, errors []map[string]any) {
	result := gql.Do(gql.Params{
		Schema:         *sch,
		RequestString:  query,
		VariableValues: variables,
		OperationName:  operationName,
	})

	var errs []map[string]any
	for _, e := range result.Errors {
		errs = append(errs, map[string]any{"message": e.Message})
	}

	return result.Data, errs
}
