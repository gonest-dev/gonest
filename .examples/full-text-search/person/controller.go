package person

import (
	"net/http"

	"gonest.dev/gonest"

	"full-text-search/shared/entity"
	"full-text-search/shared/search"
)

// resultSchema documents the POST /person/_search response body
// (Result[*Person]), built once from entity.PersonSchema (the same response
// schema GET/POST/PUT already reference for a single Person). No explicit
// Title() here -- ResultSchemaFor defaults to entity.PersonSchema's own
// title + "Result" ("PersonEntityResult"), already unique per entity, so
// this fn only needs to exist to satisfy the callback signature.
var resultSchema = search.ResultSchemaFor[*entity.Person](entity.PersonSchema, func(t *search.Result[*entity.Person], s *gonest.Schema) {})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/person")
	controller.Tags("person")

	service := gonest.MustInject[*Service](controller)

	// POST /person/_search (Elasticsearch-style path), not the HTTP QUERY
	// method (gonest.HttpQuery/RouteQuery) -- OpenAPI's Path Item Object
	// (3.0 AND 3.1) only defines fixed operation fields for the standard
	// verbs (get/put/post/delete/options/head/patch/trace); QUERY is still
	// an IETF draft RFC with no OpenAPI representation at all, so a route
	// registered under it never shows up in Swagger UI (silently ignored --
	// found live testing /docs against this same example). A real POST to
	// a dedicated sub-path sidesteps the gap entirely instead of working
	// around it in the generated doc.
	controller.RoutePost("/_search", func(r *gonest.Route) {
		r.Summary("Search person entity using query")
		r.Description(
			"Generic query endpoint, modeled after github.com/leandroluk's Search.ts gist --",
			"exercises gonest.Schema nested Object()/Array() refs, Enum-constrained Fields.Select/Remove",
			"and Sort.Field, and Accessor dirty-tracking on both write DTOs and the Where filter.")
		r.RequestBody(QueryDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.RouteResponse) { response.Schema(resultSchema) })
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			q := gonest.MustParse[QueryDTO](req.Body().Json(), QueryDTOSchema)
			res.Json(service.Search(q))
		})
	})

	controller.RouteGet("/:person_id", func(r *gonest.Route) {
		r.Summary("Get a person by id")
		r.Params(ParamsDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.RouteResponse) { response.Schema(entity.PersonSchema) })
		r.Response(http.StatusNotFound, func(response *gonest.RouteResponse) {
			response.Description("Cannot find a person using person_id")
		})
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			p := gonest.MustParse[ParamsDTO](req.Params(), ParamsDTOSchema)
			person := service.Get(p.PersonID)
			if person == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			res.Json(person)
		})
	})

	controller.RoutePut("/:person_id", func(r *gonest.Route) {
		r.Summary("Update a person (only fields present in the body are applied)")
		r.Params(ParamsDTOSchema)
		r.RequestBody(BodyUpdateDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.RouteResponse) { response.Schema(entity.PersonSchema) })
		r.Response(http.StatusNotFound, func(response *gonest.RouteResponse) {
			response.Description("Cannot find a person using person_id")
		})
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			p := gonest.MustParse[ParamsDTO](req.Params(), ParamsDTOSchema)
			body := gonest.MustParse[BodyUpdateDTO](req.Body().Json(), BodyUpdateDTOSchema)
			person := service.Update(p.PersonID, body)
			if person == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			res.Json(person)
		})
	})

	controller.RouteDelete("/:person_id", func(r *gonest.Route) {
		r.Summary("Delete a person")
		r.Params(ParamsDTOSchema)
		r.HttpCode(http.StatusNoContent)
		r.Response(http.StatusNoContent, func(response *gonest.RouteResponse) {
			response.Description("Person deleted")
		})
		r.Response(http.StatusNotFound, func(response *gonest.RouteResponse) {
			response.Description("Cannot find a person using person_id")
		})
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			p := gonest.MustParse[ParamsDTO](req.Params(), ParamsDTOSchema)
			if !service.Delete(p.PersonID) {
				panic(gonest.NewNotFoundException(nil))
			}
			res.Status(http.StatusNoContent)
		})
	})

	controller.RoutePost("/", func(r *gonest.Route) {
		r.Summary("Create a person")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(BodyCreateDTOSchema)
		r.Response(http.StatusCreated, func(response *gonest.RouteResponse) { response.Schema(entity.PersonSchema) })
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			body := gonest.MustParse[BodyCreateDTO](req.Body().Json(), BodyCreateDTOSchema)
			res.Status(http.StatusCreated).Json(service.Create(body))
		})
	})
})
