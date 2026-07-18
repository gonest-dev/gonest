package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/adapter/fiber"
	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/schema"
)

type graphqlUserEntity struct {
	Id    int64  `json:"id"`
	Email string `json:"email"`
}

func TestNewApp_GraphqlQuery_RealHTTPDispatch_HappyPath(t *testing.T) {
	zero := &graphqlUserEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	userSchema := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	userSchema.Property(&zero.Id).Integer().Required()
	userSchema.Property(&zero.Email).Email().Required()

	userResolver := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Query("user", func(q *gqlresolver.Query) {
			q.Returns(userSchema)
			q.Handler(func(ctx *gqlresolver.GraphqlContext) any {
				return map[string]any{"id": int64(1), "email": "john@example.com"}
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(userResolver)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{
		"query": `{ user { id email } }`,
	})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var out struct {
		Data struct {
			User struct {
				Id    int64  `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", out.Errors)
	}
	if out.Data.User.Id != 1 || out.Data.User.Email != "john@example.com" {
		t.Fatalf("unexpected data: %+v", out.Data)
	}
}

func TestNewApp_GraphqlMutation_InvalidArgs_ProducesGraphqlError(t *testing.T) {
	type createUserArgs struct {
		Email string `json:"email"`
	}
	argsZero := &createUserArgs{}
	argsTyp := reflect.TypeOf(*argsZero)
	t.Cleanup(func() { schema.Deregister(argsTyp) })
	argsSchema := schema.New(argsTyp, uintptr(unsafe.Pointer(argsZero)))
	argsSchema.Property(&argsZero.Email).Email().Required()

	userResolver := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Mutation("createUser", func(m *gqlresolver.Mutation) {
			m.Args(argsSchema)
			m.Handler(func(ctx *gqlresolver.GraphqlContext) any {
				var args createUserArgs
				if err := ctx.Args().ParseInto(&args, argsSchema); err != nil {
					panic(err)
				}
				return args.Email
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(userResolver)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{
		"query":     `mutation($email: String!) { createUser(email: $email) }`,
		"variables": map[string]any{"email": "not-an-email"},
	})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Errors []any `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Errors) == 0 {
		t.Fatal("expected a GraphQL error for an invalid email arg, got none")
	}
}
