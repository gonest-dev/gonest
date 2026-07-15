package post

import (
	"net/http"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

type createBody struct {
	UserID int64  `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

var createBodySchema = gonest.NewSchema[createBody](func(t *createBody, m *gonest.Schema) {
	// Title distinguishes this from user/comment's own "createBody" local
	// type -- see user.createBodySchema's own comment for why this is
	// required, not optional, once 2+ controllers each have a same-named
	// unexported request-body type.
	m.Title("CreatePostRequest")
	m.Property(&t.UserID).Integer().Min(1).Required()
	m.Property(&t.Title).String().Min(1).Required()
	m.Property(&t.Body).String().Min(1).Required()
})

type idParams struct {
	ID int64 `param:"id"`
}

var idParamsSchema = gonest.NewSchema[idParams](func(t *idParams, m *gonest.Schema) {
	m.Property(&t.ID).Integer().Min(1).Required()
})

type listQuery struct {
	UserID int64 `query:"user_id"`
}

var listQuerySchema = gonest.NewSchema[listQuery](func(t *listQuery, m *gonest.Schema) {
	m.Property(&t.UserID).Integer()
})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/posts")
	controller.Tags("posts")
	controller.Guards(shared.AuthGuard)
	controller.Interceptors(shared.TimingInterceptor)

	service := gonest.MustInject[*Service](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Summary("List posts, optionally filtered by user_id")
		r.QueryParams(listQuerySchema)
		r.Response(http.StatusOK, EntitySchema)
		r.Handler(func(ctx *gonest.Context) {
			q := gonest.MustQuery[*listQuery](ctx)
			ctx.Json(service.List(q.UserID))
		})
	})

	controller.Route(gonest.HttpGet, "/:id", func(r *gonest.Route) {
		r.Summary("Get a post by id")
		r.PathParams(idParamsSchema)
		r.Response(http.StatusOK, EntitySchema)
		r.Response(http.StatusNotFound)
		r.Handler(func(ctx *gonest.Context) {
			p := gonest.MustParams[*idParams](ctx)
			post := service.Get(p.ID)
			if post == nil {
				panic(gonest.NewNotFoundException(nil))
			}
			ctx.Json(post)
		})
	})

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Create a post")
		r.HttpCode(http.StatusCreated)
		r.RequestBody(createBodySchema)
		r.Response(http.StatusCreated, EntitySchema)
		r.Response(http.StatusNotFound)
		r.Handler(func(ctx *gonest.Context) {
			body := gonest.MustJsonBody[*createBody](ctx)
			ctx.Status(http.StatusCreated).Json(service.Create(body.UserID, body.Title, body.Body))
		})
	})
})
