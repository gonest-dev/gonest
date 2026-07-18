package main

import "gonest.dev/gonest"

// CreatePostArgs is the "createPost" Mutation's Args -- json tags are read
// by GraphQL Args binding exactly like they already are for REST JSON
// bodies (unified-parse-api's Parseable, reused unchanged).
type CreatePostArgs struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

var createPostArgsSchema = gonest.NewSchema(func(t *CreatePostArgs, m *gonest.Schema) {
	m.Title("CreatePostArgs")
	m.Property(&t.Title).String().Min(1).Required()
	m.Property(&t.Body).String().Min(1).Required()
})

// PostIDArgs is the "post" Query's Args.
type PostIDArgs struct {
	ID int64 `json:"id"`
}

var postIDArgsSchema = gonest.NewSchema(func(t *PostIDArgs, m *gonest.Schema) {
	m.Title("PostIDArgs")
	m.Property(&t.ID).Integer().Min(1).Required()
})
