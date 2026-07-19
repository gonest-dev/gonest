package main

import "gonest.dev/gonest"

// PostResolver is the GraphQL-only counterpart of a REST Controller --
// registered via module.Resolvers(...), consuming dependencies through
// MustInject exactly like a Controller does. Query/Mutation reuse the
// EXACT SAME Schema (postSchema) OpenAPI/REST would use for the response
// body; Args (createPostArgsSchema/postIDArgsSchema) are validated via the
// SAME gonest.MustParse[T] REST already uses, just fed by ctx.Args()
// instead of req.Body().Json()/req.Params().
var PostResolver = gonest.NewGraphqlResolver(func(resolver *gonest.GraphqlResolver) {
	service := gonest.MustInject[*Service](resolver)
	emitter := gonest.MustInject[*gonest.Emitter](resolver)

	resolver.Query("posts", func(query *gonest.GraphqlQuery) {
		query.ReturnsList(postSchema) // Handler returns a []*PostEntity, not a single value
		query.Handler(func(ctx *gonest.GraphqlContext) any {
			return service.List()
		})
	})

	resolver.Query("post", func(query *gonest.GraphqlQuery) {
		query.Args(postIDArgsSchema)
		query.Returns(postSchema)
		query.Handler(func(ctx *gonest.GraphqlContext) any {
			args := gonest.MustParse[PostIDArgs](ctx.Args(), postIDArgsSchema)
			p := service.Get(args.ID)
			if p == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			return p
		})
	})

	resolver.Mutation("createPost", func(mutation *gonest.GraphqlMutation) {
		mutation.Args(createPostArgsSchema)
		mutation.Returns(postSchema)
		mutation.Handler(func(ctx *gonest.GraphqlContext) any {
			args := gonest.MustParse[CreatePostArgs](ctx.Args(), createPostArgsSchema)
			return service.Create(args.Title, args.Body)
		})
	})

	// Subscription: a STREAM, not request-response -- Handler's signature
	// is (ctx, emit), not (ctx) any. gonest.Subscribe[T](emitter, done) is
	// Emitter's dynamic counterpart to NewListener/Emit: a channel that lives
	// only as long as ctx.Done() stays open (this ONE connection),
	// cancelled automatically when the client disconnects (SSE: write
	// failure/heartbeat detects it; WebSocket: the read loop detects it).
	resolver.Subscription("postCreated", func(subscription *gonest.GraphqlSubscription) {
		subscription.Returns(postSchema)
		subscription.Handler(func(ctx *gonest.GraphqlContext, emit func(any)) {
			for event := range gonest.Subscribe[PostCreatedEvent](emitter, ctx.Done()) {
				emit(event.Post)
			}
		})
	})
})
