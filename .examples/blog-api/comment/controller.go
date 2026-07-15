package comment

import (
	"net/http"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/comments")
	controller.Tags("comments")
	controller.Guards(shared.AuthGuard)
	controller.Interceptors(shared.TimingInterceptor)

	service := gonest.MustInject[*Service](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Summary("List comments, optionally filtered by post_id and/or user_id")
		r.QueryParams(listQueryDTOSchema)
		r.Response(http.StatusOK, Schema)
		r.Handler(func(ctx *gonest.Context) {
			q := gonest.MustQuery[*ListQueryDTO](ctx)
			ctx.Json(service.List(q.PostID, q.UserID))
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a comment")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(createBodyDTOSchema)
		r.Response(http.StatusCreated, Schema)
		r.Response(http.StatusNotFound)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*CreateBodyDTO](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.PostID, body.UserID, body.Body))
		})
	})
})
