package comment

import (
	"net/http"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

type createBody struct {
	PostID int64  `json:"post_id"`
	UserID int64  `json:"user_id"`
	Body   string `json:"body"`
}

var createBodyMetadata = gonest.NewMetadata[createBody](func(t *createBody, m *gonest.Metadata) {
	m.Property(&t.PostID).Integer().Min(1).Required()
	m.Property(&t.UserID).Integer().Min(1).Required()
	m.Property(&t.Body).String().Min(1).Required()
})

type listQuery struct {
	PostID int64 `query:"post_id"`
	UserID int64 `query:"user_id"`
}

var listQueryMetadata = gonest.NewMetadata[listQuery](func(t *listQuery, m *gonest.Metadata) {
	m.Property(&t.PostID).Integer()
	m.Property(&t.UserID).Integer()
})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/comments")
	controller.Tags("comments")
	controller.Guards(shared.AuthGuard)
	controller.Interceptors(shared.TimingInterceptor)

	service := gonest.MustInject[*Service](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Summary("List comments, optionally filtered by post_id and/or user_id")
		r.Handler(func(ctx *gonest.Context) {
			q := gonest.MustQuery[*listQuery](ctx)
			ctx.Json(service.List(q.PostID, q.UserID))
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a comment")
		r.HttpCode(http.StatusCreated)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*createBody](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.PostID, body.UserID, body.Body))
		})
	})
})
