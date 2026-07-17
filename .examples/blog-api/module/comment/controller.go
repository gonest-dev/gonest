package comment

import (
	"net/http"

	"gonest.dev/gonest"

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
		r.Query(listQueryDTOSchema)
		r.Response(http.StatusOK, func(response *gonest.RouteResponse) { response.Schema(Schema) })
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			q := gonest.MustParse[ListQueryDTO](req.Query(), listQueryDTOSchema)
			res.Json(service.List(q.PostID, q.UserID))
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a comment")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(createBodyDTOSchema)
		r.Response(http.StatusCreated, func(response *gonest.RouteResponse) { response.Schema(Schema) })
		r.Response(http.StatusNotFound)
		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			body := gonest.MustParse[CreateBodyDTO](req.Body().Json(), createBodyDTOSchema)
			res.Status(http.StatusCreated).Json(service.Create(body.PostID, body.UserID, body.Body))
		})
	})
})
