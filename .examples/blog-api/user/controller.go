package user

import (
	"net/http"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

type createBody struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var createBodyMetadata = gonest.NewMetadata[createBody](func(t *createBody, m *gonest.Metadata) {
	m.Property(&t.Name).String().Min(1).Required()
	m.Property(&t.Email).Email().Required()
})

type idParams struct {
	ID int64 `param:"id"`
}

var idParamsMetadata = gonest.NewMetadata[idParams](func(t *idParams, m *gonest.Metadata) {
	m.Property(&t.ID).Integer().Min(1).Required()
})

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
		r.Handler(func(ctx *gonest.Context) {
			ctx.Json(service.List())
		})
	})

	controller.Route(gonest.HttpGet, "/:id", func(r *gonest.Route) {
		r.Summary("Get a user by id")
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*idParams](ctx)
			u := service.Get(p.ID)
			if u == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(u)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a user")
		r.HttpCode(http.StatusCreated)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*createBody](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.Name, body.Email))
		})
	})
})
