// Command full-text-search is a gonest example: a Person CRUD plus a
// generic POST /person/_search endpoint (text/where/fields/sort/offset/
// limit; Elasticsearch-style path, not the HTTP QUERY method -- OpenAPI has
// no representation for QUERY, see person/controller.go's own doc comment
// on that route), modeled after github.com/leandroluk's Search.ts gist --
// exercises
// gonest.Schema nested Object()/Array() refs, gonest.Accessor dirty-tracking
// in both a write DTO (BodyCreateDTO/BodyUpdateDTO) and a read/filter DTO
// (person.QueryDTOWhere), Go generics (search.Query[T]/Fields[T]/
// SortField[T]/Result[I]), the real Enum(...) constraint (Milestone 21) on
// Fields.Select/Remove and SortField.Field, and a full OpenAPI/Swagger
// document generated from all of the above.
//
// Run:
//
//	cd .examples/full-text-search && go run .
//
// Try:
//
//	curl -X POST localhost:3002/person -d '{"name":"Ada","age":30,"is_active":true}'
//	curl localhost:3002/person/<id>
//	curl -X PUT localhost:3002/person/<id> -d '{"age":31}'
//	curl -X DELETE localhost:3002/person/<id>
//	curl -X POST localhost:3002/person/_search -d '{"where":{"age":{"gte":18}},"sort":[{"field":"age","order":-1}],"limit":10}'
//	curl localhost:3002/docs   (Swagger UI, no auth required)
package main

import (
	"gonest.dev/gonest"

	"full-text-search/person"
)

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(person.Module)
	module.Use(gonest.NewLoggerMiddleware())
})

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule)

	doc := gonest.NewOpenAPI("3.1.0", func(b *gonest.OpenAPI) {
		b.Title("Full-Text Search Example")
		b.Description("gonest dogfood example: Person CRUD + a generic, entity-agnostic search.Query[T]/Where/Fields[T]/SortField[T]/Result[I] API")
		b.Version("1.0.0")
	})
	gonest.OpenapiGenerate(app, doc)
	if err := gonest.SetupSwagger(app, "/docs", doc, gonest.SwaggerOptions{
		JsonDocumentUrl: "/openapi.json",
		DocExpansion:    "list",
	}); err != nil {
		panic(err)
	}

	app.MustListen(":3002")
}
