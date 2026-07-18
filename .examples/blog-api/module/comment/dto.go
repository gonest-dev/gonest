package comment

import (
	"gonest.dev/gonest"
)

type CreateBodyDTO struct {
	PostID int64  `json:"post_id"`
	UserID int64  `json:"user_id"`
	Body   string `json:"body"`
}

var createBodyDTOSchema = gonest.NewSchema(func(t *CreateBodyDTO, s *gonest.Schema) {
	s.Title("CommentCreateBodyDTO")
	s.Property(&t.PostID).Integer().Min(1).Required()
	s.Property(&t.UserID).Integer().Min(1).Required()
	s.Property(&t.Body).String().Min(1).Required()
})

type ListQueryDTO struct {
	PostID int64 `query:"post_id"`
	UserID int64 `query:"user_id"`
}

var listQueryDTOSchema = gonest.NewSchema(func(t *ListQueryDTO, s *gonest.Schema) {
	s.Title("CommentListQueryDTO")
	s.Property(&t.PostID).Integer()
	s.Property(&t.UserID).Integer()
})
