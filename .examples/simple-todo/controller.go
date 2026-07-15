package main

import (
	"net/http"

	"github.com/gonest-dev/gonest"
)

type createTodoBody struct {
	Title string `json:"title"`
}

var createTodoBodyMetadata = gonest.NewMetadata[createTodoBody](func(t *createTodoBody, m *gonest.Metadata) {
	m.Property(&t.Title).String().Min(1).Required()
})

type updateTodoBody struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

var updateTodoBodyMetadata = gonest.NewMetadata[updateTodoBody](func(t *updateTodoBody, m *gonest.Metadata) {
	m.Property(&t.Title).String().Min(1).Required()
	m.Property(&t.Done).Boolean()
})

type todoIDParams struct {
	ID int64 `param:"id"`
}

var todoIDParamsMetadata = gonest.NewMetadata[todoIDParams](func(t *todoIDParams, m *gonest.Metadata) {
	m.Property(&t.ID).Integer().Min(1).Required()
})

var TodoController = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/todos")

	service := gonest.MustInject[*TodoService](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.Context) {
			ctx.Json(service.List())
		})
	})

	controller.Route(gonest.HttpGet, "/:id", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*todoIDParams](ctx)
			todo := service.Get(p.ID)
			if todo == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(todo)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.HttpCode(http.StatusCreated)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*createTodoBody](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.Title))
		})
	})

	controller.Route(gonest.HttpPut, "/:id", func(r *gonest.Route) {
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*todoIDParams](ctx)
			body := gonest.MustJsonBody[*updateTodoBody](ctx)
			todo := service.Update(p.ID, body.Title, body.Done)
			if todo == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(todo)
		})
	})

	controller.Route(gonest.HttpDelete, "/:id", func(r *gonest.Route) {
		r.HttpCode(http.StatusNoContent)
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*todoIDParams](ctx)
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
