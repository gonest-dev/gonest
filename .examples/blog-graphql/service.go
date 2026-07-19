package main

import (
	"sync"

	"gonest.dev/gonest"
)

// Service is an in-memory Post store -- kept deliberately simple (no
// database) since this example's whole point is demonstrating the GraphQL
// surface (Query/Mutation/Subscription), not persistence.
type Service struct {
	emitter *gonest.Emitter

	mu     sync.Mutex
	nextID int64
	posts  []*PostEntity
}

func (s *Service) List() []*PostEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*PostEntity(nil), s.posts...)
}

func (s *Service) Get(id int64) *PostEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.posts {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Create stores a new Post and publishes a PostCreatedEvent -- the
// Subscription resolver (resolver.go) never learns about this call
// directly, it only subscribes to the EVENT TYPE via gonest.Subscribe[T],
// same "Uma Única Fonte de Verdade" event bus NewListener/Emit already provide.
func (s *Service) Create(title, body string) *PostEntity {
	s.mu.Lock()
	s.nextID++
	p := &PostEntity{ID: s.nextID, Title: title, Body: body}
	s.posts = append(s.posts, p)
	s.mu.Unlock()

	s.emitter.Emit(PostCreatedEvent{Post: p})
	return p
}
