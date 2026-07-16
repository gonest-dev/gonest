// Package comment implements the Comment domain: a Comment belongs to
// exactly one Post AND exactly one User (its author) -- simulating the
// M:N shape "a User can comment on many Posts, a Post can have comments
// from many Users" via two foreign keys, no explicit join table needed.
package comment

import (
	"gonest.dev/gonest"
)

type Entity struct {
	ID     int64  `json:"id"`
	PostID int64  `json:"post_id"`
	UserID int64  `json:"user_id"`
	Body   string `json:"body"`
}

// Schema registers Entity's own OpenAPI schema (components.schemas.
// comment.Entity) -- referenced by Controller's Route.Response calls so
// generated docs actually describe the response body shape, not just the
// bare path/summary.
var Schema = gonest.NewSchema(func(t *Entity, m *gonest.Schema) {
	m.Title("CommentEntity")
	m.Property(&t.ID).Integer().Required()
	m.Property(&t.PostID).Integer().Required()
	m.Property(&t.UserID).Integer().Required()
	m.Property(&t.Body).String().Required()
})
