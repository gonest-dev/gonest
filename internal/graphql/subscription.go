package graphql

import "gonest.dev/gonest/internal/schema"

// Subscription represents one declared GraphQL subscription field.
// Genuinely distinct from Query/Mutation (design.md's D3, context.md):
// Query/Mutation are request-response (Handler returns ONE value and
// ends); Subscription is a STREAM -- the resolver stays alive emitting N
// values while the client remains connected, so it needs its own Handler
// signature (func(ctx, emit), not func(ctx) any).
type Subscription struct {
	name    string
	args    *schema.Schema
	returns *schema.Schema
	handler func(ctx *Context, emit func(any))
}

// newSubscription creates a *Subscription and runs fn on it immediately --
// same deferred-run-immediately shape as newQuery.
func newSubscription(name string, fn func(*Subscription)) *Subscription {
	s := &Subscription{name: name}
	if fn != nil {
		fn(s)
	}
	return s
}

// Name returns this Subscription's field name, as passed to
// Resolver.Subscription.
func (s *Subscription) Name() string {
	return s.name
}

// Args stores schema as this Subscription's argument schema and returns s
// so calls can chain.
func (s *Subscription) Args(schema *schema.Schema) *Subscription {
	s.args = schema
	return s
}

// ArgsSchema returns the schema stored via Args, or nil if Args was never
// called.
func (s *Subscription) ArgsSchema() *schema.Schema {
	return s.args
}

// Returns stores schema as this Subscription's emitted-value schema and
// returns s so calls can chain.
func (s *Subscription) Returns(schema *schema.Schema) *Subscription {
	s.returns = schema
	return s
}

// ReturnsSchema returns the schema stored via Returns, or nil if Returns
// was never called.
func (s *Subscription) ReturnsSchema() *schema.Schema {
	return s.returns
}

// Handler stores fn as this Subscription's streaming resolve function and
// returns s so calls can chain. emit publishes one value at a time to the
// connected client; fn only returns when the subscription ends (typically
// by observing <-ctx.Done()).
func (s *Subscription) Handler(fn func(ctx *Context, emit func(any))) *Subscription {
	s.handler = fn
	return s
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler
// was never called.
func (s *Subscription) HandlerFunc() func(ctx *Context, emit func(any)) {
	return s.handler
}
