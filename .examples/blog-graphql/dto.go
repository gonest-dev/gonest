package main

import "gonest.dev/gonest"

// CreatePostArgs is the "createPost" Mutation's Args -- json tags are read
// by GraphQL Args binding exactly like they already are for REST JSON
// bodies (unified-parse-api's Parseable, reused unchanged).
type CreatePostArgs struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

var createPostArgsSchema = gonest.NewSchema(func(t *CreatePostArgs, s *gonest.Schema) {
	s.Title("CreatePostArgs")
	s.Property(&t.Title).String().Min(1).Required()
	s.Property(&t.Body).String().Min(1).Required()
})

// PostIDArgs is the "post" Query's Args.
type PostIDArgs struct {
	ID int64 `json:"id"`
}

var postIDArgsSchema = gonest.NewSchema(func(t *PostIDArgs, s *gonest.Schema) {
	s.Title("PostIDArgs")
	s.Property(&t.ID).Integer().Min(1).Required()
})
