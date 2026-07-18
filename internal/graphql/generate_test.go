package graphql_test

import (
	"reflect"
	"testing"
	"unsafe"

	gql "github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/schema"
)

type genUserEntity struct {
	Id    int64  `json:"id"`
	Email string `json:"email"`
}

func newGenTestSchema(t *testing.T) (*genUserEntity, *schema.Schema) {
	t.Helper()
	zero := &genUserEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

func TestBuild_SimpleQuery_ProducesValidSchema(t *testing.T) {
	zero, userSchema := newGenTestSchema(t)
	userSchema.Property(&zero.Id).Integer().Required()
	userSchema.Property(&zero.Email).Email().Required()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("user", func(qb *graphql.Query) {
			qb.Returns(userSchema)
			qb.Handler(func(ctx *graphql.GraphqlContext) any { return nil })
		})
	})
	res.Declare()
	queries := res.OwnQueries()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	sch, err := graphql.Build(queries, nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	queryType := sch.QueryType()
	if queryType == nil {
		t.Fatal("Build() produced a schema with no Query root type")
	}
	if _, ok := queryType.Fields()["user"]; !ok {
		t.Fatalf("Query root type has no 'user' field, fields: %v", queryType.Fields())
	}

	result := gql.Do(gql.Params{
		Schema:        *sch,
		RequestString: `{ __schema { queryType { name } } }`,
	})
	if result.HasErrors() {
		t.Fatalf("introspection query failed: %v", result.Errors)
	}
}

type genEmailOnlyEntity struct {
	Email string `json:"email"`
}

func TestBuild_EmailFormat_ProducesEmailCustomScalar(t *testing.T) {
	zero := &genEmailOnlyEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	m.Property(&zero.Email).Email().Required()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("me", func(qb *graphql.Query) {
			qb.Returns(m)
			qb.Handler(func(ctx *graphql.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	sch, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	typeMap := sch.TypeMap()
	scalarType, ok := typeMap["Email"]
	if !ok {
		t.Fatalf("expected a custom scalar named 'Email' in the schema's type map, got types: %v", typeMap)
	}
	if _, ok := scalarType.(*gql.Scalar); !ok {
		t.Fatalf("'Email' type = %T, want *gql.Scalar", scalarType)
	}
}

func TestBuild_MutationWithArgs_ArgsBecomeFieldConfigArgument(t *testing.T) {
	type createUserArgs struct {
		Name string `json:"name"`
	}
	argsZero := &createUserArgs{}
	argsTyp := reflect.TypeOf(*argsZero)
	t.Cleanup(func() { schema.Deregister(argsTyp) })
	argsSchema := schema.New(argsTyp, uintptr(unsafe.Pointer(argsZero)))
	argsSchema.Property(&argsZero.Name).String().Required()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Mutation("createUser", func(mb *graphql.Mutation) {
			mb.Args(argsSchema)
			mb.Handler(func(ctx *graphql.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	sch, err := graphql.Build(nil, res.OwnMutations(), nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	mutationType := sch.MutationType()
	if mutationType == nil {
		t.Fatal("Build() produced a schema with no Mutation root type")
	}
	field, ok := mutationType.Fields()["createUser"]
	if !ok {
		t.Fatal("Mutation root type has no 'createUser' field")
	}
	found := false
	for _, arg := range field.Args {
		if arg.Name() == "name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("'createUser' field has no 'name' argument, args: %+v", field.Args)
	}
}

func TestBuild_DuplicateQueryName_ReturnsError(t *testing.T) {
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("dup", func(qb *graphql.Query) {
			qb.Handler(func(ctx *graphql.GraphqlContext) any { return 1 })
		})
		r.Query("dup", func(qb *graphql.Query) {
			qb.Handler(func(ctx *graphql.GraphqlContext) any { return 2 })
		})
	})
	res.Declare()

	_, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err == nil {
		t.Fatal("expected an error for duplicate Query name 'dup', got nil")
	}
}

func TestBuild_CustomWithoutGraphqlScalar_ReturnsError(t *testing.T) {
	type badEntity struct {
		Weird string
	}
	zero := &badEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	m.Property(&zero.Weird).Custom(func(raw any) (any, error) { return raw, nil })

	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("bad", func(qb *graphql.Query) {
			qb.Returns(m)
			qb.Handler(func(ctx *graphql.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	_, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err == nil {
		t.Fatal("expected an error for Custom(fn) without GraphqlScalar(name), got nil")
	}
}

// --- regressions found via a live .examples/blog-graphql dispatch, not
// caught by any earlier unit test (all of which returned map[string]any
// from Handler, never a real Go struct) ---

type genPostEntity struct {
	Id    int64  `json:"id"`
	Title string `json:"title"`
}

func TestBuild_QueryReturningStruct_ResolvesFieldsCorrectly(t *testing.T) {
	zero := &genPostEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	postSchema := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	postSchema.Property(&zero.Id).Integer().Required()
	postSchema.Property(&zero.Title).String().Required()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("post", func(qb *graphql.Query) {
			qb.Returns(postSchema)
			qb.Handler(func(ctx *graphql.GraphqlContext) any {
				// A REAL struct value, NOT a map[string]any -- graphql-go's
				// own DEFAULT field resolver (used when Field.Resolve is
				// nil) only knows how to read a map by key, so this used to
				// come back all-null before fieldResolver was added.
				return &genPostEntity{Id: 1, Title: "Hello"}
			})
		})
	})
	res.Declare()

	sch, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := gql.Do(gql.Params{
		Schema:        *sch,
		RequestString: `{ post { id title } }`,
	})
	if result.HasErrors() {
		t.Fatalf("query failed: %v", result.Errors)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result.Data = %T, want map[string]any", result.Data)
	}
	post, ok := data["post"].(map[string]any)
	if !ok {
		t.Fatalf("data[\"post\"] = %T, want map[string]any", data["post"])
	}
	if post["title"] != "Hello" {
		t.Fatalf("post[\"title\"] = %v, want %q (struct field resolution)", post["title"], "Hello")
	}
}

func TestBuild_QueryReturnsList_ProducesArrayOfResults(t *testing.T) {
	zero := &genPostEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	postSchema := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	postSchema.Property(&zero.Id).Integer().Required()
	postSchema.Property(&zero.Title).String().Required()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("posts", func(qb *graphql.Query) {
			qb.ReturnsList(postSchema)
			qb.Handler(func(ctx *graphql.GraphqlContext) any {
				return []*genPostEntity{{Id: 1, Title: "A"}, {Id: 2, Title: "B"}}
			})
		})
	})
	res.Declare()

	sch, err := graphql.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := gql.Do(gql.Params{
		Schema:        *sch,
		RequestString: `{ posts { id title } }`,
	})
	if result.HasErrors() {
		t.Fatalf("query failed: %v", result.Errors)
	}
	data := result.Data.(map[string]any)
	posts, ok := data["posts"].([]any)
	if !ok {
		t.Fatalf("data[\"posts\"] = %T, want []any (ReturnsList must produce a GraphQL LIST type)", data["posts"])
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}
}
