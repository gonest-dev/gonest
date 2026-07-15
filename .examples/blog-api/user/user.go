// Package user implements the User domain: entity, service (SQLite-backed),
// and controller.
package user

import (
	"database/sql"
	"strings"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
)

type Entity struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// EntityMetadata registers Entity's own OpenAPI schema (components.schemas.
// UserEntity) -- referenced by Controller's Route.Response calls so
// generated docs actually describe the response body shape, not just the
// bare path/summary.
var EntityMetadata = gonest.NewMetadata[Entity](func(t *Entity, m *gonest.Metadata) {
	m.Title("UserEntity")
	m.Property(&t.ID).Integer().Required()
	m.Property(&t.Name).String().Required()
	m.Property(&t.Email).Email().Required()
})

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List() []*Entity {
	rows, err := s.db.Query(`SELECT id, name, email FROM users ORDER BY id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var out []*Entity
	for rows.Next() {
		e := &Entity{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Email); err != nil {
			panic(err)
		}
		out = append(out, e)
	}
	return out
}

func (s *Service) Get(id int64) *Entity {
	e := &Entity{}
	err := s.db.QueryRow(`SELECT id, name, email FROM users WHERE id = ?`, id).Scan(&e.ID, &e.Name, &e.Email)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		panic(err)
	}
	return e
}

func (s *Service) Create(name, email string) *Entity {
	res, err := s.db.Exec(`INSERT INTO users (name, email) VALUES (?, ?)`, name, email)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			panic(shared.NewDuplicateEmailException(email))
		}
		panic(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		panic(err)
	}
	return &Entity{ID: id, Name: name, Email: email}
}

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := gonest.MustInject[*sql.DB](provider)
	provider.Constructor(func() *Service { return NewService(db) })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DBModule)
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
