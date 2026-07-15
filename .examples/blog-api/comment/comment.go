// Package comment implements the Comment domain: a Comment belongs to
// exactly one Post AND exactly one User (its author) -- simulating the
// M:N shape "a User can comment on many Posts, a Post can have comments
// from many Users" via two foreign keys, no explicit join table needed.
package comment

import (
	"database/sql"

	"github.com/gonest-dev/gonest"

	"blog-api/post"
	"blog-api/shared"
	"blog-api/user"
)

type Entity struct {
	ID     int64  `json:"id"`
	PostID int64  `json:"post_id"`
	UserID int64  `json:"user_id"`
	Body   string `json:"body"`
}

type Service struct {
	db          *sql.DB
	userService *user.Service
	postService *post.Service
}

func NewService(db *sql.DB, userService *user.Service, postService *post.Service) *Service {
	return &Service{db: db, userService: userService, postService: postService}
}

func (s *Service) List(postID, userID int64) []*Entity {
	query := `SELECT id, post_id, user_id, body FROM comments WHERE 1=1`
	var args []any
	if postID > 0 {
		query += ` AND post_id = ?`
		args = append(args, postID)
	}
	if userID > 0 {
		query += ` AND user_id = ?`
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
		if err := rows.Scan(&e.ID, &e.PostID, &e.UserID, &e.Body); err != nil {
			panic(err)
		}
		out = append(out, e)
	}
	return out
}

func (s *Service) Create(postID, userID int64, body string) *Entity {
	if s.postService.Get(postID) == nil {
		panic(gonest.NewNotFoundException(map[string]any{"post_id": postID}))
	}
	if s.userService.Get(userID) == nil {
		panic(gonest.NewNotFoundException(map[string]any{"user_id": userID}))
	}

	res, err := s.db.Exec(`INSERT INTO comments (post_id, user_id, body) VALUES (?, ?, ?)`, postID, userID, body)
	if err != nil {
		panic(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		panic(err)
	}
	return &Entity{ID: id, PostID: postID, UserID: userID, Body: body}
}

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := gonest.MustInject[*sql.DB](provider)
	userService := gonest.MustInject[*user.Service](provider)
	postService := gonest.MustInject[*post.Service](provider)
	provider.Constructor(func() *Service { return NewService(db, userService, postService) })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DBModule, user.Module, post.Module)
	module.Providers(Provider)
	module.Controllers(Controller)
})
