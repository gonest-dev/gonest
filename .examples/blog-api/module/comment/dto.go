package comment

import (
	"github.com/gonest-dev/gonest"
)

type CreateBodyDTO struct {
	PostID int64  `json:"post_id"`
	UserID int64  `json:"user_id"`
	Body   string `json:"body"`
}

var createBodyDTOSchema = gonest.NewSchema(func(t *CreateBodyDTO, m *gonest.Schema) {
	m.Title("CommentCreateBodyDTO")
	m.Property(&t.PostID).Integer().Min(1).Required()
	m.Property(&t.UserID).Integer().Min(1).Required()
	m.Property(&t.Body).String().Min(1).Required()
})

type ListQueryDTO struct {
	PostID int64 `query:"post_id"`
	UserID int64 `query:"user_id"`
}

var listQueryDTOSchema = gonest.NewSchema(func(t *ListQueryDTO, m *gonest.Schema) {
	m.Title("CommentListQueryDTO")
	m.Property(&t.PostID).Integer()
	m.Property(&t.UserID).Integer()
})
