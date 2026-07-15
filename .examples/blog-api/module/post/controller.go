package post

import (
	"net/http"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/posts")
	controller.Tags("posts")
	controller.Guards(shared.AuthGuard)
	controller.Interceptors(shared.TimingInterceptor)

	service := gonest.MustInject[*Service](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Summary("List posts, optionally filtered by user_id")
		r.Query(listQueryDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.Response) { response.Schema(Schema) })
		r.Handler(func(ctx *gonest.Context) {
			q := gonest.MustQuery[*ListQueryDTO](ctx)
			ctx.Json(service.List(q.UserID))
		})
	})

	controller.Route(gonest.HttpGet, "/:post_id", func(r *gonest.Route) {
		r.Summary("Get a post by id")
		r.Params(paramsDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.Response) { response.Schema(Schema) })
		r.Response(http.StatusNotFound, func(response *gonest.Response) {
			response.Description("Cannot find a post using post_id")
		})
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*ParamsDTO](ctx)
			post := service.Get(p.PostID)
			if post == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(post)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a post")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(createBodyDTOSchema)
		r.Response(http.StatusCreated, func(response *gonest.Response) { response.Schema(Schema) })
		r.Response(http.StatusNotFound)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*CreateBodyDTO](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.UserID, body.Title, body.Body))
		})
	})
})
