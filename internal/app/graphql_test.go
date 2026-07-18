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
	"gonest.dev/gonest/internal/graphql"
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

	userResolver := graphql.New(func(r *graphql.Resolver) {
		r.Query("user", func(q *graphql.Query) {
			q.Returns(userSchema)
			q.Handler(func(ctx *graphql.GraphqlContext) any {
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

	userResolver := graphql.New(func(r *graphql.Resolver) {
		r.Mutation("createUser", func(m *graphql.Mutation) {
			m.Args(argsSchema)
			m.Handler(func(ctx *graphql.GraphqlContext) any {
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

func TestNewApp_GraphqlPath_OverriddenViaAppOptions(t *testing.T) {
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(res)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{GraphqlPath: "/api/gql"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{"query": `{ ping }`})

	// The default path must NOT be registered when overridden.
	defaultReq := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	defaultReq.Header.Set("Content-Type", "application/json")
	defaultResp, err := fiberAdapter.FiberApp().Test(defaultReq)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer defaultResp.Body.Close()
	if defaultResp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /graphql (default path, should be unregistered) status = %d, want 404", defaultResp.StatusCode)
	}

	// The overridden path must work.
	customReq := httptest.NewRequest(http.MethodPost, "/api/gql", bytes.NewReader(body))
	customReq.Header.Set("Content-Type", "application/json")
	customResp, err := fiberAdapter.FiberApp().Test(customReq)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer customResp.Body.Close()
	if customResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/gql status = %d, want 200", customResp.StatusCode)
	}

	var out struct {
		Data struct {
			Ping string `json:"ping"`
		} `json:"data"`
	}
	if err := json.NewDecoder(customResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Data.Ping != "pong" {
		t.Fatalf("data.ping = %q, want %q", out.Data.Ping, "pong")
	}
}
