package post

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

// uploadsDir is where uploaded attachments land -- streamed straight to
// disk via io.Copy, one write at a time, never buffered whole in memory
// first (Multipart Form Streaming feature). A real deployment would swap
// this for an S3 (or similar) io.Writer instead; the gonest-facing code
// (MustParseRestFormBody's onFile callback below) would look identical.
const uploadsDir = "uploads"

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
		r.Handler(func(ctx *gonest.RestContext) {
			q := gonest.MustParseRestQuery[*ListQueryDTO](ctx, listQueryDTOSchema)
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
		r.Handler(func(ctx *gonest.RestContext) {
			p := gonest.MustParseRestParams[*ParamsDTO](ctx, paramsDTOSchema)
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
		r.Handler(func(ctx *gonest.RestContext) {
			body := gonest.MustParseRestJsonBody[*CreateBodyDTO](ctx, createBodyDTOSchema)
			ctx.Status(http.StatusCreated).Json(service.Create(body.UserID, body.Title, body.Body))
		})
	})

	// Real usage of MustParseRestFormBody (Multipart Form Streaming feature):
	// the uploaded file is streamed straight to disk via io.Copy as its bytes
	// arrive -- onFile fires before the rest of the multipart body is even
	// read, so gonest never buffers the whole file in memory first. Requires
	// AppOptions.EnableFormStreaming (see main.go).
	controller.Route(gonest.HttpPost, "/:post_id/attachment", func(r *gonest.Route) {
		r.Summary("Upload an attachment for a post (multipart/form-data, streamed to disk)")
		r.Params(paramsDTOSchema)
		r.Response(http.StatusCreated)
		r.Response(http.StatusNotFound, func(response *gonest.Response) {
			response.Description("Cannot find a post using post_id")
		})
		r.Handler(func(ctx *gonest.RestContext) {
			p := gonest.MustParseRestParams[*ParamsDTO](ctx, paramsDTOSchema)
			if service.Get(p.PostID) == nil {
				panic(gonest.NewNotFoundException(nil))
			}

			var savedFilename string
			var savedSize int64
			form := gonest.MustParseRestFormBody[*UploadAttachmentFormDTO](ctx, uploadAttachmentFormDTOSchema, func(f *gonest.FormFile) error {
				if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
					return err
				}
				savedFilename = fmt.Sprintf("%d_%s", p.PostID, f.Filename())
				dst, err := os.Create(filepath.Join(uploadsDir, savedFilename))
				if err != nil {
					return err
				}
				defer dst.Close()

				n, err := io.Copy(dst, f.Reader())
				savedSize = n
				return err
			})

			ctx.Status(http.StatusCreated).Json(map[string]any{
				"filename":    savedFilename,
				"size":        savedSize,
				"description": form.Description,
			})
		})
	})
})
