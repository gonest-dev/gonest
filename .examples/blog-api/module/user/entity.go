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
var Schema = gonest.NewSchema(func(t *Entity, m *gonest.Schema) {
	m.Title("UserEntity")
	m.Property(&t.ID).Integer().Required()
	m.Property(&t.Name).String().Required()
	m.Property(&t.Email).Email().Required()
})
