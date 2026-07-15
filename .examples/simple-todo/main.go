// Command simple-todo is the minimal gonest example: a single in-memory
// TodoService, one Controller, one Module, no external dependencies beyond
// gonest itself. No guards/interceptors/middleware/OpenAPI -- see
// .examples/blog-api for the dense counterpart exercising those.
//
// Run:
//
//	cd .examples/simple-todo && go run .
//
// (its own go.mod, replace-directed at the repo root -- keeps this
// example's dependencies isolated from the library's own go.mod/go.sum)
//
// Try:
//
//	curl -X POST localhost:3000/todos -d '{"title":"buy milk"}'
//	curl localhost:3000/todos
//	curl -X PUT localhost:3000/todos/1 -d '{"title":"buy milk","done":true}'
//	curl -X DELETE localhost:3000/todos/1
package main

import (
	"github.com/gonest-dev/gonest"
)

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
	app.MustListen(":3000", func() {
		println("simple-todo listening on :3000")
	})
}
