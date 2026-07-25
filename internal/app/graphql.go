package app

import (
	"encoding/json"

	gql "github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/validate"
)

// defaultGraphqlPath is used when Options.GraphqlPath is empty (the
// zero value) -- design.md's Tech Decisions left this configurable-or-not
// open, resolved now via Options.GraphqlPath.
const defaultGraphqlPath = "/graphql"

// graphqlEventStreamTokenHeader mirrors internal/graphql's own unexported
// sseSingleTokenHeader constant (ssesingle.go) -- this dispatcher only
// needs to know whether a graphql-sse Single connection mode token is
// PRESENT (header or query) to decide which handler to delegate to; the
// handlers themselves (already built, T11-T14) own the actual token
// lookup/validation logic. Kept as a literal rather than exporting
// graphql's constant to avoid growing that package's public surface for a
// single string only this file needs.
const graphqlEventStreamTokenHeader = "X-GraphQL-Event-Stream-Token"

// resolvableResolver is a locally-declared interface used to type-assert
// module.ResolverRef values down to the methods Stage 2.5-equivalent
// registration needs (OwnQueries/OwnMutations/OwnSubscriptions) but that
// module.ResolverRef itself does not expose -- same pattern as this
// package's own routableController for REST Controllers. Already
// implemented by *graphql.Resolver.
type resolvableResolver interface {
	OwnQueries() []*graphql.Query
	OwnMutations() []*graphql.Mutation
	OwnSubscriptions() []*graphql.Subscription
}

// registerGraphql collects every Query/Mutation/Subscription registered
// across every module's OwnResolvers(), builds the resulting
// *gql.Schema via internal/graphql.Build, and registers the 4 real-protocol
// routes (graphql-realtime-protocols feature, Milestone 18, T15) -- POST,
// PUT, GET, DELETE -- ALL on the same graphqlPath, replacing the previous
// POST-only + ad-hoc-WS/SSE-path registration. A no-op (no route
// registered at all) if no module registered any resolver -- an app that
// never uses GraphQL gets no extra endpoint, unchanged from before.
//
// See design.md's "Central Decision: Removing the /graphql Routing
// Collision" -- registering all 4 methods via the ordinary RegisterRoute
// (no app.Use-style middleware) is what makes this coexistence possible at
// all; each handler below is dispatched to purely by HTTP method, exactly
// like any other multi-method path in the framework.
func registerGraphql(adapter HttpAdapter, modules []*module.Module, opts Options) error {
	var queries []*graphql.Query
	var mutations []*graphql.Mutation
	var subscriptions []*graphql.Subscription

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

	sch, err := graphql.Build(queries, mutations, subscriptions)
	if err != nil {
		return err
	}

	graphqlPath := opts.GraphqlPath
	if graphqlPath == "" {
		graphqlPath = defaultGraphqlPath
	}

	subsByName := make(map[string]*graphql.Subscription, len(subscriptions))
	for _, s := range subscriptions {
		subsByName[s.Name()] = s
	}

	reg := graphql.NewReservationRegistry()

	if err := adapter.RegisterRoute(route.HttpPost, graphqlPath, graphqlPostDispatcher(sch, subsByName, reg)); err != nil {
		return err
	}
	if err := adapter.RegisterRoute(route.HttpPut, graphqlPath, graphql.SSESingleReserveHandler(reg)); err != nil {
		return err
	}
	if err := adapter.RegisterRoute(route.HttpGet, graphqlPath, graphqlGetDispatcher(sch, subsByName, reg)); err != nil {
		return err
	}
	return adapter.RegisterRoute(route.HttpDelete, graphqlPath, graphql.SSESingleCancelHandler(reg))
}

// requestCarriesEventStreamToken reports whether req carries a graphql-sse
// Single connection mode reservation token -- either via the
// X-GraphQL-Event-Stream-Token header or a ?token= query parameter, the
// same two locations every Single connection mode handler (ssesingle.go)
// itself accepts a token from. Used only to DISPATCH to the right handler
// below; the handlers themselves independently re-read and validate the
// token.
func requestCarriesEventStreamToken(req *execution.Request) bool {
	if req.Header(graphqlEventStreamTokenHeader) != "" {
		return true
	}
	return req.Queries()["token"] != ""
}

