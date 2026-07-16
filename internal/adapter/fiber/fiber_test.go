package fiber

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/appoptions"
	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/route"
)

// TestRegisterRoute_AllHttpMethods_DispatchToTheirOwnVerb proves fiberMethod
// maps every route.HttpMethod (including Patch/Head/Options/Trace/Connect,
// which used to silently fall through to GET before this test existed) to
// its own real Fiber verb -- not just that registration doesn't error, but
// that a request using the SAME verb actually reaches the handler.
func TestRegisterRoute_AllHttpMethods_DispatchToTheirOwnVerb(t *testing.T) {
	cases := []struct {
		name     string
		method   route.HttpMethod
		httpVerb string
	}{
		{"Get", route.HttpGet, http.MethodGet},
		{"Post", route.HttpPost, http.MethodPost},
		{"Put", route.HttpPut, http.MethodPut},
		{"Patch", route.HttpPatch, http.MethodPatch},
		{"Delete", route.HttpDelete, http.MethodDelete},
		{"Head", route.HttpHead, http.MethodHead},
		{"Options", route.HttpOptions, http.MethodOptions},
		{"Trace", route.HttpTrace, http.MethodTrace},
		{"Connect", route.HttpConnect, http.MethodConnect},
		{"Query", route.HttpQuery, "QUERY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := New()
			called := false
			if err := app.RegisterRoute(tc.method, "/verb", func(ctx *execution.Context) {
				called = true
				ctx.Json(map[string]string{"ok": "true"})
			}); err != nil {
				t.Fatalf("RegisterRoute returned error: %v", err)
			}

			req := httptest.NewRequest(tc.httpVerb, "/verb", nil)
			resp, err := app.FiberApp().Test(req)
			if err != nil {
				t.Fatalf("app.Test returned error: %v", err)
			}
			defer resp.Body.Close()

			if !called {
				t.Fatalf("%s request did not reach the handler registered for route.%s -- fiberMethod likely mapped it to the wrong verb", tc.httpVerb, tc.name)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
		})
	}
}

// TestFiberMethod_UnknownHttpMethod_Panics proves an out-of-range
// route.HttpMethod fails loud (panic) instead of silently registering as
// GET -- the bug this test guards against before the fix.
func TestFiberMethod_UnknownHttpMethod_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected fiberMethod to panic on an unknown HttpMethod, it did not")
		}
	}()

	fiberMethod(route.HttpMethod(999))
}

