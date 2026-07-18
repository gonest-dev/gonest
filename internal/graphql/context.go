package graphql

import "gonest.dev/gonest/internal/execution"

// GraphqlContext is the single parameter passed to every Query/Mutation/
// Subscription Handler -- wraps the GraphQL request's own args behind the
// same Parseable vocabulary execution.Request already uses for REST, so
// gonest.MustParse[T](ctx.Args(), schema) works identically across both.
type GraphqlContext struct {
	args execution.Parseable
	done <-chan struct{}
}

// NewGraphqlContext builds a *GraphqlContext for a single field resolution.
// done may be nil for Query/Mutation (request-response, no cancellation
// concept); Subscription always supplies one, closed when the client
// disconnects. Exported so internal/graphqlgen (Resolve callbacks) and
// internal/gqltransport (Subscription transport) can construct one without
// a same-package helper -- gqlresolver holds the type, but building it is
// the dispatch layer's job, not this package's own.
func NewGraphqlContext(args execution.Parseable, done <-chan struct{}) *GraphqlContext {
	return &GraphqlContext{args: args, done: done}
}

// Args returns a Parseable built from the GraphQL request's own arguments
// map, consumable by gonest.MustParse[T]/Parse[T] exactly like
// req.Params()/req.Query()/req.Body().Json() already are for REST.
func (c *GraphqlContext) Args() execution.Parseable {
	return c.args
}

// Done returns a channel that closes when the underlying connection ends
// (Subscription's own cancellation signal, same idiom as context.Context's
// own Done()). For a Query/Mutation (request-response, no live connection
// to track) this is nil -- callers that only ever use Done() from inside a
// Subscription Handler never observe that case.
func (c *GraphqlContext) Done() <-chan struct{} {
	return c.done
}
