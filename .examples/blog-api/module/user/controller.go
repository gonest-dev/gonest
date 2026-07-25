package user

import (
	"net/http"

	"gonest.dev/gonest"

	"blog-api/shared"
)

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/users")
	controller.Tags("users")
	// Guards apply to the WHOLE controller (no per-route scoping in
	// gonest today) -- every /users route requires the demo bearer token.
	controller.Guards(shared.AuthGuard)
	controller.Interceptors(shared.TimingInterceptor)

	service := gonest.MustInject[*Service](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Summary("List users")
		r.Response(http.StatusOK, func(response *gonest.Response) { response.Schema(Schema) })
		r.Handler(func(c *gonest.HttpContext) {
			res := c.Response()
			res.Json(service.List())
		})
	})

	controller.Route(gonest.HttpGet, "/:user_id", func(r *gonest.Route) {
		r.Summary("Get a user by id")
		r.Params(paramsDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.Response) { response.Schema(Schema) })
		r.Response(http.StatusNotFound)
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			p := gonest.MustParse[ParamsDTO](req.Params(), paramsDTOSchema)
			u := service.Get(p.UserID)
			if u == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			res.Json(u)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a user")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(createBodyDTOSchema)
		r.Response(http.StatusCreated, func(response *gonest.Response) { response.Schema(Schema) })
		r.Response(http.StatusConflict)
		r.Handler(func(c *gonest.HttpContext) {
			req, res := c.Request(), c.Response()
			body := gonest.MustParse[CreateBodyDTO](req.Body().Json(), createBodyDTOSchema)
			res.Status(http.StatusCreated).Json(service.Create(body.Name, body.Email))
		})
	})
})
