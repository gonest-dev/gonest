package user

import (
	"github.com/gonest-dev/gonest"
)

type CreateBodyDTO struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Title MUST be distinct across every Schema in the whole app --
// components.schemas is a single flat namespace, and Go's own
// unqualified type name ("createBody") collides across user/post/
// comment's own local types of the same name otherwise (found via
// this exact example: only ONE "createBody" schema ever showed up in
// /openapi.json, silently wrong for 2 of the 3 controllers).

var createBodyDTOSchema = gonest.NewSchema(func(t *CreateBodyDTO, m *gonest.Schema) {
	m.Title("user.CreateBodyDTO")
	m.Property(&t.Name).String().Min(1).Required()
	m.Property(&t.Email).Email().Required()
})

type ParamsDTO struct {
	UserID int64 `param:"user_id"`
}

var paramsDTOSchema = gonest.NewSchema(func(t *ParamsDTO, m *gonest.Schema) {
	m.Title("user.ParamsDTO")
	m.Property(&t.UserID).Integer().Min(1).Required()
})
