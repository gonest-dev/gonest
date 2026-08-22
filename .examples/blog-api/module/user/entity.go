// Package user implements the User domain: entity, service (SQLite-backed),
// and controller.
package user

import (
	"gonest.dev/gonest"
)

type Entity struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Schema registers Entity's own OpenAPI schema (components.schemas.
// user.Entity) -- referenced by Controller's Route.Response calls so
// generated docs actually describe the response body shape, not just the
// bare path/summary.
var Schema = gonest.NewSchema(func(t *Entity, s *gonest.Schema) {
	s.Title("UserEntity")
	s.Property(&t.ID).Integer().Required()
	s.Property(&t.Name).String().Required()
	s.Property(&t.Email).Email().Required()
})
