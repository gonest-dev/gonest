package comment

import (
	"database/sql"

	"gonest.dev/gonest"

	"blog-api/module/post"
	"blog-api/module/user"
)

type Service struct {
	db          *sql.DB
	userService *user.Service
	postService *post.Service
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
		panic(gonest.NewNotFoundException("", map[string]any{"post_id": postID}))
	}
	if s.userService.Get(userID) == nil {
		panic(gonest.NewNotFoundException("", map[string]any{"user_id": userID}))
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
