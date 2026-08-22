// Command blog-graphql is the GraphQL-focused gonest example: one
// GraphqlResolver exposing Query/Mutation/Subscription over an in-memory
// Post store, reusing the exact same Schema/Parse[T]/MustInject REST
// already uses (see .examples/blog-api for the REST-heavy example this
// one deliberately stays separate from).
//
// It also demonstrates all 3 real-protocol transports the
// graphql-realtime-protocols feature (Milestone 18) exposes, all on the
// SAME /graphql path, dispatched purely by HTTP method/headers -- see
// README.md in this directory for the full walkthrough of each one
// (graphql-transport-ws over WebSocket, graphql-sse Distinct connections
// mode, graphql-sse Single connection mode).
//
// Run:
//
//	cd .examples/blog-graphql && go run .
//
// Quick tries (see README.md for the WS and SSE transports):
//
//	# Query
//	curl -X POST localhost:3002/graphql -d '{"query":"{ posts { id title body } }"}'
//
//	# Mutation
//	curl -X POST localhost:3002/graphql -d '{"query":"mutation { createPost(title: \"Hello\", body: \"World\") { id title } }"}'
//
//	# Query by id
//	curl -X POST localhost:3002/graphql -d '{"query":"{ post(id: 1) { id title body } }"}'
package main

import "gonest.dev/gonest"

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule)
	app.MustListen(":3002")
}
