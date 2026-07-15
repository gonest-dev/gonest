// Command blog-api is the DENSE gonest example: guards, interceptors,
// middleware, a dev-defined exception + filter, OpenAPI/Swagger, SQLite
// persistence, and a real 3-module domain (User has many Posts, Post has
// many Comments, User has many Comments -- simulating an M:N shape via 2
// foreign keys on comments, no explicit join table).
//
// Run:
//
//	cd .examples/blog-api && go run .
//
// (its own go.mod, replace-directed at the repo root -- keeps
// modernc.org/sqlite and this example's other dependencies isolated from
// the library's own go.mod/go.sum, since a dot-prefixed directory is
// invisible to `go build/test/mod tidy ./...` at the repo root anyway)
//
// Try (Authorization header required on every route, per shared.AuthGuard):
//
//	curl -H "Authorization: Bearer demo-token" -X POST localhost:3001/users -d '{"name":"Ada","email":"ada@example.com"}'
//	curl -H "Authorization: Bearer demo-token" -X POST localhost:3001/posts -d '{"user_id":1,"title":"Hello","body":"World"}'
//	curl -H "Authorization: Bearer demo-token" -X POST localhost:3001/comments -d '{"post_id":1,"user_id":1,"body":"Nice post!"}'
//	curl -H "Authorization: Bearer demo-token" localhost:3001/comments?post_id=1
//	curl localhost:3001/docs   (Swagger UI, no auth required)
package main

import (
	"github.com/gonest-dev/gonest"

	"blog-api/module/comment"
	"blog-api/module/post"
	"blog-api/module/user"
	"blog-api/shared"
)

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(user.Module, post.Module, comment.Module)
	// Global (root-only) registrations -- Middleware/Filter cascade to
	// every route in the whole tree; Guard/Interceptor stay per-controller
	// (no Module-level global registration exists for those 2 today).
	module.Use(shared.RequestIDMiddleware)
	module.Filters(shared.DomainFilter)
})

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})

	doc := gonest.NewOpenAPI("3.1.0", func(b *gonest.OpenAPI) {
		b.Title("Blog API")
		b.Description("Dense gonest dogfood example: User/Post/Comment over SQLite")
		b.Version("1.0.0")
		b.BearerAuth()
	})
	gonest.GenerateOpenApiSchema(app, doc)
	if err := gonest.SetupSwagger(app, "/docs", doc, gonest.SwaggerOptions{
		JsonDocumentUrl: "/openapi.json",
		PersistAuth:     true,
		DocExpansion:    "none",
	}); err != nil {
		panic(err)
	}

	app.MustListen(":3001")
}
