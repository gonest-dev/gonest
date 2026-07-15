package post

import (
	"github.com/gonest-dev/gonest"
)

type CreateBodyDTO struct {
	UserID int64  `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

var createBodyDTOSchema = gonest.NewSchema(func(t *CreateBodyDTO, m *gonest.Schema) {
	m.Title("post.CreateBodyDTO")
	m.Property(&t.UserID).Integer().Min(1).Required()
	m.Property(&t.Title).String().Min(1).Required()
	m.Property(&t.Body).String().Min(1).Required()
})

type ParamsDTO struct {
	PostID int64 `param:"post_id"`
}

var paramsDTOSchema = gonest.NewSchema(func(t *ParamsDTO, m *gonest.Schema) {
	m.Title("post.ParamsDTO")
	m.Property(&t.PostID).Integer().Min(1).Required()
})

type ListQueryDTO struct {
	UserID int64 `query:"user_id"`
}

var listQueryDTOSchema = gonest.NewSchema(func(t *ListQueryDTO, m *gonest.Schema) {
	m.Title("post.ListQueryDTO")
	m.Property(&t.UserID).Integer()
})
