// Package post implements the Post domain: a Post belongs to exactly one
// User (its author).
package post

import (
	"database/sql"

	"github.com/gonest-dev/gonest"

	"blog-api/shared"
	"blog-api/user"
)

type Entity struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type Service struct {
	db          *sql.DB
	userService *user.Service
}

func NewService(db *sql.DB, userService *user.Service) *Service {
	return &Service{db: db, userService: userService}
}

func (s *Service) List(userID int64) []*Entity {
	query := `SELECT id, user_id, title, body FROM posts`
	args := []any{}
	if userID > 0 {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY id`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var out []*Entity
	for rows.Next() {
		e := &Entity{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.Title, &e.Body); err != nil {
			panic(err)
		}
		out = append(out, e)
	}
	return out
}

func (s *Service) Get(id int64) *Entity {
	e := &Entity{}
	err := s.db.QueryRow(`SELECT id, user_id, title, body FROM posts WHERE id = ?`, id).Scan(&e.ID, &e.UserID, &e.Title, &e.Body)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		panic(err)
	}
	return e
}

func (s *Service) Create(userID int64, title, body string) *Entity {
	if s.userService.Get(userID) == nil {
		panic(gonest.NewNotFoundException(map[string]any{"user_id": userID}))
	}

	res, err := s.db.Exec(`INSERT INTO posts (user_id, title, body) VALUES (?, ?, ?)`, userID, title, body)
	if err != nil {
		panic(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		panic(err)
	}
	return &Entity{ID: id, UserID: userID, Title: title, Body: body}
}

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := gonest.MustInject[*sql.DB](provider)
	userService := gonest.MustInject[*user.Service](provider)
	provider.Constructor(func() *Service { return NewService(db, userService) })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DBModule, user.Module)
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
