package graphql_test

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
)

// newSSEDistinctTestSchema builds a minimal *gql.Schema exposing one Query
// field ("ping" -> "pong") -- same shape execute_test.go's own
// newExecTestSchema uses, duplicated here (unexported, test-file-local)
// rather than shared across files to keep each test file's fixtures
// self-contained, matching this package's existing convention (sse_test.go
// and ws_test.go each build their own fixtures too).
func newSSEDistinctTestSchema(t *testing.T) *graphql.Resolver {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})
	res.Declare()
	return res
}

func TestSSEDistinctHandler_ValidQuery_RespondsNextThenComplete(t *testing.T) {
	resolver := newSSEDistinctTestSchema(t)
	sch, err := graphql.Build(resolver.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	responder := newFakeSSEResponder("", map[string]string{"query": `{ ping }`})
	req, res := execution.New(responder)

	graphql.SSEDistinctHandler(sch, map[string]*graphql.Subscription{})(req, res)

	r := bufio.NewReader(responder.pr)

	eventLine := readLine(t, r, time.Second)
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}

	dataLine := readLine(t, r, time.Second)
	if !strings.HasPrefix(dataLine, "data: ") {
		t.Fatalf("second line = %q, want a 'data: ' frame", dataLine)
	}

	var payload struct {
		Data   map[string]any   `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &payload); err != nil {
		t.Fatalf("failed to decode SSE frame payload: %v", err)
	}
	if len(payload.Errors) != 0 {
		t.Fatalf("payload.Errors = %+v, want none", payload.Errors)
	}
	if payload.Data["ping"] != "pong" {
		t.Fatalf("payload.Data[ping] = %v, want %q", payload.Data["ping"], "pong")
	}

	// The blank line terminating the "next" frame.
	readLine(t, r, time.Second)

	completeLine := readLine(t, r, time.Second)
	if strings.TrimSpace(completeLine) != "event: complete" {
		t.Fatalf("complete line = %q, want %q", completeLine, "event: complete")
	}
}

func TestSSEDistinctHandler_InvalidQuery_RespondsNextWithErrorNot400(t *testing.T) {
	resolver := newSSEDistinctTestSchema(t)
	sch, err := graphql.Build(resolver.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	responder := newFakeSSEResponder("", map[string]string{"query": `{ doesNotExist`})
	req, res := execution.New(responder)

	graphql.SSEDistinctHandler(sch, map[string]*graphql.Subscription{})(req, res)

	if responder.GetStatus() != 200 {
		t.Fatalf("GetStatus() = %d, want 200 (error must arrive as a next event, never a bare HTTP status)", responder.GetStatus())
	}

	r := bufio.NewReader(responder.pr)

	eventLine := readLine(t, r, time.Second)
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}

	dataLine := readLine(t, r, time.Second)
	if !strings.HasPrefix(dataLine, "data: ") {
		t.Fatalf("second line = %q, want a 'data: ' frame", dataLine)
	}

	var payload struct {
		Data   any              `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &payload); err != nil {
		t.Fatalf("failed to decode SSE frame payload: %v", err)
	}
	if len(payload.Errors) == 0 {
		t.Fatal("payload.Errors = none, want at least one for a syntactically invalid query")
	}
}