// graphqlPostBodyPeek is the minimal shape needed to detect a graphql-sse
// Single connection mode "start operation" POST (body carries
// extensions.operationId) vs the plain GraphQL-over-HTTP POST /graphql
// (graphqlRequestBody, below) -- both share the same query/variables/
// operationName triple, so only extensions.operationId's presence tells
// them apart.
type graphqlPostBodyPeek struct {
	Extensions struct {
		OperationId string `json:"operationId"`
	} `json:"extensions"`
}

// graphqlPostDispatcher builds the POST graphqlPath handler: a
// graphql-sse Single connection mode "start operation" request (token
// present AND body carries extensions.operationId) delegates to
// graphql.SSESingleOperationHandler; every other POST is the original
// plain GraphQL-over-HTTP JSON dispatch, unchanged, via graphqlHandler.
//
// The body is peeked at (json.Unmarshal into graphqlPostBodyPeek) before
// deciding which branch to take -- both branches re-parse/re-read the body
// themselves afterwards (graphqlHandler and SSESingleOperationHandler each
// wire their own BodySource via req.WithSources and read req.Body().Raw()
// independently), so this peek does not consume or share any parsing
// state with either.
func graphqlPostDispatcher(sch *gql.Schema, subsByName map[string]*graphql.Subscription, reg *graphql.ReservationRegistry) func(c *execution.HttpContext) {
	plainHandler := graphqlHandler(sch)
	singleOperationHandler := graphql.SSESingleOperationHandler(sch, subsByName, reg)

	return func(c *execution.HttpContext) {
		req := c.Request()
		if requestCarriesEventStreamToken(req) {
			// Body().Raw() needs a BodySource wired first (req.res.RawBody()
			// is only reachable through it -- see BodySource.Raw()'s own doc
			// comment: a zero-value BodySource, which req.Body() returns
			// before any WithSources call, always returns nil). Both branches
			// below re-wire WithSources themselves (graphqlHandler and
			// SSESingleOperationHandler each do, independently) -- wiring it
			// again here first is redundant but harmless, and lets this peek
			// read the real body without duplicating BodySource's own
			// plumbing.
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

			var peek graphqlPostBodyPeek
			if err := json.Unmarshal(req.Body().Raw(), &peek); err == nil && peek.Extensions.OperationId != "" {
				singleOperationHandler(c)
				return
			}
		}
		plainHandler(c)
	}
}

// graphqlGetDispatcher builds the GET graphqlPath handler: a WebSocket
// upgrade handshake delegates to graphql.WSProtocolHandler (the real
// graphql-transport-ws state machine); otherwise, a graphql-sse Single
// connection mode token (header/query) delegates to
// graphql.SSESingleConnectHandler; otherwise (GraphQL-over-HTTP GET with
// Accept: text/event-stream, or any other GET) falls through to
// graphql.SSEDistinctHandler.
func graphqlGetDispatcher(sch *gql.Schema, subsByName map[string]*graphql.Subscription, reg *graphql.ReservationRegistry) func(c *execution.HttpContext) {
	wsHandler := graphql.WSProtocolHandler(sch, subsByName)
	connectHandler := graphql.SSESingleConnectHandler(reg)
	distinctHandler := graphql.SSEDistinctHandler(sch, subsByName)

	return func(c *execution.HttpContext) {
		req, res := c.Request(), c.Response()
		if req.IsWebSocketUpgrade() {
			res.UpgradeWebSocket(wsHandler, "graphql-transport-ws")
			return
		}
		if requestCarriesEventStreamToken(req) {
			connectHandler(c)
			return
		}
		distinctHandler(c)
	}
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

// graphqlHandler builds the plain POST /graphql HTTP handler: decode the
// standard GraphQL-over-HTTP body, execute it against sch via
// graphql.Execute (Query/Mutation dispatch happens INSIDE gql.Do, which
// graphql.Execute wraps -- see internal/graphql's own Resolve callbacks,
// wired at Build time), write {data, errors} back.
// Subscription requests are never dispatched here -- they connect via
// WS/SSE (graphql-support and graphql-realtime-protocols features), a
// genuinely different transport, not this JSON-over-HTTP endpoint.
func graphqlHandler(sch *gql.Schema) func(c *execution.HttpContext) {
	return func(c *execution.HttpContext) {
		req, res := c.Request(), c.Response()
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

		data, execErrs := graphql.Execute(sch, body.Query, body.Variables, body.OperationName)

		var errs []any
		for _, e := range execErrs {
			errs = append(errs, e)
		}

		res.Json(graphqlResponseBody{Data: data, Errors: errs})
	}
}
