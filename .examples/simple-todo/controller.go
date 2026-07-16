package main

import (
	"net/http"

	"gonest.dev/gonest"
)

type createTodoBody struct {
	Title string `json:"title"`
}

var createTodoBodySchema = gonest.NewSchema[createTodoBody](func(t *createTodoBody, m *gonest.Schema) {
	m.Property(&t.Title).String().Min(1).Required()
})

type updateTodoBody struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

var updateTodoBodySchema = gonest.NewSchema[updateTodoBody](func(t *updateTodoBody, m *gonest.Schema) {
	m.Property(&t.Title).String().Min(1).Required()
	m.Property(&t.Done).Boolean()
})

type todoIDParams struct {
	ID int64 `param:"id"`
}

var todoIDParamsSchema = gonest.NewSchema[todoIDParams](func(t *todoIDParams, m *gonest.Schema) {
	m.Property(&t.ID).Integer().Min(1).Required()
})

var TodoController = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/todos")

	service := gonest.MustInject[*TodoService](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.RestContext) {
			ctx.Json(service.List())
		})
	})

	controller.Route(gonest.HttpGet, "/:id", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.RestContext) {
			p := gonest.MustParseRestParams[*todoIDParams](ctx, todoIDParamsSchema)
			todo := service.Get(p.ID)
			if todo == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(todo)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.HttpCode(http.StatusCreated)
		r.Handler(func(ctx *gonest.RestContext) {
			body := gonest.MustParseRestJsonBody[*createTodoBody](ctx, createTodoBodySchema)
			ctx.Status(http.StatusCreated).Json(service.Create(body.Title))
		})
	})

	controller.Route(gonest.HttpPut, "/:id", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.RestContext) {
			p := gonest.MustParseRestParams[*todoIDParams](ctx, todoIDParamsSchema)
			body := gonest.MustParseRestJsonBody[*updateTodoBody](ctx, updateTodoBodySchema)
			todo := service.Update(p.ID, body.Title, body.Done)
			if todo == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(todo)
		})
	})

	controller.Route(gonest.HttpDelete, "/:id", func(r *gonest.Route) {
		r.HttpCode(http.StatusNoContent)
		r.Handler(func(ctx *gonest.RestContext) {
			p := gonest.MustParseRestParams[*todoIDParams](ctx, todoIDParamsSchema)
			if !service.Delete(p.ID) {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Status(http.StatusNoContent).Json(nil)
		})
	})
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(TodoProvider)
	module.Controllers(TodoController)
})
