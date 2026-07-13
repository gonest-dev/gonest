package fiberapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
)

// TestNew_RegisterRoute_NoError proves RegisterRoute accepts a simple GET
// route against a fresh FiberApp without returning an error -- the minimal
// "registration succeeds" case before any dispatch is exercised.
func TestNew_RegisterRoute_NoError(t *testing.T) {
	app := New()

	err := app.RegisterRoute(route.HttpGet, "/ping", func(ctx *httpctx.Context) {
		ctx.Status(200).Json(map[string]string{"ok": "true"})
	})

	if err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}
}

// TestRegisterRoute_RealDispatch_RunsGonestHandler proves a real HTTP
// request dispatched via Fiber's own app.Test (no port required, see
// TESTING.md's "integration" tier) reaches the registered route and runs
// the gonest Handler -- not just that registration didn't error.
func TestRegisterRoute_RealDispatch_RunsGonestHandler(t *testing.T) {
	app := New()

	called := false
	if err := app.RegisterRoute(route.HttpGet, "/ping", func(ctx *httpctx.Context) {
		called = true
		ctx.Json(map[string]string{"pong": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatalf("expected gonest Handler to run, but it did not")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestRegisterRoute_HandlerPanics_Responds500 proves a Handler that panics
// with something that is not (yet, Milestone 2 doesn't exist) an Exception
// is recovered by the adapter's OWN recover() -- not Fiber's native
// recover middleware, not Fiber's error-return contract -- and turned into
// a generic 500 response, per design.md's error handling table. Critically:
// the test process itself must not die.
func TestRegisterRoute_HandlerPanics_Responds500(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *httpctx.Context) {
		panic("something went wrong")
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
}

// TestRegisterRoute_JsonLandsInRealResponseBody proves ctx.Json(value)
// serializes correctly into the real HTTP response body as observed by
// app.Test -- the Responder implementation actually bridges to fiber.Ctx's
// JSON, not just to a fake in a unit test.
func TestRegisterRoute_JsonLandsInRealResponseBody(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpGet, "/greeting", func(ctx *httpctx.Context) {
		ctx.Json(map[string]string{"message": "hello"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/greeting", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["message"] != "hello" {
		t.Fatalf("expected body message %q, got %q", "hello", body["message"])
	}
}

// TestRegisterRoute_StatusLandsInRealResponse proves ctx.Status(code)
// lands as the real HTTP response status code as observed by app.Test.
func TestRegisterRoute_StatusLandsInRealResponse(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpPost, "/created", func(ctx *httpctx.Context) {
		ctx.Status(201).Json(map[string]string{"id": "1"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/created", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
}

// TestRegisterRoute_ParamReachesHandler proves route params declared in the
// path (":id") are readable from within the gonest Handler via ctx.Param --
// i.e. the fiber.Ctx-backed Responder's GetParam is wired to Fiber's own
// Params(key), completing the Responder contract beyond just Json/Status.
func TestRegisterRoute_ParamReachesHandler(t *testing.T) {
	app := New()

	var gotID string
	if err := app.RegisterRoute(route.HttpGet, "/users/:id", func(ctx *httpctx.Context) {
		gotID = ctx.Param("id")
		ctx.Json(map[string]string{"id": gotID})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if gotID != "42" {
		t.Fatalf("expected param id %q, got %q", "42", gotID)
	}
}
