package gqlresolver

// Mutation is a true Go type alias of Query (design.md's Tech Decisions):
// both share the exact same fields/methods today (name, args schema,
// returns schema, Handler(ctx) any) -- the GraphQL root type a field ends
// up under (Query vs Mutation in the generated SDL) is decided by which of
// Resolver.Query/Resolver.Mutation registered it, not by a distinct Go
// type. If Mutation ever needs behavior Query doesn't have, this becomes a
// real struct at that point -- not before (YAGNI).
type Mutation = Query

// newMutation is newQuery under a Mutation-shaped name, kept as a separate
// function (rather than calling newQuery directly from Resolver.Mutation)
// so a future divergence between the two only requires changing this one
// function, not every call site.
func newMutation(name string, fn func(*Mutation)) *Mutation {
	return newQuery(name, fn)
}
