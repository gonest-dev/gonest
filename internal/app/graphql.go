package app

import (
	"encoding/json"

	"github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/gqltransport"
	"gonest.dev/gonest/internal/graphqlgen"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/validate"
)

// graphqlPath is the fixed HTTP endpoint every registered GraphQL
// Query/Mutation dispatches through (graphql-support feature, Milestone
// 17). Not yet configurable via AppOptions -- design.md's Tech Decisions
// left this open, a fixed default is enough for the MVP.
const graphqlPath = "/graphql"

// graphqlSubscriptionPath is the SSE transport's own endpoint (T9) --
// :name selects which registered Subscription to stream. WebSocket (T10)
// uses a sibling path, registered separately.
const graphqlSubscriptionPath = "/graphql/stream/:name"

// resolvableResolver is a locally-declared interface used to type-assert
// module.ResolverRef values down to the methods Stage 2.5-equivalent
// registration needs (OwnQueries/OwnMutations/OwnSubscriptions) but that
// module.ResolverRef itself does not expose -- same pattern as this
// package's own routableController for REST Controllers. Already
// implemented by *gqlresolver.Resolver.
type resolvableResolver interface {
	OwnQueries() []*gqlresolver.Query
	OwnMutations() []*gqlresolver.Mutation
	OwnSubscriptions() []*gqlresolver.Subscription
}

// registerGraphql collects every Query/Mutation/Subscription registered
// across every module's OwnResolvers(), builds the resulting
// *graphql.Schema via internal/graphqlgen.Build, and registers ONE POST
// /graphql route dispatching through it. A no-op (no route registered) if
// no module registered any resolver at all -- an app that never uses
// GraphQL gets no extra endpoint.
func registerGraphql(adapter HttpAdapter, modules []*module.Module) error {
	var queries []*gqlresolver.Query
	var mutations []*gqlresolver.Mutation
	var subscriptions []*gqlresolver.Subscription

	for _, m := range modules {
		for _, r := range m.OwnResolvers() {
			rr, ok := r.(resolvableResolver)
			if !ok {
				continue
			}
			queries = append(queries, rr.OwnQueries()...)
			mutations = append(mutations, rr.OwnMutations()...)
			subscriptions = append(subscriptions, rr.OwnSubscriptions()...)
		}
	}

	if len(queries) == 0 && len(mutations) == 0 && len(subscriptions) == 0 {
		return nil
	}

	sch, err := graphqlgen.Build(queries, mutations, subscriptions)
	if err != nil {
		return err
	}

	if err := adapter.RegisterRoute(route.HttpPost, graphqlPath, graphqlHandler(sch)); err != nil {
		return err
	}

	if len(subscriptions) == 0 {
		return nil
	}

	subsByName := make(map[string]*gqlresolver.Subscription, len(subscriptions))
	for _, s := range subscriptions {
		subsByName[s.Name()] = s
	}
	return adapter.RegisterRoute(route.HttpGet, graphqlSubscriptionPath, gqltransport.SSEHandler(subsByName))
}

// graphqlRequestBody is the standard GraphQL-over-HTTP request shape
// (query/variables/operationName) -- a well-established convention (used
// verbatim by Apollo Client, urql, GraphiQL, etc), not fabricated here.
type graphqlRequestBody struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
}

// graphqlResponseBody is GraphQL's own standard response shape --
// {data, errors}, both fields present (errors omitted when empty via
// omitempty, matching the spec's own "errors entry is optional" wording).
type graphqlResponseBody struct {
	Data   any   `json:"data,omitempty"`
	Errors []any `json:"errors,omitempty"`
}

// graphqlHandler builds the POST /graphql HTTP handler: decode the
// standard GraphQL-over-HTTP body, execute it against sch via graphql.Do
// (Query/Mutation dispatch happens INSIDE graphql.Do -- see
// internal/graphqlgen's own Resolve callbacks, wired at Build time), write
// {data, errors} back. Subscription requests are never dispatched here --
// they connect via SSE/WebSocket (T9/T10, internal/gqltransport), a
// genuinely different transport, not this JSON-over-HTTP endpoint.
func graphqlHandler(sch *graphql.Schema) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		// Same BodySource wiring registerRoutes' own withRoute closure does
		// for every REST route -- req.Body().Raw() (and, transitively,
		// GraphqlContext.Args()' MustParse[T] calls) need it, and this
		// route is registered directly via adapter.RegisterRoute, bypassing
		// that closure entirely.
		req.WithSources(
			validate.NewParamsSource(req),
			validate.NewQuerySource(req),
			validate.NewHeadersSource(req),
			execution.NewBodySource(
				req,
				func() execution.Parseable { return validate.NewJSONBodySource(req) },
				func(onFile func(*execution.FormFile) error) execution.Parseable {
					return validate.NewFormBodySource(req, onFile)
				},
			),
		)

		var body graphqlRequestBody
		if err := json.Unmarshal(req.Body().Raw(), &body); err != nil {
			res.Status(400)
			res.Json(graphqlResponseBody{Errors: []any{map[string]any{"message": "invalid GraphQL request body: " + err.Error()}}})
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:         *sch,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			OperationName:  body.OperationName,
		})

		var errs []any
		for _, e := range result.Errors {
			errs = append(errs, map[string]any{"message": e.Message})
		}

		res.Json(graphqlResponseBody{Data: result.Data, Errors: errs})
	}
}