// TestInit_ZeroValueFiberApp_BecomesUsable proves Init lazily sets up the
// internal *fiber.App on a zero-value FiberApp{} (as reflect.New would
// produce inside NewApp[T] in internal/app, T8) -- without Init, RegisterRoute
// on a zero-value FiberApp would nil-panic dereferencing a nil f.app.
func TestInit_ZeroValueFiberApp_BecomesUsable(t *testing.T) {
	app := &FiberApp{}

	app.Init(appoptions.AppOptions{})

	if err := app.RegisterRoute(route.HttpGet, "/ping", func(ctx *execution.Context) {
		ctx.Status(200).Json(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error after Init: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestInit_CalledTwice_DoesNotResetExistingApp proves Init is idempotent: if
// f.app is already set (e.g. by New()), calling Init again does not replace
// it with a fresh *fiber.App (which would silently drop any routes already
// registered on the original one).
func TestInit_CalledTwice_DoesNotResetExistingApp(t *testing.T) {
	app := New()
	original := app.FiberApp()

	app.Init(appoptions.AppOptions{})

	if app.FiberApp() != original {
		t.Fatalf("Init replaced an already-initialized *fiber.App")
	}
}

// TestNew_RegisterRoute_NoError proves RegisterRoute accepts a simple GET
// route against a fresh FiberApp without returning an error -- the minimal
// "registration succeeds" case before any dispatch is exercised.
func TestNew_RegisterRoute_NoError(t *testing.T) {
	app := New()

	err := app.RegisterRoute(route.HttpGet, "/ping", func(ctx *execution.Context) {
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
	if err := app.RegisterRoute(route.HttpGet, "/ping", func(ctx *execution.Context) {
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

	if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *execution.Context) {
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

	if err := app.RegisterRoute(route.HttpGet, "/greeting", func(ctx *execution.Context) {
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

	if err := app.RegisterRoute(route.HttpPost, "/created", func(ctx *execution.Context) {
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

// TestRegisterRoute_HtmlLandsInRealResponse proves ctx.HTML(s) sets a real
// text/html Content-Type response header and writes s verbatim as the
// response body, as observed by app.Test -- the fiberResponder.HTML
// implementation actually bridges to fiber.Ctx's Type/SendString, not just a
// fake in a unit test.
func TestRegisterRoute_HtmlLandsInRealResponse(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpGet, "/page", func(ctx *execution.Context) {
		ctx.HTML("<h1>hello</h1>")
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected Content-Type to start with %q, got %q", "text/html", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "<h1>hello</h1>" {
		t.Fatalf("expected body %q, got %q", "<h1>hello</h1>", string(body))
	}
}

// TestRegisterRoute_ParamReachesHandler proves route params declared in the
// path (":id") are readable from within the gonest Handler via ctx.Param --
// i.e. the fiber.Ctx-backed Responder's GetParam is wired to Fiber's own
// Params(key), completing the Responder contract beyond just Json/Status.
func TestRegisterRoute_ParamReachesHandler(t *testing.T) {
	app := New()

	var gotID string
	if err := app.RegisterRoute(route.HttpGet, "/users/:id", func(ctx *execution.Context) {
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

// TestListen_OnListenFires_BeforeBlockingForGood proves the onListen
// callback fires exactly once, and fires BEFORE Listen's call blocks for
// good (i.e. before the accept loop takes over permanently) -- per
// design.md's HttpAdapter.Listen (extended contract): Fiber's own
// Hooks().OnListen runs synchronously right after bind succeeds, from
// inside Listen's own goroutine, before app.server.Serve(ln) starts
// blocking. Proven with a channel the callback closes, NOT a time.Sleep
// race: Listen runs in its own goroutine (it blocks by design), the test
// waits on the channel with a timeout, then dials the now-bound address to
// confirm the server really is accepting connections by the time the
// callback fired.
func TestListen_OnListenFires_BeforeBlockingForGood(t *testing.T) {
	app := New()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve an ephemeral port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}

	fired := make(chan struct{})
	var callCount int
	onListen := func() {
		callCount++
		close(fired)
	}

	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- app.Listen(addr, onListen)
	}()
	t.Cleanup(func() {
		if shutdownErr := app.FiberApp().Shutdown(); shutdownErr != nil {
			t.Errorf("Shutdown returned error: %v", shutdownErr)
		}
	})

	select {
	case <-fired:
		// onListen fired -- proceed to confirm the server is really up.
	case err := <-listenErrCh:
		t.Fatalf("Listen returned before onListen fired: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for onListen to fire")
	}

	if callCount != 1 {
		t.Fatalf("expected onListen to fire exactly once, fired %d times", callCount)
	}

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("expected to dial %s after onListen fired, got error: %v", addr, err)
	}
	conn.Close()
}

// TestListen_NilOnListen_DoesNotPanicAndBlocksNormally proves Listen(addr,
// nil) is safe: no hook gets registered with Fiber (Fiber's own OnListen
// handler slice stays empty), and the call still blocks/serves normally
// instead of panicking on a nil func call.
func TestListen_NilOnListen_DoesNotPanicAndBlocksNormally(t *testing.T) {
	app := New()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve an ephemeral port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}

	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- app.Listen(addr, nil)
	}()
	t.Cleanup(func() {
		if shutdownErr := app.FiberApp().Shutdown(); shutdownErr != nil {
			t.Errorf("Shutdown returned error: %v", shutdownErr)
		}
	})

	deadline := time.Now().Add(2 * time.Second)
	var dialErr error
	for time.Now().Before(deadline) {
		var conn net.Conn
		conn, dialErr = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("expected to dial %s with nil onListen, got error: %v", addr, dialErr)
	}

	select {
	case err := <-listenErrCh:
		t.Fatalf("Listen returned unexpectedly (should still be blocking/serving): %v", err)
	default:
		// Still blocking, as expected.
	}
}

// TestRegisterRoute_BodyReachesHandler_WithRealPostedBytes proves the
// fiber.Ctx-backed Responder's Body() is wired to Fiber's own Ctx.Body(),
// via a real HTTP dispatch (app.Test) posting a JSON body -- per L-009's
// precedent, body-reading through fasthttp/fiber has real footguns
// (zero-copy buffer reuse) that only surface through a real request, not a
// fake Responder in a unit test.
func TestRegisterRoute_BodyReachesHandler_WithRealPostedBytes(t *testing.T) {
	app := New()

	want := `{"name":"Ada","age":36}`
	var got string
	if err := app.RegisterRoute(route.HttpPost, "/echo", func(ctx *execution.Context) {
		got = string(ctx.Body())
		ctx.Status(200).Json(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(want))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if got != want {
		t.Fatalf("expected Body() to return %q, got %q", want, got)
	}
}

// TestRegisterRoute_QueriesReachHandler_WithRealQueryString proves the
// fiber.Ctx-backed Responder's Queries() is wired to Fiber's own
// Ctx.Queries(), via a real HTTP dispatch (app.Test) with a real query
// string -- not just that a fake Responder in a unit test returns whatever
// map it was handed.
func TestRegisterRoute_QueriesReachHandler_WithRealQueryString(t *testing.T) {
	app := New()

	var got map[string]string
	if err := app.RegisterRoute(route.HttpGet, "/search", func(ctx *execution.Context) {
		got = ctx.Queries()
		ctx.Status(200).Json(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/search?term=gonest&page=2", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if got["term"] != "gonest" || got["page"] != "2" {
		t.Fatalf("expected Queries() to return term=gonest page=2, got %+v", got)
	}
}

// TestRegisterRoute_QueriesEmpty_WithNoQueryString proves Queries() returns
// an empty (not nil-panicking) map when the request has no query string at
// all.
func TestRegisterRoute_QueriesEmpty_WithNoQueryString(t *testing.T) {
	app := New()

	var got map[string]string
	if err := app.RegisterRoute(route.HttpGet, "/search", func(ctx *execution.Context) {
		got = ctx.Queries()
		ctx.Status(200).Json(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if len(got) != 0 {
		t.Fatalf("expected Queries() to be empty with no query string, got %+v", got)
	}
}

// customTestException mirrors INSIGHT.md's dev-defined-exception pattern
// (`type FooExampleError struct { gonest.HttpException }`) -- a type this
// package never named anywhere in fiber.go, that satisfies
// exception.Exception purely because it embeds exception.HttpException. Used
// to prove RegisterRoute's recover branch detects Exception structurally
// (via a type assertion against the Exception interface), not via a type
// switch hardcoded to the framework's five built-ins.
type customTestException struct {
	exception.HttpException
}

// TestRegisterRoute_HandlerPanicsWithException_RespondsWithStructuredBody
// proves that panicking with anything satisfying exception.Exception --
// each of the five built-ins, plus a dev-defined type that only embeds
// exception.HttpException -- is detected by RegisterRoute's recover branch
// and turned into {status, name, message, details} instead of the generic
// 500 fallback. Table-driven across all six cases since they all assert the
// same shape.
func TestRegisterRoute_HandlerPanicsWithException_RespondsWithStructuredBody(t *testing.T) {
	tests := []struct {
		name        string
		panicValue  any
		wantStatus  int
		wantName    string
		wantDetails any
	}{
		{
			name:        "NotFoundException",
			panicValue:  exception.NewNotFoundException(map[string]any{"id": "42"}),
			wantStatus:  http.StatusNotFound,
			wantName:    "NotFoundException",
			wantDetails: map[string]any{"id": "42"},
		},
		{
			name:        "BadRequestException",
			panicValue:  exception.NewBadRequestException("bad input"),
			wantStatus:  http.StatusBadRequest,
			wantName:    "BadRequestException",
			wantDetails: "bad input",
		},
		{
			name:        "ConflictException",
			panicValue:  exception.NewConflictException("already exists"),
			wantStatus:  http.StatusConflict,
			wantName:    "ConflictException",
			wantDetails: "already exists",
		},
		{
			name:        "UnauthorizedException",
			panicValue:  exception.NewUnauthorizedException("no token"),
			wantStatus:  http.StatusUnauthorized,
			wantName:    "UnauthorizedException",
			wantDetails: "no token",
		},
		{
			name:        "ForbiddenException",
			panicValue:  exception.NewForbiddenException("nope"),
			wantStatus:  http.StatusForbidden,
			wantName:    "ForbiddenException",
			wantDetails: "nope",
		},
		{
			name: "dev-defined exception via embedding",
			panicValue: customTestException{
				HttpException: exception.NewHttpException().SetStatus(http.StatusTeapot).SetName("customTestException").SetMessage("I am a teapot").SetDetails("extra"),
			},
			wantStatus:  http.StatusTeapot,
			wantName:    "customTestException",
			wantDetails: "extra",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := New()

			if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *execution.Context) {
				panic(tc.panicValue)
			}); err != nil {
				t.Fatalf("RegisterRoute returned error: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/boom", nil)
			resp, err := app.FiberApp().Test(req)
			if err != nil {
				t.Fatalf("app.Test returned error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.StatusCode)
			}

			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode JSON body: %v", err)
			}
			if body["name"] != tc.wantName {
				t.Fatalf("expected name %q, got %v", tc.wantName, body["name"])
			}
			if _, ok := body["message"]; !ok {
				t.Fatalf("expected body to contain a message field, got %v", body)
			}
			gotDetails, gotOK := body["details"]
			if !gotOK {
				t.Fatalf("expected body to contain a details field, got %v", body)
			}
			if wantMap, ok := tc.wantDetails.(map[string]any); ok {
				gotMap, ok := gotDetails.(map[string]any)
				if !ok {
					t.Fatalf("expected details to decode as an object, got %T: %v", gotDetails, gotDetails)
				}
				for k, v := range wantMap {
					if gotMap[k] != v {
						t.Fatalf("expected details[%q] = %v, got %v", k, v, gotMap[k])
					}
				}
			} else if gotDetails != tc.wantDetails {
				t.Fatalf("expected details %v, got %v", tc.wantDetails, gotDetails)
			}
		})
	}
}

// TestRegisterRoute_ExceptionWithNilDetails_SerializesDetailsAsJsonNull
// proves that an exception constructed with nil details round-trips as a
// JSON `null` for the "details" key -- present in the body, not omitted --
// since HttpException.Details() explicitly promises to return a bare nil
// rather than a synthesized zero value (see exception.go's doc comment),
// and the recover branch must not paper over that with omitempty or similar.
func TestRegisterRoute_ExceptionWithNilDetails_SerializesDetailsAsJsonNull(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *execution.Context) {
		panic(exception.NewNotFoundException(nil))
	}); err != nil {
		t.Fatalf("RegisterRoute returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes := new(bytes.Buffer)
	if _, err := bodyBytes.ReadFrom(resp.Body); err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	rawBody := bodyBytes.String()

	if !strings.Contains(rawBody, `"details":null`) {
		t.Fatalf(`expected raw body to contain "details":null, got: %s`, rawBody)
	}

	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(rawBody)).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	details, ok := body["details"]
	if !ok {
		t.Fatalf("expected body to contain a details key, got %v", body)
	}
	if details != nil {
		t.Fatalf("expected details to decode as nil, got %v", details)
	}
}

// TestRegisterRoute_HandlerPanicsWithNonException_StillRespondsGeneric500
// proves that panicking with a value that does NOT satisfy exception.Exception
// -- an *errors.errorString, a raw string, and a genuine runtime panic
// (index out of range) -- all still fall through to the EXACT SAME generic
// 500 + "Internal Server Error" body that existed before this feature, and
// crucially that no internal detail (e.g. the wrapped error's own message)
// ever leaks into the response body.
func TestRegisterRoute_HandlerPanicsWithNonException_StillRespondsGeneric500(t *testing.T) {
	tests := []struct {
		name       string
		makePanic  func()
		leakString string
	}{
		{
			name: "errors.New",
			makePanic: func() {
				panic(errors.New("some internal detail"))
			},
			leakString: "some internal detail",
		},
		{
			name: "raw string",
			makePanic: func() {
				panic("raw string panic detail")
			},
			leakString: "raw string panic detail",
		},
		{
			name: "index out of range",
			makePanic: func() {
				s := []int{}
				_ = s[0]
			},
			leakString: "index out of range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := New()

			if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *execution.Context) {
				tc.makePanic()
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

			bodyBytes := new(bytes.Buffer)
			if _, err := bodyBytes.ReadFrom(resp.Body); err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			body := bodyBytes.String()

			if body != "Internal Server Error" {
				t.Fatalf("expected body %q, got %q", "Internal Server Error", body)
			}
			if strings.Contains(body, tc.leakString) {
				t.Fatalf("expected body to NOT contain leaked detail %q, got %q", tc.leakString, body)
			}
		})
	}
}

// TestRegisterRoute_HandlerPanicsWithNil_FallsThroughToGeneric500 proves
// panic(nil) is handled safely. As of Go 1.21, the runtime turns a bare
// panic(nil) into recover() returning a non-nil *runtime.PanicNilError
// (see https://go.dev/doc/go1.21#runtime and go.mod's `go 1.25.0`, well
// past that change) -- so this is NOT the "recover() returned nil, meaning
// no panic occurred" case; recover() genuinely returns a non-nil value that
// does not satisfy exception.Exception, and must fall through to the
// generic 500 fallback exactly like any other non-Exception panic value,
// without crashing the test process.
func TestRegisterRoute_HandlerPanicsWithNil_FallsThroughToGeneric500(t *testing.T) {
	app := New()

	if err := app.RegisterRoute(route.HttpGet, "/boom", func(ctx *execution.Context) {
		panic(nil)
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

	bodyBytes := new(bytes.Buffer)
	if _, err := bodyBytes.ReadFrom(resp.Body); err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if bodyBytes.String() != "Internal Server Error" {
		t.Fatalf("expected body %q, got %q", "Internal Server Error", bodyBytes.String())
	}
}
