// Package gqlresolver implements the declarative GraphQL Resolver API
// (graphql-support feature, Milestone 17) -- analogous to internal/
// controller's Controller for REST. Deliberately NOT named "resolver" at
// the package level: internal/resolver already exists (DI search helpers,
// FindDirect/FindDirectAll) and is an unrelated concept -- see design.md's
// "Naming Collision Note". Resolver reuses internal/resolver's search
// functions internally, it just doesn't share its package name.
package gqlresolver

import (
	"reflect"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Resolver represents a declarative unit that consumes providers via
// MustInject, analogous to Controller but exposing GraphQL Query/Mutation/
// Subscription fields instead of REST routes.
//
// New(fn) does not execute fn at call time -- fn is deferred until
// bootstrap (Stage 2, mirroring Controller's own Declare timing) runs it,
// since that is the point at which MustInject calls inside fn need a known
// owner module to resolve scope against.
type Resolver struct {
	fn func(*Resolver)

	owner    *module.Module
	declared bool

	queries       []*Query
	mutations     []*Mutation
	subscriptions []*Subscription
}

// New creates a Resolver that defers fn until bootstrap runs it.
func New(fn func(*Resolver)) *Resolver {
	return &Resolver{fn: fn}
}

// Declare runs this resolver's deferred fn exactly once -- a no-op on any
// call after the first (including when fn is nil), mirroring Controller.
// Declare's own idempotency contract.
func (r *Resolver) Declare() {
	if r.declared {
		return
	}
	r.declared = true
	if r.fn != nil {
		r.fn(r)
	}
}

// IsResolver is the marker method that satisfies module.ResolverRef, so
// *Resolver can be passed to (*module.Module).Resolvers. Exported for the
// same reason Controller.IsController is exported: Go ties unexported
// interface methods to the declaring package.
func (r *Resolver) IsResolver() {}

// SetOwnerModule associates this resolver with the module that owns it.
func (r *Resolver) SetOwnerModule(m *module.Module) {
	r.owner = m
}

// OwnerModule implements module.Owner. Returns nil until SetOwnerModule
// has been called.
func (r *Resolver) OwnerModule() *module.Module {
	return r.owner
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoping the search to this Resolver's own OwnerModule -- same
// single-module scope Controller.ResolveDirect already uses.
func (r *Resolver) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{r.owner}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (r *Resolver) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{r.owner}, t)
}

// Query creates a *Query via query.New(name, fn) -- which runs fn
// immediately, see Query's own doc comment -- and appends it to this
// resolver's internal query list.
func (r *Resolver) Query(name string, fn func(*Query)) {
	r.queries = append(r.queries, newQuery(name, fn))
}

// Mutation creates a *Mutation via mutation.New(name, fn) and appends it to
// this resolver's internal mutation list.
func (r *Resolver) Mutation(name string, fn func(*Mutation)) {
	r.mutations = append(r.mutations, newMutation(name, fn))
}

// Subscription creates a *Subscription via subscription.New(name, fn) and
// appends it to this resolver's internal subscription list.
func (r *Resolver) Subscription(name string, fn func(*Subscription)) {
	r.subscriptions = append(r.subscriptions, newSubscription(name, fn))
}

// OwnQueries returns a copy of every *Query registered so far via Query.
// Read-only: mutating the returned slice does not affect this Resolver's
// internal state (same defensive-copy pattern as Controller.OwnRoutes).
func (r *Resolver) OwnQueries() []*Query {
	return append([]*Query(nil), r.queries...)
}

// OwnMutations returns a copy of every *Mutation registered so far via
// Mutation. Same defensive-copy pattern as OwnQueries.
func (r *Resolver) OwnMutations() []*Mutation {
	return append([]*Mutation(nil), r.mutations...)
}

// OwnSubscriptions returns a copy of every *Subscription registered so far
// via Subscription. Same defensive-copy pattern as OwnQueries.
func (r *Resolver) OwnSubscriptions() []*Subscription {
	return append([]*Subscription(nil), r.subscriptions...)
}
