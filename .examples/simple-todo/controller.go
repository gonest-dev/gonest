package main

import (
	"net/http"

	"gonest.dev/gonest"
)

type createTodoBody struct {
	Title string `json:"title"`
}

var createTodoBodySchema = gonest.NewSchema(func(t *createTodoBody, s *gonest.Schema) {
	s.Property(&t.Title).String().Min(1).Required()
})

type updateTodoBody struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

var updateTodoBodySchema = gonest.NewSchema(func(t *updateTodoBody, s *gonest.Schema) {
	s.Property(&t.Title).String().Min(1).Required()
	s.Property(&t.Done).Boolean()
})

type todoIDParams struct {
	ID int64 `param:"id"`
}

var todoIDParamsSchema = gonest.NewSchema(func(t *todoIDParams, s *gonest.Schema) {
	s.Property(&t.ID).Integer().Min(1).Required()
})

var TodoController = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/todos")

	service := gonest.MustInject[*TodoService](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Handler(func(c *gonest.HttpContext) {
			res := c.Response()
			res.Json(service.List())
		})
	})

	// This route resolves *TodoService itself, directly, via
	// gonest.MustInject[T](r) -- not through the Controller-level `service`
	// variable above -- proving Route resolves from the exact same
	// module scope a Controller would (route-must-inject feature). Only
	// this one route pays for building a TodoStatsUsecase; every other
	// route on TodoController never constructs one.
	controller.Route(gonest.HttpGet, "/stats", func(r *gonest.Route) {
		statsUsecase := &TodoStatsUsecase{service: gonest.MustInject[*TodoService](r)}
		r.Handler(func(c *gonest.HttpContext) {
			c.Response().Json(statsUsecase.Run())
		})
	})

	controller.Route(gonest.HttpGet, "/:id", func(r *gonest.Route) {
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			p := gonest.MustParse[todoIDParams](req.Params(), todoIDParamsSchema)
			todo := service.Get(p.ID)
			if todo == nil {
				panic(gonest.NewNotFoundException("", nil))
			}
			res.Json(todo)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.HttpCode(http.StatusCreated)
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			body := gonest.MustParse[createTodoBody](req.Body().Json(), createTodoBodySchema)
			res.Status(http.StatusCreated).Json(service.Create(body.Title))
		})
	})

	controller.Route(gonest.HttpPut, "/:id", func(r *gonest.Route) {
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			p := gonest.MustParse[todoIDParams](req.Params(), todoIDParamsSchema)
			body := gonest.MustParse[updateTodoBody](req.Body().Json(), updateTodoBodySchema)
			todo := service.Update(p.ID, body.Title, body.Done)
			if todo == nil {
				panic(gonest.NewNotFoundException("", nil))
			}
			res.Json(todo)
		})
	})

	controller.Route(gonest.HttpDelete, "/:id", func(r *gonest.Route) {
		r.HttpCode(http.StatusNoContent)
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			p := gonest.MustParse[todoIDParams](req.Params(), todoIDParamsSchema)
			if !service.Delete(p.ID) {
				panic(gonest.NewNotFoundException("", nil))
			}
			res.Status(http.StatusNoContent).Json(nil)
		})
	})
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(TodoProvider)
	module.Controllers(TodoController)
})
