package main

import (
	"sync"

	"gonest.dev/gonest"
)

// TodoService holds todos in memory -- no database, no external
// dependency, deliberately the simplest possible persistence.
type TodoService struct {
	mu     sync.Mutex
	nextID int64
	items  []*TodoEntity
}

func (s *TodoService) List() []*TodoEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.items
}

func (s *TodoService) Get(id int64) *TodoEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.items {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (s *TodoService) Create(title string) *TodoEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	t := &TodoEntity{ID: s.nextID, Title: title}
	s.items = append(s.items, t)
	return t
}

func (s *TodoService) Update(id int64, title string, done bool) *TodoEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.items {
		if t.ID == id {
			t.Title, t.Done = title, done
			return t
		}
	}
	return nil
}

func (s *TodoService) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.items {
		if t.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return true
		}
	}
	return false
}

var TodoProvider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *TodoService {
		return &TodoService{items: make([]*TodoEntity, 0)}
	})
})
