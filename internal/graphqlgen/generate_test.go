package graphqlgen_test

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/graphqlgen"
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

	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Query("user", func(qb *gqlresolver.Query) {
			qb.Returns(userSchema)
			qb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return nil })
		})
	})
	res.Declare()
	queries := res.OwnQueries()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	sch, err := graphqlgen.Build(queries, nil, nil)
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

	result := graphql.Do(graphql.Params{
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

	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Query("me", func(qb *gqlresolver.Query) {
			qb.Returns(m)
			qb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	sch, err := graphqlgen.Build(res.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	typeMap := sch.TypeMap()
	scalarType, ok := typeMap["Email"]
	if !ok {
		t.Fatalf("expected a custom scalar named 'Email' in the schema's type map, got types: %v", typeMap)
	}
	if _, ok := scalarType.(*graphql.Scalar); !ok {
		t.Fatalf("'Email' type = %T, want *graphql.Scalar", scalarType)
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

	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Mutation("createUser", func(mb *gqlresolver.Mutation) {
			mb.Args(argsSchema)
			mb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	sch, err := graphqlgen.Build(nil, res.OwnMutations(), nil)
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
	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Query("dup", func(qb *gqlresolver.Query) {
			qb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return 1 })
		})
		r.Query("dup", func(qb *gqlresolver.Query) {
			qb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return 2 })
		})
	})
	res.Declare()

	_, err := graphqlgen.Build(res.OwnQueries(), nil, nil)
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

	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Query("bad", func(qb *gqlresolver.Query) {
			qb.Returns(m)
			qb.Handler(func(ctx *gqlresolver.GraphqlContext) any { return nil })
		})
	})
	res.Declare()

	_, err := graphqlgen.Build(res.OwnQueries(), nil, nil)
	if err == nil {
		t.Fatal("expected an error for Custom(fn) without GraphqlScalar(name), got nil")
	}
}
