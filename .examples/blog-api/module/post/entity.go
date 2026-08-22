// Package post implements the Post domain: a Post belongs to exactly one
// User (its author).
package post

import (
	"gonest.dev/gonest"
)

type Entity struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// Schema registers Entity's own OpenAPI schema (components.schemas.
// post.Entity) -- referenced by Controller's Route.Response calls so
// generated docs actually describe the response body shape, not just the
// bare path/summary.
var Schema = gonest.NewSchema(func(t *Entity, s *gonest.Schema) {
	s.Title("PostEntity")
	s.Property(&t.ID).Integer().Required()
	s.Property(&t.UserID).Integer().Required()
	s.Property(&t.Title).String().Required()
	s.Property(&t.Body).String().Required()
})
