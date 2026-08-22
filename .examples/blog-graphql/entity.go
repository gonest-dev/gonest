package main

import "gonest.dev/gonest"

// PostEntity is the domain type exposed over GraphQL -- Query.Returns and
// Mutation.Returns below reuse the SAME Schema OpenAPI/REST would use, no
// second declaration.
type PostEntity struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var postSchema = gonest.NewSchema(func(t *PostEntity, s *gonest.Schema) {
	s.Title("PostEntity")
	s.Property(&t.ID).Integer().Required()
	s.Property(&t.Title).String().Min(1).Required()
	s.Property(&t.Body).String().Min(1).Required()
})

// PostCreatedEvent is published (via gonest.Emitter.Emit) every time a Post
// is created -- the "postCreated" Subscription below is subscribed to this
// exact Go type via gonest.Subscribe[T], the same event-bus the Emitter &
// Listener feature already provides for static (NewListener) listeners.
type PostCreatedEvent struct {
	Post *PostEntity
}
