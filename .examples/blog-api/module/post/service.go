package post

import (
	"database/sql"

	"gonest.dev/gonest"

	"blog-api/module/user"
)

type Service struct {
	db          *sql.DB
	userService *user.Service
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
		panic(gonest.NewNotFoundException("", map[string]any{"user_id": userID}))
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
