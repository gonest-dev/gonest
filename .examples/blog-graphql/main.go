// Command blog-graphql is the GraphQL-focused gonest example: one
// GraphqlResolver exposing Query/Mutation/Subscription over an in-memory
// Post store, reusing the exact same Schema/Parse[T]/MustInject REST
// already uses (see .examples/blog-api for the REST-heavy example this
// one deliberately stays separate from).
//
// Run:
//
//	cd .examples/blog-graphql && go run .
//
// Try:
//
//	# Query
//	curl -X POST localhost:3002/graphql -d '{"query":"{ posts { id title body } }"}'
//
//	# Mutation
//	curl -X POST localhost:3002/graphql -d '{"query":"mutation { createPost(title: \"Hello\", body: \"World\") { id title } }"}'
//
//	# Query by id
//	curl -X POST localhost:3002/graphql -d '{"query":"{ post(id: 1) { id title body } }"}'
//
//	# Subscription (SSE) -- open in one terminal, then run a createPost
//	# Mutation in another and watch the event arrive here:
//	curl -N localhost:3002/graphql/stream/postCreated
//
//	# Subscription (WebSocket), e.g. via websocat:
//	websocat ws://localhost:3002/graphql/ws/postCreated
package main

import "gonest.dev/gonest"

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule)
	app.MustListen(":3002")
}
