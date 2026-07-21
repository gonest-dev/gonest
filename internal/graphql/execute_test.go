package graphql_test

import (
	"testing"

	"gonest.dev/gonest/internal/graphql"
)

// newExecTestSchema builds a minimal *gql.Schema exposing one Query field
// ("ping" -> "pong") -- just enough surface for Execute's own tests, no
// entity/schema.Schema registration needed since the query returns a plain
// string.
func newExecTestSchema(t *testing.T) *graphql.Resolver {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.Context) any { return "pong" })
		})
	})
	res.Declare()
	return res
}

func TestExecute_ValidQuery_ReturnsData(t *testing.T) {
	res := newExecTestSchema(t)
	sch, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	data, errs := graphql.Execute(sch, `{ ping }`, nil, "")

	if len(errs) != 0 {
		t.Fatalf("Execute() errors = %+v, want none", errs)
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("Execute() data = %#v, want map[string]any", data)
	}
	if m["ping"] != "pong" {
		t.Fatalf("Execute() data[ping] = %v, want %q", m["ping"], "pong")
	}
}

func TestExecute_InvalidQuery_ReturnsErrors(t *testing.T) {
	res := newExecTestSchema(t)
	sch, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	data, errs := graphql.Execute(sch, `{ doesNotExist }`, nil, "")

	if len(errs) == 0 {
		t.Fatal("Execute() errors = none, want at least one for an unknown field")
	}
	if _, ok := errs[0]["message"]; !ok {
		t.Fatalf("Execute() errs[0] = %+v, want a \"message\" key", errs[0])
	}
	if data != nil {
		t.Fatalf("Execute() data = %#v, want nil for a query that fails validation", data)
	}
}
