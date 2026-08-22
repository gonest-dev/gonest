package post

import (
	"gonest.dev/gonest"
)

type CreateBodyDTO struct {
	UserID int64  `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

var createBodyDTOSchema = gonest.NewSchema(func(t *CreateBodyDTO, s *gonest.Schema) {
	s.Title("PostCreateBodyDTO")
	s.Property(&t.UserID).Integer().Min(1).Required()
	s.Property(&t.Title).String().Min(1).Required()
	s.Property(&t.Body).String().Min(1).Required()
})

type ParamsDTO struct {
	PostID int64 `param:"post_id"`
}

var paramsDTOSchema = gonest.NewSchema(func(t *ParamsDTO, s *gonest.Schema) {
	s.Title("PostParamsDTO")
	s.Property(&t.PostID).Integer().Min(1).Required()
})

type ListQueryDTO struct {
	UserID int64 `query:"user_id"`
}

var listQueryDTOSchema = gonest.NewSchema(func(t *ListQueryDTO, s *gonest.Schema) {
	s.Title("post.ListQueryDTO")
	s.Property(&t.UserID).Integer()
})

// UploadAttachmentFormDTO is the multipart/form-data TEXT field validated
// alongside the file part -- the file itself has no "value" to check via
// Schema (only name/size/content-type), so it's handled separately by
// gonest.MustParse[T](ctx.Body().Form(onFile))'s own onFile callback (see controller.go).
type UploadAttachmentFormDTO struct {
	Description string `form:"description"`
}

var uploadAttachmentFormDTOSchema = gonest.NewSchema(func(t *UploadAttachmentFormDTO, s *gonest.Schema) {
	s.Title("PostUploadAttachmentFormDTO")
	s.Property(&t.Description).String()
})
