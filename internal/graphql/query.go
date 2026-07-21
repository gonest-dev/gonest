package graphql

import "gonest.dev/gonest/internal/schema"

// Query represents one declared GraphQL query field -- name, args schema,
// return schema, handler. Mutation is a type alias of Query (see this
// package's mutation.go): both have IDENTICAL shape today (design.md's
// Tech Decisions, "minimiza código novo até haver motivo real de
// divergência" -- only Subscription's Handler signature genuinely differs,
// see subscription.go), the GraphQL root type they end up under (Query vs
// Mutation) is decided by which list Resolver.Query/Mutation appended them
// to, not by their own Go type.
type Query struct {
	name        string
	args        *schema.Schema
	returns     *schema.Schema
	returnsList bool
	handler     func(ctx *Context) any
}

// newQuery creates a *Query and runs fn on it immediately -- same
// deferred-run-immediately shape as route.New (by the time Resolver.
// Query(...) calls newQuery, we're already inside the Resolver's own
// deferred fn, so there is no further stage left to usefully defer to).
func newQuery(name string, fn func(*Query)) *Query {
	q := &Query{name: name}
	if fn != nil {
		fn(q)
	}
	return q
}

// Name returns this Query's field name, as passed to Resolver.Query.
func (q *Query) Name() string {
	return q.name
}

// Args stores s as this Query's argument schema and returns q so calls can
// chain. Validated via gonest.MustParse[T](ctx.Args(), s) exactly like a
// REST Route's RequestBody schema.
func (q *Query) Args(s *schema.Schema) *Query {
	q.args = s
	return q
}

// ArgsSchema returns the schema stored via Args, or nil if Args was never
// called.
func (q *Query) ArgsSchema() *schema.Schema {
	return q.args
}

// Returns stores s as this Query's return-value schema (used by
// internal/graphqlgen to generate the corresponding GraphQL type) and
// returns q so calls can chain.
func (q *Query) Returns(s *schema.Schema) *Query {
	q.returns = s
	return q
}

// ReturnsSchema returns the schema stored via Returns, or nil if Returns
// was never called.
func (q *Query) ReturnsSchema() *schema.Schema {
	return q.returns
}

// ReturnsList stores s as this Query's return-value schema, same as
// Returns, but declares the field's GraphQL type as a LIST of s (`[X!]`)
// instead of a bare object -- for a Handler returning a Go slice (e.g.
// `service.List()`), not a single value. Returns q so calls can chain.
func (q *Query) ReturnsList(s *schema.Schema) *Query {
	q.returns = s
	q.returnsList = true
	return q
}

// ReturnsIsList reports whether ReturnsList (rather than Returns) was
// used to declare this Query's return schema.
func (q *Query) ReturnsIsList() bool {
	return q.returnsList
}

// Handler stores fn as this Query's resolve function and returns q so
// calls can chain. fn's return value directly becomes the GraphQL
// response's data -- no separate Response/write-side, unlike REST's
// Route.Handler (context.md's D2: GraphQL resolvers have no status/headers
// to justify one).
func (q *Query) Handler(fn func(ctx *Context) any) *Query {
	q.handler = fn
	return q
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler
// was never called.
func (q *Query) HandlerFunc() func(ctx *Context) any {
	return q.handler
}
