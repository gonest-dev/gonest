// Package user implements the User domain: entity, service (SQLite-backed),
// and controller.
package user

import (
	"database/sql"
	"strings"

	"blog-api/shared"
)

type Service struct {
	db *sql.DB
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
