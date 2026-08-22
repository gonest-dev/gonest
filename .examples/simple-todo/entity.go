package main

// TodoEntity is the persisted shape returned to clients.
type TodoEntity struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
