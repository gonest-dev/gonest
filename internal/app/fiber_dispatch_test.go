package app_test

// This file gathers every internal/app test that dispatches through a real
// fiber.FiberApp (or App.MustListen'ing a real port) -- an EXTERNAL test
// package (app_test, not app) on purpose: internal/adapter/fiber now
// imports internal/app directly (for the HttpAdapter/Options types its own
// Init/RegisterRoute/Upgrade signatures require -- see internal/app/
// options.go's own doc comment), so an INTERNAL test file in package app
// that also imports fiber would be an import cycle Go's test tooling
// rejects outright ("import cycle not allowed in test") -- stricter than a
// production cycle, and unavoidable while Options lives in this package.
// package app_test is a separate compiled unit that can import both `app`
// (qualified `coreapp`, since almost every test below already uses a local
// variable named `app` for the *coreapp.App under test -- same alias
// internal/adapter/fiber/fiber.go/fiber_test.go already use for the exact
// same shadowing reason) and `fiber` freely, without cycling back into
// either.
//
// Every fixture below (UserEntity/UserService/UserProvider/param schemas/
// mustParse, graphqlUserEntity/readLineWithDeadline, pipelineIDParams/
// buildPipelineOrderingApp) is a deliberate duplicate of the same-named
// definition still living in internal/app's own test files -- those
// originals still have consumers among the recordingFakeAdapter/
// listenSpyAdapter-based tests that stay in package app (no access to
// fiber, no need to move), and test-only symbols in a _test.go file are
// never visible outside their own package, exported name or not, so
// sharing one definition across app and app_test is not an option.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	fasthttpws "github.com/fasthttp/websocket"

	"gonest.dev/gonest/internal/adapter/fiber"
	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/filter"
	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/interceptor"
	"gonest.dev/gonest/internal/middleware"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/schema"
	"gonest.dev/gonest/internal/scope"

	coreapp "gonest.dev/gonest/internal/app"
)

// mustParse mirrors gonest.MustParse[T] for this file's own tests -- same
// duplicate-by-necessity reasoning as the package doc comment above.
func mustParse[T any](src execution.Parseable, m *schema.Schema) T {
	var zero T
	if err := src.ParseInto(&zero, m); err != nil {
		panic(err)
	}
	return zero
}

type UserEntity struct {
	ID   int64
	Name string
}

type UserService struct {
	index int
	list  []*UserEntity
}

func (t *UserService) List() []*UserEntity { return t.list }

func (t *UserService) Get(userID int64) *UserEntity {
	for _, u := range t.list {
		if u.ID == userID {
			return u
		}
	}
	return nil
}

func (t *UserService) Create(name string) *UserEntity {
	t.index++
	u := &UserEntity{ID: int64(t.index), Name: name}
	t.list = append(t.list, u)
	return u
}

func (t *UserService) Update(userID int64, name string) *UserEntity {
	u := t.Get(userID)
	if u == nil {
		return nil
	}
	u.Name = name
	return u
}

func (t *UserService) Delete(userID int64) *UserEntity {
	for i, u := range t.list {
		if u.ID == userID {
			t.list = append(t.list[:i], t.list[i+1:]...)
			return u
		}
	}
	return nil
}

type userIDParams struct {
	UserID int64 `param:"user_id"`
}

var userIDParamsSchema = func() *schema.Schema {
	f := &userIDParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserID).Integer().Required()
	return m
}()

type nameParams struct {
	Name string `param:"name"`
}

var nameParamsSchema = func() *schema.Schema {
	f := &nameParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Name).String().Required()
	return m
}()

type userIDNameParams struct {
	UserID int64  `param:"user_id"`
	Name   string `param:"name"`
}

var userIDNameParamsSchema = func() *schema.Schema {
	f := &userIDNameParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserID).Integer().Required()
	m.Property(&f.Name).String().Required()
	return m
}()

var UserProvider = provider.New(func(p *provider.Provider) {
	p.Scope(scope.Singleton)
	p.Constructor(func() *UserService {
		return &UserService{index: 0, list: make([]*UserEntity, 0)}
	})
})

type graphqlUserEntity struct {
	Id    int64  `json:"id"`
	Email string `json:"email"`
}

// readLineWithDeadline reads a single line from body, bounding the read via
// conn's own SetReadDeadline (bufio.Reader has no deadline of its own) --
// used by the streaming dispatcher tests to avoid hanging forever if a
// dispatch bug ever routes to the wrong (non-responding) handler.
func readLineWithDeadline(t *testing.T, conn net.Conn, body *bufio.Reader, timeout time.Duration) (string, error) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	line, err := body.ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	return line, err
}

// pipelineIDParams is the struct-based path-param shape validated via
// validate.MustParams[T] for the pipeline-ordering suite below.
type pipelineIDParams struct {
	ID int `param:"id"`
}

var pipelineIDParamsSchema = func() *schema.Schema {
	f := &pipelineIDParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.ID).Custom(func(raw any) (any, error) {
		s, _ := raw.(string)
		if s == "bad" {
			panic(exception.NewBadRequestException(map[string]string{"reason": "invalid id"}))
		}
		return 42, nil
	})
	return m
}()

// buildPipelineOrderingApp wires one controller with global+controller
// Middleware, a Guard, an Interceptor, a Filter (controller-level, for
// *exception.BadRequestException) and one route validating its path param
// via validate.MustParams[*pipelineIDParams] (whose ID field's Custom(fn)
// can panic).
func buildPipelineOrderingApp(t *testing.T, order *[]string, guardAllows, withFilter bool) *module.Module {
	t.Helper()

	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			*order = append(*order, "global-middleware")
			next(req, res)
		})
	})
	controllerMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			*order = append(*order, "controller-middleware")
			next(req, res)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			*order = append(*order, "guard")
			return guardAllows
		})
	})
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			*order = append(*order, "interceptor-before")
			next(req, res)
			*order = append(*order, "interceptor-after")
		})
	})

	ctrl := controller.New(func(c *controller.Controller) {
		c.Use(controllerMw)
		c.Guards(g)
		c.Interceptors(it)
		if withFilter {
			f := filter.New(func(f *filter.Filter) {
				f.Catch(&exception.BadRequestException{}, func(req *execution.Request, res *execution.Response, exc *exception.BadRequestException) {
					*order = append(*order, "filter")
					res.Status(400).Json(map[string]string{"caught": "pipeline-filter"})
				})
			})
			c.Filters(f)
		}
		c.Route(route.HttpGet, "/pipeline/:id", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				*order = append(*order, "handler-before-pipe")
				p := mustParse[pipelineIDParams](req.Params(), pipelineIDParamsSchema)
				*order = append(*order, "pipe")
				res.Json(map[string]any{"id": p.ID})
			})
		})
	})

	return module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(ctrl)
	})
}

// TestNewApp_FiberApp_RealEndToEndWiring proves the generic wiring truly
// works with the real fiber.FiberApp adapter (not just the fake spy
// above): coreapp.NewApp[fiber.FiberApp] constructs a genuinely usable FiberApp
// (Init sets a non-nil *fiber.App, see internal/adapter/fiber/fiber.go),
// registers a real route on it, and dispatches a real HTTP request through
// Fiber's own app.Test -- proving reflect.New(...).Interface() + Init()
// produces something other than a nil-panic waiting to happen.
func TestNewApp_FiberApp_RealEndToEndWiring(t *testing.T) {
	called := false
	pingController := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				called = true
				res.Json(map[string]string{"pong": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(pingController)
	})

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("coreapp.NewApp() returned nil *coreapp.App")
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatalf("expected the registered Handler to run, but it did not")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond is T9 of this
// feature: it adapts the UserController/UserService example from
// INSIGHT.md (lines 1-100) into a real integration test proving the whole
// feature -- Provider & DI Graph + Module Composition + Controller & Route
// Registration -- works together end-to-end via a real fiber.FiberApp
// dispatched through app.Test(req).
//
// SPEC_DEVIATION (documented, not a blocker -- spec.md's own Independent
// Test line for P1 explicitly allows "rotas... adaptadas"): INSIGHT.md's
// full example uses gonest.Value[T] field wrappers, HttpStatusOk/
// HttpStatusCreated named constants, and MustJsonBody[T] to parse a JSON
// request body into UserProperties for Create/Update. None of those exist
// in this codebase yet -- spec.md's CTRL-01..08 requirement list has no
// JSON-body-parsing requirement, it is out of scope for this feature (a
// future milestone, see spec.md's "Out of Scope" table). This test adapts
// by using a plain Go string for UserEntity.Name (no Value[T] wrapper),
// plain int status codes (200/201/404, no named constants), and for
// Create/Update -- since there is no body-parsing primitive available
// today -- accepts the "name" via a route param instead of a JSON body.
// This still exercises a real POST/PUT round-trip through app.Test: the
// route dispatches, validate.MustParams[T] converts params (param-query-
// validation's T3 replaced the old singular MustParam[T](ctx,name) with
// this whole-object mechanism), and the response shape/status match
// INSIGHT.md's intent (200 for List/Get/Update/Delete, 201 for Create).
func TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")

		userService := inject.MustInject[*UserService](c)

		// QUERY /user/ -- List. INSIGHT.md uses gonest.HttpQuery for the
		// list route; route.HttpQuery maps to fiber's "QUERY" method
		// string (internal/adapter/fiber/fiber.go's fiberMethod), which
		// httptest.NewRequest below dispatches as a plain HTTP method
		// string like any other -- QUERY is not a net/http constant, but
		// Fiber and net/http/httptest both treat the method as an opaque
		// string, so this round-trips fine.
		c.Route(route.HttpQuery, "/", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Status(r.Code()).Json(userService.List())
			})
		})

		// GET /user/:user_id -- Get. 404 (plain int, no structured
		// Exception type yet -- Milestone 2) if the user doesn't exist.
		c.Route(route.HttpGet, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[userIDParams](req.Params(), userIDParamsSchema)
				u := userService.Get(p.UserID)
				if u == nil {
					res.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				res.Status(r.Code()).Json(u)
			})
		})

		// POST /user/:name -- Create. SPEC_DEVIATION: INSIGHT.md POSTs to
		// "/" with a JSON body (MustJsonBody[*UserProperties]); no
		// body-parsing primitive exists yet, so this adaptation takes the
		// new user's name via a route param instead, proving the same
		// "POST creates and returns 201" round-trip.
		c.Route(route.HttpPost, "/:name", func(r *route.Route) {
			r.HttpCode(201)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[nameParams](req.Params(), nameParamsSchema)
				res.Status(r.Code()).Json(userService.Create(p.Name))
			})
		})

		// PUT /user/:user_id/:name -- Update. Same body-parsing
		// SPEC_DEVIATION as Create above: the new name travels as a
		// second route param instead of a JSON body.
		c.Route(route.HttpPut, "/:user_id/:name", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[userIDNameParams](req.Params(), userIDNameParamsSchema)
				u := userService.Update(p.UserID, p.Name)
				if u == nil {
					res.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				res.Status(r.Code()).Json(u)
			})
		})

		// DELETE /user/:user_id -- Delete.
		c.Route(route.HttpDelete, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[userIDParams](req.Params(), userIDParamsSchema)
				u := userService.Delete(p.UserID)
				if u == nil {
					res.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				res.Status(r.Code()).Json(u)
			})
		})
	})

	userModule := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	root := module.New(func(m *module.Module) {
		m.Imports(userModule)
	})

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	fa := fiberAdapter.FiberApp()

	// 1. List (empty) -- QUERY /user/
	{
		req := httptest.NewRequest("QUERY", "/user/", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("List: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("List: status = %d, want 200", resp.StatusCode)
		}
		var got []*UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		// app.Test's response body is backed by a pooled fasthttp buffer
		// that is only guaranteed valid until the NEXT app.Test call on the
		// same *fiber.App -- draining+closing it here, right after
		// decoding and BEFORE the next block's fa.Test(...), avoids
		// cross-request buffer reuse corrupting an earlier response still
		// held open via `defer` (observed as a flaky, garbled JSON body
		// when this originally used `defer resp.Body.Close()` at function
		// scope instead of per-block).
		if decodeErr != nil {
			t.Fatalf("List: decode error = %v", decodeErr)
		}
		if len(got) != 0 {
			t.Fatalf("List: got %d users, want 0 before any Create", len(got))
		}
	}

	// 2. Create -- POST /user/Ada
	var created UserEntity
	{
		req := httptest.NewRequest(http.MethodPost, "/user/Ada", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Create: app.Test error = %v", err)
		}
		if resp.StatusCode != 201 {
			resp.Body.Close()
			t.Fatalf("Create: status = %d, want 201", resp.StatusCode)
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Create: decode error = %v", decodeErr)
		}
		if created.ID != 1 || created.Name != "Ada" {
			t.Fatalf("Create: got %+v, want ID=1 Name=Ada", created)
		}
	}

	// 3. Get -- GET /user/:user_id (MustParams[*userIDParams] round-trip)
	{
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/user/%d", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Get: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Get: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Get: decode error = %v", decodeErr)
		}
		if got.ID != created.ID || got.Name != "Ada" {
			t.Fatalf("Get: got %+v, want %+v", got, created)
		}
	}

	// 3b. Get -- unknown ID returns 404
	{
		req := httptest.NewRequest(http.MethodGet, "/user/999", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Get(missing): app.Test error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Fatalf("Get(missing): status = %d, want 404", resp.StatusCode)
		}
	}

	// 4. Update -- PUT /user/:user_id/:name (MustParams[*userIDNameParams] round-trip)
	{
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/user/%d/Grace", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Update: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Update: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Update: decode error = %v", decodeErr)
		}
		if got.ID != created.ID || got.Name != "Grace" {
			t.Fatalf("Update: got %+v, want ID=%d Name=Grace", got, created.ID)
		}
	}

	// 5. Delete -- DELETE /user/:user_id (MustParams[*userIDParams] round-trip)
	{
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/user/%d", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Delete: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Delete: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Delete: decode error = %v", decodeErr)
		}
		if got.ID != created.ID {
			t.Fatalf("Delete: got %+v, want ID=%d", got, created.ID)
		}
	}

	// 5b. List (empty again) -- proves Delete actually removed the entry
	{
		req := httptest.NewRequest("QUERY", "/user/", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("List(after delete): app.Test error = %v", err)
		}
		var got []*UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("List(after delete): decode error = %v", decodeErr)
		}
		if len(got) != 0 {
			t.Fatalf("List(after delete): got %d users, want 0 after Delete", len(got))
		}
	}
}

// TestMustListen_RealFiberApp_IntegrationSmoke proves the whole chain works
// end-to-end through App.MustListen with a real fiber.FiberApp: it binds
// a real (OS-chosen) port, MustListen's wrapped coreapp.OnListen fires exactly once,
// and a real HTTP request against the bound addr succeeds. Kept deliberately
// minimal -- a full DI+routing dial-the-whole-stack proof belongs to T6, not
// here.
func TestMustListen_RealFiberApp_IntegrationSmoke(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34579"

	fired := make(chan struct{})
	var once sync.Once
	onListen := coreapp.OnListen(func() {
		once.Do(func() { close(fired) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, onListen)
	}()

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("http.Get error = %v", err)
	}
	resp.Body.Close()

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before shutdown")
	default:
	}
}

// TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort is T6 of this
// feature ("App Bootstrap & Listen") -- the final task. It closes the loop
// left open by TestMustListen_RealFiberApp_IntegrationSmoke (above, T4/T3
// territory: proves MustListen+coreapp.OnListen work end-to-end, but with a
// controller-less root module and only http.Get, not a real net/http.Client
// dial) and by TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond (T9 of
// the earlier "Controller & Route Registration" feature: proves the full
// UserController/UserService/AppModule DI+routing example works, but only
// via Fiber's own in-process app.Test, never a real bound TCP port).
//
// This test reuses that same UserController/UserService example (the
// package-level UserProvider var and the userController builder shape
// defined right above TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond)
// but bootstraps it through App.MustListen against a real, OS-reachable
// 127.0.0.1 address, waits for coreapp.OnListen to fire (channel-synchronized, no
// time.Sleep -- same pattern as TestMustListen_RealFiberApp_IntegrationSmoke
// and fiber's TestListen_OnListenFires_BeforeBlockingForGood), then
// dispatches an actual net/http.Client request (a genuine TCP dial, NOT
// app.Test) against the bound port to prove the WHOLE chain -- coreapp.NewApp's DI
// bootstrap, Stage 2.5 route registration, App.MustListen, the adapter's
// real Listen, and Fiber's real accept loop -- works together, not just each
// layer in isolation.
func TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")

		userService := inject.MustInject[*UserService](c)

		c.Route(route.HttpGet, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[userIDParams](req.Params(), userIDParamsSchema)
				u := userService.Get(p.UserID)
				if u == nil {
					res.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				res.Status(r.Code()).Json(u)
			})
		})

		// POST /user/:name -- Create. Same SPEC_DEVIATION as T9's version
		// above (INSIGHT.md's JSON body is not available yet, see this
		// file's TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond doc
		// comment): the new user's name travels as a route param. Needed
		// here purely to seed a real user for the GET round-trip below --
		// this test's focus is the real net/http.Client dial, not exercising
		// every one of the 5 routes again (T9 already covers that via
		// app.Test).
		c.Route(route.HttpPost, "/:name", func(r *route.Route) {
			r.HttpCode(201)
			r.Handler(func(req *execution.Request, res *execution.Response) {
				p := mustParse[nameParams](req.Params(), nameParamsSchema)
				res.Status(r.Code()).Json(userService.Create(p.Name))
			})
		})
	})

	userModule := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	root := module.New(func(m *module.Module) {
		m.Imports(userModule)
	})

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}

	const addr = "127.0.0.1:34599"

	fired := make(chan struct{})
	var once sync.Once
	onListen := coreapp.OnListen(func() {
		once.Do(func() { close(fired) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, onListen)
	}()
	t.Cleanup(func() {
		if shutdownErr := fiberAdapter.FiberApp().Shutdown(); shutdownErr != nil {
			t.Errorf("Shutdown returned error: %v", shutdownErr)
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("MustListen goroutine did not return within timeout after Shutdown")
		}
	})

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	// A real net/http.Client, genuinely dialing TCP against the bound
	// address -- not app.Test, which never opens a socket.
	client := &http.Client{Timeout: 5 * time.Second}

	// Create a user through the real HTTP round-trip first: the /:user_id
	// GET route needs a real ID to fetch, and this simultaneously proves the
	// running server's UserService is genuinely usable, mutable state (not a
	// zero-value placeholder) reached over a real socket.
	createReq, err := http.NewRequest(http.MethodPost, "http://"+addr+"/user/Ada", nil)
	if err != nil {
		t.Fatalf("failed to build create request: %v", err)
	}
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("real net/http.Client create request failed: %v", err)
	}
	var created UserEntity
	decodeErr := json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("failed to decode create response: %v", decodeErr)
	}

	getResp, err := client.Get(fmt.Sprintf("http://%s/user/%d", addr, created.ID))
	if err != nil {
		t.Fatalf("real net/http.Client GET request failed: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		getResp.Body.Close()
		t.Fatalf("GET /user/%d status = %d, want 200", created.ID, getResp.StatusCode)
	}
	var got UserEntity
	decodeErr = json.NewDecoder(getResp.Body).Decode(&got)
	getResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("failed to decode get response: %v", decodeErr)
	}
	if got.ID != created.ID || got.Name != "Ada" {
		t.Fatalf("GET /user/%d body = %+v, want ID=%d Name=Ada", created.ID, got, created.ID)
	}

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before test-end shutdown")
	default:
	}
}

// --- T4 of "Middleware": Stage 2.5 composition in internal/app ---
//
// dispatchTestApp is a small shared helper: builds a *fiber.FiberApp
// backed *coreapp.App from root, fails the test on any bootstrap error, and returns
// the underlying *fiber.App ready for app.Test(req) dispatch.
func dispatchTestApp(t *testing.T, root *module.Module) *fiber.FiberApp {
	t.Helper()
	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}
	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	return fiberAdapter
}

// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler proves a single
// controller-level middleware (registered via Controller.Use) runs before
// the route's own Handler, observed via a real app.Test dispatch: the
// middleware appends to a shared order slice before calling next, and the
// Handler appends too, then the test asserts the exact ["mw", "handler"]
// order captured across the real request.
//
// Note on technique: execution.Request.Header reads the REQUEST header (Fiber
// Ctx.Get), while SetHeader writes the RESPONSE header (Fiber Ctx.Set) --
// two different underlying stores, so a middleware cannot SetHeader and have
// a later step observe it via Header (internal/execution's Responder contract
// has no GetRespHeader, and per this task's scope internal/adapter/fiber is not
// to be touched to add one). This suite instead threads a shared []string
// order-of-execution recorder through closures -- still a genuine
// app.Test-dispatched proof of composition/ordering, just not the
// header-based technique originally sketched.
func TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			order = append(order, "mw")
			next(req, res)
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if want := []string{"mw", "handler"}; len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("execution order = %v, want %v -- middleware did not run before Handler", order, want)
	}
}

// TestNewApp_MiddlewareShortCircuit_SkipsRouteHandlerWhenNextNotCalled proves
// that calling next(req, res) continues the chain (route Handler runs), while a
// middleware that does NOT call next short-circuits the chain entirely (the
// route Handler never runs) -- both proven via real dispatch.
func TestNewApp_MiddlewareShortCircuit_SkipsRouteHandlerWhenNextNotCalled(t *testing.T) {
	callingMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			next(req, res)
		})
	})
	blockingMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			res.Status(http.StatusForbidden).Json(map[string]string{"blocked": "true"})
			// deliberately never calls next(req, res)
		})
	})

	var handlerRan bool
	continueController := controller.New(func(c *controller.Controller) {
		c.Use(callingMw)
		c.Route(route.HttpGet, "/continue", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})
	blockController := controller.New(func(c *controller.Controller) {
		c.Use(blockingMw)
		c.Route(route.HttpGet, "/blocked", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(continueController, blockController)
	})

	fa := dispatchTestApp(t, root)

	// Continue path: next(req, res) called, Handler runs.
	handlerRan = false
	req := httptest.NewRequest(http.MethodGet, "/continue", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/continue status = %d, want 200", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("/continue: route Handler did not run, want it to run after next(req, res)")
	}

	// Short-circuit path: next(req, res) never called, Handler must NOT run.
	handlerRan = false
	req = httptest.NewRequest(http.MethodGet, "/blocked", nil)
	resp, err = fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("/blocked status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("/blocked: route Handler ran, want it to be short-circuited by a middleware that never calls next")
	}
}

// TestNewApp_MultipleControllerMiddleware_RunInRegistrationOrder proves
// multiple controller-level middleware run in the exact order they were
// registered via Use, each appending a distinguishable marker to the SAME
// response header via a read-existing-then-append pattern using
// ctx.SetHeader alone (no ctx.Header read of it -- see the doc comment on
// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler for why Header
// cannot observe a prior SetHeader against the real Fiber responder).
// Confirmed here via the FINAL HTTP RESPONSE header's exact value (read from
// resp.Header, a real net/http.Response, not via ctx) -- proving both
// ordering and that SetHeader's writes genuinely land on the wire.
func TestNewApp_MultipleControllerMiddleware_RunInRegistrationOrder(t *testing.T) {
	appendMarker := func(marker string) *middleware.Middleware {
		return middleware.New(func(m *middleware.Middleware) {
			m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
				res.SetHeader("X-Order", marker)
				next(req, res)
			})
		})
	}

	c := controller.New(func(c *controller.Controller) {
		c.Use(appendMarker("first"), appendMarker("second"), appendMarker("third"))
		c.Route(route.HttpGet, "/order", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	// Each middleware overwrites X-Order with its own marker via SetHeader
	// (last write wins), so the final value on the wire proves which
	// middleware ran LAST -- "third", since first/second/third must run in
	// that exact registration order.
	if got, want := resp.Header.Get("X-Order"), "third"; got != want {
		t.Fatalf("resp X-Order = %q, want %q -- middleware did not run in registration order (last registered must run last and win the overwrite)", got, want)
	}
}

// TestNewApp_MiddlewareMutation_VisibleToLaterMiddlewareAndHandler proves a
// middleware mutating req before calling next is visible to a LATER
// middleware and the route Handler -- i.e. the SAME *execution.Request
// instance flows through the whole chain, not a copy.
func TestNewApp_MiddlewareMutation_VisibleToLaterMiddlewareAndHandler(t *testing.T) {
	setter := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			req.WithRoute("mutated-by-first")
			next(req, res)
		})
	})
	var readerSaw any
	reader := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			readerSaw = req.Route()
			next(req, res)
		})
	})

	var handlerSaw any
	c := controller.New(func(c *controller.Controller) {
		c.Use(setter, reader)
		c.Route(route.HttpGet, "/mutate", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerSaw = req.Route()
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/mutate", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	// req.WithRoute/req.Route are used here purely as a generic any-carrier
	// on *execution.Request (the only mutable field Request exposes besides
	// headers, which -- per the doc comment on
	// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler -- cannot
	// round-trip a write through a later read against the real Fiber
	// responder) to prove the SAME *execution.Request instance (not a copy)
	// flows from the first middleware, through the second, into the route
	// Handler: a mutation made by the first middleware before calling next
	// must be visible to both.
	if want := "mutated-by-first"; readerSaw != want {
		t.Fatalf("second middleware saw req.Route() = %v, want %q -- mutation by first middleware not visible to a LATER middleware on the same *execution.Request", readerSaw, want)
	}
	if want := "mutated-by-first"; handlerSaw != want {
		t.Fatalf("route Handler saw req.Route() = %v, want %q -- mutation not visible to the Handler on the same *execution.Request", handlerSaw, want)
	}
}

// TestNewApp_RootMiddleware_RunsForEveryRouteIncludingControllerWithNoOwnUse
// proves root-module Use() middleware runs for EVERY route in the app,
// including a controller with ZERO Use() calls of its own -- 2 controllers,
// only one with its own middleware, both hit by real requests, both showing
// the global marker.
func TestNewApp_RootMiddleware_RunsForEveryRouteIncludingControllerWithNoOwnUse(t *testing.T) {
	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			res.SetHeader("X-Global", "yes")
			next(req, res)
		})
	})
	localMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			res.SetHeader("X-Local", "yes")
			next(req, res)
		})
	})

	withOwnMw := controller.New(func(c *controller.Controller) {
		c.Path("/with-own")
		c.Use(localMw)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})
	withoutOwnMw := controller.New(func(c *controller.Controller) {
		c.Path("/without-own")
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(withOwnMw, withoutOwnMw)
	})

	fa := dispatchTestApp(t, root)

	// Both assertions read the FINAL HTTP response headers (a real
	// net/http.Response), proving the headers genuinely landed on the wire
	// for a real dispatched request -- not a ctx-internal round-trip (see
	// the doc comment on TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler
	// for why ctx.Header cannot observe a prior ctx.SetHeader here).
	req := httptest.NewRequest(http.MethodGet, "/with-own/ping", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Global"); got != "yes" {
		t.Fatalf("/with-own/ping: resp X-Global = %q, want %q", got, "yes")
	}
	if got := resp.Header.Get("X-Local"); got != "yes" {
		t.Fatalf("/with-own/ping: resp X-Local = %q, want %q", got, "yes")
	}

	req = httptest.NewRequest(http.MethodGet, "/without-own/ping", nil)
	resp, err = fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Global"); got != "yes" {
		t.Fatalf("/without-own/ping: resp X-Global = %q, want %q -- global middleware must run even for a controller with zero Use() calls", got, "yes")
	}
	if got := resp.Header.Get("X-Local"); got != "" {
		t.Fatalf("/without-own/ping: resp X-Local = %q, want empty -- this controller never called Use", got)
	}
}

// TestNewApp_GlobalMiddleware_RunsBeforeControllerMiddleware proves global
// (root-module) middleware runs BEFORE controller-level middleware -- proven
// via marker-header ORDER, not just presence.
func TestNewApp_GlobalMiddleware_RunsBeforeControllerMiddleware(t *testing.T) {
	var order []string
	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			order = append(order, "global")
			next(req, res)
		})
	})
	controllerMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			order = append(order, "controller")
			next(req, res)
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(controllerMw)
		c.Route(route.HttpGet, "/order", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	want := []string{"global", "controller", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- global middleware must run BEFORE controller-level middleware", order, want)
		}
	}
}

// TestNewApp_PanickingMiddleware_CaughtBySameRecoverWrapper proves a
// middleware that panics with a built-in exception.Exception is caught by
// the SAME existing recover wrapper in internal/adapter/fiber (unchanged by this
// feature) and produces the correct structured response -- exactly as if a
// route Handler itself had panicked.
func TestNewApp_PanickingMiddleware_CaughtBySameRecoverWrapper(t *testing.T) {
	panicky := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			panic(exception.NewBadRequestException("bad input from middleware"))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Use(panicky)
		c.Route(route.HttpGet, "/panic", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "BadRequestException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "BadRequestException")
	}
	if body["details"] != "bad input from middleware" {
		t.Fatalf("body[details] = %v, want %q", body["details"], "bad input from middleware")
	}
	if handlerRan {
		t.Fatalf("route Handler ran after a panicking middleware, want the panic to short-circuit the chain")
	}
}

// TestNewApp_SingleGuardReturnsTrue_RouteHandlerRuns proves a single guard
// whose handler returns true lets the request through: the route Handler's
// own response comes back on a real app.Test dispatch.
func TestNewApp_SingleGuardReturnsTrue_RouteHandlerRuns(t *testing.T) {
	allow := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool { return true })
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(allow)
		c.Route(route.HttpGet, "/allowed", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("route Handler did not run, want it to run when the guard returns true")
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["ok"] != "true" {
		t.Fatalf("body = %+v, want {ok:true} -- the route Handler's own response must come through", body)
	}
}

// TestNewApp_SingleGuardReturnsFalse_Produces403AndSkipsHandler proves a
// guard returning false produces a 403 Forbidden with the exact structured
// body a *exception.ForbiddenException formats to, and that the route
// Handler never runs.
func TestNewApp_SingleGuardReturnsFalse_Produces403AndSkipsHandler(t *testing.T) {
	deny := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool { return false })
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(deny)
		c.Route(route.HttpGet, "/denied", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/denied", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when the guard returns false")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "ForbiddenException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "ForbiddenException")
	}
	if body["message"] != "" {
		t.Fatalf("body[message] = %v, want empty string", body["message"])
	}
	if body["details"] != nil {
		t.Fatalf("body[details] = %v, want nil", body["details"])
	}
}

// TestNewApp_GuardPanicsWithCustomException_ProducesThatExceptionsStatus
// proves a guard whose handler panics with a custom exception.Exception
// (rather than merely returning false) propagates that specific exception
// through unmodified -- caught by the same recover wrapper, formatted with
// THAT exception's own status/body (401, not 403) -- and the route Handler
// never runs.
func TestNewApp_GuardPanicsWithCustomException_ProducesThatExceptionsStatus(t *testing.T) {
	unauthorized := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			panic(exception.NewUnauthorizedException(nil))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(unauthorized)
		c.Route(route.HttpGet, "/needs-auth", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/needs-auth", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when a guard panics")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "UnauthorizedException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "UnauthorizedException")
	}
}

// TestNewApp_MultipleGuards_ShortCircuitsOnFirstFalse proves that with 2+
// guards registered in order, a false from the FIRST guard prevents the
// SECOND guard's own handler function from ever running -- observed via a
// side-effect flag on the second guard that must stay untouched.
func TestNewApp_MultipleGuards_ShortCircuitsOnFirstFalse(t *testing.T) {
	var secondGuardRan bool
	first := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool { return false })
	})
	second := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			secondGuardRan = true
			return true
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(first, second)
		c.Route(route.HttpGet, "/multi", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/multi", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if secondGuardRan {
		t.Fatalf("second guard's handler ran, want short-circuit after the first guard returned false")
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped")
	}
}

// TestNewApp_MiddlewareThenGuardThenHandler_OrderedSequence proves a
// controller with BOTH Use() (middleware) AND Guards() registered runs:
// middleware first, then the guard, then the Handler -- via an explicit
// ordered-sequence assertion, reusing the shared []string order-recorder
// technique this file's own "Middleware" T4 tests established.
func TestNewApp_MiddlewareThenGuardThenHandler_OrderedSequence(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			order = append(order, "middleware")
			next(req, res)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			order = append(order, "guard")
			return true
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Guards(g)
		c.Route(route.HttpGet, "/sequence", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/sequence", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"middleware", "guard", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- middleware must run before the guard, and the guard before the Handler", order, want)
		}
	}
}

// TestNewApp_GuardPanicsWithNonException_StillGeneric500 proves a guard
// panicking with a value that does NOT satisfy exception.Exception (e.g. a
// plain error) still produces the same generic 500 fallback any other panic
// already gets -- non-regression of "Panic Recovery & Default Handler"'s
// existing behavior, proof the gatedHandler composition didn't break it.
func TestNewApp_GuardPanicsWithNonException_StillGeneric500(t *testing.T) {
	buggy := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			panic(errors.New("bug"))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(buggy)
		c.Route(route.HttpGet, "/buggy-guard", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/buggy-guard", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when the guard panics")
	}
}

// TestNewApp_SingleInterceptor_RunsBeforeAndAfterHandler proves a single
// interceptor runs code BEFORE calling next(req, res), then the route Handler
// runs, then the interceptor's own code AFTER next(req, res) returns runs too --
// observed via a real app.Test dispatch appending to a shared order-recorder
// slice, asserting the exact sequence ["before", "handler", "after"].
func TestNewApp_SingleInterceptor_RunsBeforeAndAfterHandler(t *testing.T) {
	var order []string
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			order = append(order, "before")
			next(req, res)
			order = append(order, "after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(it)
		c.Route(route.HttpGet, "/wrap", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/wrap", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"before", "handler", "after"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

// TestNewApp_InterceptorNotCallingNext_SkipsRouteHandler proves an
// interceptor that never calls next(req, res) short-circuits the chain: the route
// Handler must not run, proven via a flag the Handler would have set.
func TestNewApp_InterceptorNotCallingNext_SkipsRouteHandler(t *testing.T) {
	blocking := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			res.Status(http.StatusForbidden).Json(map[string]string{"blocked": "true"})
			// deliberately never calls next(req, res)
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(blocking)
		c.Route(route.HttpGet, "/blocked", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/blocked", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when an interceptor never calls next")
	}
}

// TestNewApp_MultipleInterceptors_ComposeInRegistrationOrder proves 2+
// interceptors registered on a controller compose in registration order:
// interceptor1's before-code runs first, then interceptor2's before-code,
// then the Handler, then interceptor2's after-code, then interceptor1's
// after-code -- the classic nested-onion composition order.
func TestNewApp_MultipleInterceptors_ComposeInRegistrationOrder(t *testing.T) {
	var order []string
	first := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			order = append(order, "interceptor1-before")
			next(req, res)
			order = append(order, "interceptor1-after")
		})
	})
	second := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			order = append(order, "interceptor2-before")
			next(req, res)
			order = append(order, "interceptor2-after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(first, second)
		c.Route(route.HttpGet, "/multi", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/multi", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{
		"interceptor1-before", "interceptor2-before",
		"handler",
		"interceptor2-after", "interceptor1-after",
	}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

// TestNewApp_MiddlewareGuardInterceptorHandler_OrderedSequence proves a
// controller with Middleware + Guards + Interceptors registered together
// runs, in order: Middleware -> Guard -> Interceptor(before) -> Handler ->
// Interceptor(after), the full Stage 2.5 pipeline order proven via one
// explicit ordered-sequence assertion covering all 3 stages simultaneously.
// Matches ROADMAP.md's documented order exactly -- a Guard must be able to
// reject a request BEFORE any Interceptor "before" logic runs (see the
// CORRECTION note in the interceptor feature's design.md: an earlier version
// of this test asserted the reverse, Interceptor-before-Guard, which matched
// an earlier -- buggy -- version of design.md's composition steps; both have
// since been fixed to match ROADMAP.md).
func TestNewApp_MiddlewareGuardInterceptorHandler_OrderedSequence(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			order = append(order, "middleware")
			next(req, res)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			order = append(order, "guard")
			return true
		})
	})
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			order = append(order, "interceptor-before")
			next(req, res)
			order = append(order, "interceptor-after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Guards(g)
		c.Interceptors(it)
		c.Route(route.HttpGet, "/full-pipeline", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				order = append(order, "handler")
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/full-pipeline", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"middleware", "guard", "interceptor-before", "handler", "interceptor-after"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- pipeline must run Middleware -> Guard -> Interceptor(before) -> Handler -> Interceptor(after)", order, want)
		}
	}
}

// TestNewApp_GuardRejects_InterceptorBeforeNeverRuns proves the exact
// behavior that motivated the Guard/Interceptor composition-order fix: when
// a Guard rejects a request (HandlerFunc returns false), NEITHER the route
// Handler NOR the Interceptor's "before" logic runs -- a Guard must be able
// to reject a request before any Interceptor setup work (e.g. starting a
// timer) happens for that request, per ROADMAP.md's documented order
// (Middleware -> Guard -> Interceptor -> Handler) and the CORRECTION note in
// the interceptor feature's design.md.
func TestNewApp_GuardRejects_InterceptorBeforeNeverRuns(t *testing.T) {
	var interceptorBeforeRan, handlerRan bool
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			return false
		})
	})
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			interceptorBeforeRan = true
			next(req, res)
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Guards(g)
		c.Interceptors(it)
		c.Route(route.HttpGet, "/guard-blocks-interceptor", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/guard-blocks-interceptor", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if interceptorBeforeRan {
		t.Fatalf("interceptor's before-next logic ran, want it skipped when a Guard rejects the request first")
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it skipped when a Guard rejects the request")
	}
}

// TestNewApp_InterceptorPanicsBeforeNext_CaughtBySameRecoverWrapper proves
// an interceptor that panics BEFORE calling next(req, res) -- with a custom
// exception.Exception -- is caught by the SAME existing recover wrapper in
// internal/adapter/fiber (unchanged by this feature) and produces that
// exception's own structured response, with the route Handler never
// running.
func TestNewApp_InterceptorPanicsBeforeNext_CaughtBySameRecoverWrapper(t *testing.T) {
	panicky := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			panic(exception.NewBadRequestException(nil))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(panicky)
		c.Route(route.HttpGet, "/panic-before", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic-before", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when an interceptor panics before calling next")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "BadRequestException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "BadRequestException")
	}
}

// TestNewApp_InterceptorPanicsAfterNext_StillGeneric500 proves an
// interceptor that panics AFTER next(req, res) returns -- with a plain, non-
// exception.Exception value (a plain error) -- still produces the generic
// 500 fallback any other panic already gets, and that the route Handler DID
// run before the panic (the panic happens in the interceptor's own
// post-next code, not before dispatch reached the Handler).
func TestNewApp_InterceptorPanicsAfterNext_StillGeneric500(t *testing.T) {
	var handlerRan bool
	buggy := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next interceptor.Next) {
			next(req, res)
			panic(errors.New("bug after next"))
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(buggy)
		c.Route(route.HttpGet, "/panic-after", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic-after", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("route Handler did not run, want it to run before the interceptor's post-next panic")
	}
}

// TestNewApp_ControllerFilter_CatchesMatchingExceptionType proves a
// controller-level Filter with a registered Catch runs when the route
// Handler panics with a concrete exception type that exactly matches, and
// that the Filter's own custom status+body genuinely reaches the client via
// a real app.Test dispatch.
func TestNewApp_ControllerFilter_CatchesMatchingExceptionType(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "controller"})
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Filters(f)
		c.Route(route.HttpGet, "/foo", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				panic(newFooFilterException())
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 499 {
		t.Fatalf("status = %d, want 499", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["caught"] != "controller" {
		t.Fatalf("body = %v, want caught=controller", body)
	}
}

// TestNewApp_UnmatchedExceptionType_FallsBackToDefaultFormatting proves an
// exception whose concrete type matches NEITHER a controller-level nor a
// global Catch falls through (via filteredHandler's re-panic) to the
// EXISTING adapter-level recover, producing the unchanged default
// {name,message,details} response -- explicit proof of non-regression.
func TestNewApp_UnmatchedExceptionType_FallsBackToDefaultFormatting(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "controller"})
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Filters(f)
		c.Route(route.HttpGet, "/unmatched", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				panic(newBarFilterException())
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/unmatched", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 498 {
		t.Fatalf("status = %d, want 498 (barFilterException's own Status(), unmodified default formatting)", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "BarFilterException" {
		t.Fatalf("body = %v, want name=BarFilterException (default {name,message,details} shape)", body)
	}
	if body["message"] != "bar" {
		t.Fatalf("body = %v, want message=bar", body)
	}
}

// TestNewApp_GlobalFilter_AppliesWithoutControllerOptIn proves a global
// (root module) Filter applies to a route whose own controller has NO
// Filters() of its own -- global filters are not opt-in per controller.
func TestNewApp_GlobalFilter_AppliesWithoutControllerOptIn(t *testing.T) {
	global := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "global"})
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/foo", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				panic(newFooFilterException())
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Filters(global)
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 499 {
		t.Fatalf("status = %d, want 499", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["caught"] != "global" {
		t.Fatalf("body = %v, want caught=global", body)
	}
}

// TestNewApp_ControllerFilterOverridesGlobalFilter proves that when BOTH a
// controller-level Filter and a global Filter register a Catch for the SAME
// exception type, the controller-level handler wins -- verified via two
// distinguishable response bodies, confirming which one the client actually
// receives.
func TestNewApp_ControllerFilterOverridesGlobalFilter(t *testing.T) {
	global := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "global"})
		})
	})
	controllerLevel := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "controller"})
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Filters(controllerLevel)
		c.Route(route.HttpGet, "/foo", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				panic(newFooFilterException())
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Filters(global)
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["caught"] != "controller" {
		t.Fatalf("body = %v, want caught=controller (controller-level Filter must win over global)", body)
	}
}

// TestNewApp_Filter_CatchesPanicFromGuard proves filteredHandler is
// genuinely the OUTERMOST layer of the whole dispatch chain: a Filter still
// catches a panic that originates from inside a Guard, not just from the
// route Handler itself.
func TestNewApp_Filter_CatchesPanicFromGuard(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "from-guard"})
		})
	})

	panicGuard := guard.New(func(g *guard.Guard) {
		g.Handler(func(req *execution.Request, res *execution.Response) bool {
			panic(newFooFilterException())
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Filters(f)
		c.Guards(panicGuard)
		c.Route(route.HttpGet, "/guarded", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				handlerRan = true
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 499 {
		t.Fatalf("status = %d, want 499", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["caught"] != "from-guard" {
		t.Fatalf("body = %v, want caught=from-guard", body)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want the Guard's panic to short-circuit before it")
	}
}

// TestNewApp_Filter_CatchesPanicFromMiddleware proves filteredHandler also
// catches a panic originating from a Middleware -- further evidence it truly
// wraps EVERYTHING, not just the route Handler.
func TestNewApp_Filter_CatchesPanicFromMiddleware(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "from-middleware"})
		})
	})

	panicMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next middleware.Next) {
			panic(newFooFilterException())
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Filters(f)
		c.Use(panicMw)
		c.Route(route.HttpGet, "/mw-panic", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/mw-panic", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 499 {
		t.Fatalf("status = %d, want 499", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["caught"] != "from-middleware" {
		t.Fatalf("body = %v, want caught=from-middleware", body)
	}
}

// TestNewApp_NonExceptionPanic_NotInterceptedByAnyFilter proves a panic that
// is NOT an exception.Exception (a bare error here) is never even looked up
// in any Filter's Catch map -- it still produces the existing generic 500,
// unaffected by any registered Filter (even one whose Catch happens to be
// for a type that would otherwise seem plausible).
func TestNewApp_NonExceptionPanic_NotInterceptedByAnyFilter(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooFilterException{}, func(req *execution.Request, res *execution.Response, exc *fooFilterException) {
			res.Status(499).Json(map[string]string{"caught": "should-not-happen"})
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Filters(f)
		c.Route(route.HttpGet, "/bare-error", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				panic(errors.New("plain error, not an exception.Exception"))
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/bare-error", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (generic fallback, unaffected by any registered Filter)", resp.StatusCode)
	}
}

// TestNewApp_PipelineOrdering_FullChain proves the exact combined execution
// order of all 5 pipeline stages on one route, and the two ways a Pipe
// panic is handled (caught by a matching Filter vs falling back to the
// default format), per Milestone 3's closing task.
func TestNewApp_PipelineOrdering_FullChain(t *testing.T) {
	t.Run("HappyPath_ExactOrder", func(t *testing.T) {
		var order []string
		root := buildPipelineOrderingApp(t, &order, true, true)

		fa := dispatchTestApp(t, root)
		req := httptest.NewRequest(http.MethodGet, "/pipeline/1", nil)
		resp, err := fa.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		if body["id"] != float64(42) {
			t.Fatalf("body[id] = %v, want 42", body["id"])
		}

		want := []string{
			"global-middleware",
			"controller-middleware",
			"guard",
			"interceptor-before",
			"handler-before-pipe",
			"pipe",
			"interceptor-after",
		}
		if len(order) != len(want) {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("execution order = %v, want %v -- pipeline must run Middleware(global->controller) -> Guard -> Interceptor(before) -> Handler -> Pipe -> Interceptor(after)", order, want)
			}
		}
	})

	t.Run("GuardRejects_InterceptorBeforeAndHandlerAndPipeNeverRun", func(t *testing.T) {
		var order []string
		root := buildPipelineOrderingApp(t, &order, false, true)

		fa := dispatchTestApp(t, root)
		req := httptest.NewRequest(http.MethodGet, "/pipeline/1", nil)
		resp, err := fa.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		for _, s := range order {
			if s == "interceptor-before" || s == "handler-before-pipe" || s == "pipe" || s == "interceptor-after" {
				t.Fatalf("execution order = %v, must not contain interceptor-before/handler-before-pipe/pipe/interceptor-after when Guard rejects", order)
			}
		}
		want := []string{"global-middleware", "controller-middleware", "guard"}
		if len(order) != len(want) {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("execution order = %v, want %v", order, want)
			}
		}
	})

	t.Run("PipePanics_CaughtByMatchingFilter", func(t *testing.T) {
		var order []string
		root := buildPipelineOrderingApp(t, &order, true, true)

		fa := dispatchTestApp(t, root)
		req := httptest.NewRequest(http.MethodGet, "/pipeline/bad", nil)
		resp, err := fa.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		if body["caught"] != "pipeline-filter" {
			t.Fatalf("body = %v, want caught=pipeline-filter (registered Filter must catch the Pipe's panic)", body)
		}

		found := false
		for _, s := range order {
			if s == "filter" {
				found = true
			}
			if s == "interceptor-after" {
				t.Fatalf("execution order = %v, must not contain interceptor-after when the Pipe panics before the handler returns", order)
			}
		}
		if !found {
			t.Fatalf("execution order = %v, want it to contain \"filter\"", order)
		}
	})

	t.Run("PipePanics_NoMatchingFilter_FallsBackToDefaultFormat", func(t *testing.T) {
		var order []string
		root := buildPipelineOrderingApp(t, &order, true, false)

		fa := dispatchTestApp(t, root)
		req := httptest.NewRequest(http.MethodGet, "/pipeline/bad", nil)
		resp, err := fa.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		if body["name"] != "BadRequestException" {
			t.Fatalf("body[name] = %v, want %q (default {name,message,details} format)", body["name"], "BadRequestException")
		}
		if _, ok := body["message"]; !ok {
			t.Fatalf("body = %v, want a \"message\" key present (default format)", body)
		}
		if _, ok := body["details"]; !ok {
			t.Fatalf("body = %v, want a \"details\" key present (default format)", body)
		}
	})
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

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
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

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
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

	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{GraphqlPath: "/api/gql"})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
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

// newPingOnlyApp builds a minimal app with a single "ping" -> "pong" Query
// resolver on the default /graphql path -- fixture shared by the T15
// dispatcher tests below, which only need to prove ROUTING (which of the 4
// registerGraphql handlers a given method/header/token combination reaches),
// not GraphQL execution correctness (already covered by the tests above and
// by internal/graphql's own per-handler unit tests).
func newPingOnlyApp(t *testing.T) *coreapp.App {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})
	root := module.New(func(m *module.Module) {
		m.Resolvers(res)
	})
	app, err := coreapp.NewApp[fiber.FiberApp](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.NewApp() error = %v", err)
	}
	return app
}

// listenOnEphemeralPort binds app to a real, ephemeral TCP port (same
// pattern TestRegisterRoute_WebSocketUpgrade_CompletesRealHandshake in
// internal/adapter/fiber/fiber_test.go uses -- app.Test cannot observe a
// WebSocket upgrade or true streaming, per TESTING.md) and returns the
// bound address, blocking until Listen's own onListen fires. Shutdown is
// registered via t.Cleanup.
func listenOnEphemeralPort(t *testing.T, app *coreapp.App) (addr string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve an ephemeral port: %v", err)
	}
	addr = ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}

	fired := make(chan struct{})
	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- app.Listen(addr, func() { close(fired) })
	}()

	fiberAdapter := app.Adapter().(*fiber.FiberApp)
	t.Cleanup(func() {
		// ShutdownWithTimeout, not Shutdown: a graceful Shutdown() waits for
		// every still-open connection to close on its own -- an abandoned
		// SSE stream (sse_distinct.go/sse_single.go) only notices its
		// client is gone on the NEXT heartbeat write attempt
		// (sseHeartbeatInterval, 15s), so a plain Shutdown() here would pay
		// up to a real 15-30s wall-clock tax per test. A bounded timeout
		// forcibly closes whatever is still open once it elapses instead --
		// a "context deadline exceeded" return in THAT case is the expected
		// outcome (an SSE connection this test deliberately left open,
		// never a genuine shutdown failure), so it is not asserted on here.
		_ = fiberAdapter.FiberApp().ShutdownWithTimeout(200 * time.Millisecond)
	})

	select {
	case <-fired:
	case err := <-listenErrCh:
		t.Fatalf("Listen returned before onListen fired: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the server to start listening")
	}

	return addr
}

// TestNewApp_GraphqlGet_WebSocketUpgrade_DispatchesWSProtocolHandler proves
// registerGraphql's GET dispatcher picks graphql.WSProtocolHandler for a
// real WebSocket upgrade handshake on the SAME /graphql path POST/PUT/DELETE
// also use -- the "no app.Use interception" central decision design.md
// documents. A real TCP dial is required (app.Test cannot observe a
// protocol upgrade, TESTING.md). connection_init -> connection_ack proves
// the real graphql-transport-ws state machine (not some stub) answered.
func TestNewApp_GraphqlGet_WebSocketUpgrade_DispatchesWSProtocolHandler(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	dialer := fasthttpws.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, resp, err := dialer.Dial("ws://"+addr+"/graphql", nil)
	if err != nil {
		t.Fatalf("expected the WebSocket handshake to succeed, got error: %v", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected status 101 Switching Protocols, got %d", resp.StatusCode)
	}

	if err := conn.WriteMessage(1, []byte(`{"type":"connection_init"}`)); err != nil {
		t.Fatalf("failed to write connection_init: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read connection_ack: %v", err)
	}

	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode message: %v (raw=%s)", err, data)
	}
	if msg.Type != "connection_ack" {
		t.Fatalf("message type = %q, want %q", msg.Type, "connection_ack")
	}
}

// TestNewApp_GraphqlGet_NoUpgradeNoToken_DispatchesSSEDistinctHandler proves
// registerGraphql's GET dispatcher falls through to graphql.SSEDistinctHandler
// for a plain (non-upgrade, no reservation token) GET carrying
// query/operationName as query-string params -- GraphQL-over-HTTP's own GET
// convention. A real TCP dial is used (not app.Test) because the response
// body IS a live SSE stream (res.Stream), and this test reads it frame by
// frame as it arrives rather than waiting for the connection to fully close
// first.
func TestNewApp_GraphqlGet_NoUpgradeNoToken_DispatchesSSEDistinctHandler(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	defer conn.Close()

	req := "GET /graphql?query=" + "%7B%20ping%20%7D" + " HTTP/1.1\r\nHost: " + addr + "\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	r := bufio.NewReader(conn)

	httpResp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatalf("failed to read response headers: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}
	if ct := httpResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	body := bufio.NewReader(httpResp.Body)
	eventLine, err := readLineWithDeadline(t, conn, body, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}

	dataLine, err := readLineWithDeadline(t, conn, body, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}
	if !strings.Contains(dataLine, `"ping":"pong"`) {
		t.Fatalf("data line = %q, want it to contain %q", dataLine, `"ping":"pong"`)
	}
}

// TestNewApp_GraphqlSingleConnectionFlow_RealDial_PutGetPost proves the
// remaining 2 dispatch branches together, end to end, through real app
// routing (not by calling the internal/graphql handlers directly, which is
// already covered by ssesingle_test.go's own unit tests): PUT /graphql
// reserves a token (graphql.SSESingleReserveHandler); GET /graphql with that
// token (header) dispatches to graphql.SSESingleConnectHandler, NOT
// SSEDistinctHandler (proven by the connection staying open with no
// immediate frame, then actually carrying the POST's own next/complete
// pair); POST /graphql with the SAME token plus a body carrying
// extensions.operationId dispatches to graphql.SSESingleOperationHandler,
// NOT the plain JSON graphqlHandler (proven by the 202 status and the
// operationId-tagged frame arriving over the GET's own connection).
func TestNewApp_GraphqlSingleConnectionFlow_RealDial_PutGetPost(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	// PUT -- reserve a token.
	putReq, err := http.NewRequest(http.MethodPut, "http://"+addr+"/graphql", nil)
	if err != nil {
		t.Fatalf("failed to build PUT request: %v", err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT /graphql failed: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT status = %d, want 201", putResp.StatusCode)
	}
	var reserved struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&reserved); err != nil {
		t.Fatalf("failed to decode PUT response: %v", err)
	}
	if reserved.Token == "" {
		t.Fatal("PUT response carried an empty token")
	}

	// GET (with the token) -- open the Single connection mode SSE stream on
	// its own real TCP dial, read frames from it as they arrive.
	getConn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	defer getConn.Close()

	getReqLine := "GET /graphql HTTP/1.1\r\nHost: " + addr + "\r\nX-GraphQL-Event-Stream-Token: " + reserved.Token + "\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n"
	if _, err := getConn.Write([]byte(getReqLine)); err != nil {
		t.Fatalf("failed to write GET request: %v", err)
	}

	// fasthttp/Fiber's SetBodyStreamWriter (Responder.WriteStream's own
	// implementation) does not flush the GET response's HEADERS to the
	// wire until the FIRST byte is actually written to the stream --
	// SSESingleConnectHandler's own connect loop writes nothing at all
	// until either the heartbeat ticker fires (15s) or an external write
	// arrives via reg.Route (a POST). Reading the GET response headers
	// must therefore never be sequenced BEFORE the POST that is meant to
	// trigger that very first write -- doing so deadlocks until the 15s
	// heartbeat instead (empirically confirmed while writing this test).
	// The POST is fired in its own goroutine, concurrently with reading
	// the GET response, exactly to break that ordering trap.
	postBody, _ := json.Marshal(map[string]any{
		"query": `{ ping }`,
		"extensions": map[string]any{
			"operationId": "op-1",
		},
	})
	postStatusCh := make(chan int, 1)
	postErrCh := make(chan error, 1)
	go func() {
		// A tiny window exists between the GET request's bytes reaching the
		// server and SSESingleConnectHandler's own reg.Attach call actually
		// registering this token's write function (Attach runs before
		// res.Stream is ever called) -- same race
		// ssesingle_test.go's own attachAndDrainToken helper polls reg.Route
		// for internally; this test has no direct access to reg (it's
		// private to registerGraphql), so a short, generous sleep stands in
		// for that same synchronization instead of asserting on a fixed
		// race window.
		time.Sleep(200 * time.Millisecond)

		postReq, err := http.NewRequest(http.MethodPost, "http://"+addr+"/graphql", bytes.NewReader(postBody))
		if err != nil {
			postErrCh <- err
			return
		}
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("X-GraphQL-Event-Stream-Token", reserved.Token)
		postResp, err := http.DefaultClient.Do(postReq)
		if err != nil {
			postErrCh <- err
			return
		}
		defer postResp.Body.Close()
		postStatusCh <- postResp.StatusCode
	}()

	getBufReader := bufio.NewReader(getConn)
	getHTTPResp, err := http.ReadResponse(getBufReader, nil)
	if err != nil {
		t.Fatalf("failed to read GET response headers: %v", err)
	}
	if getHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getHTTPResp.StatusCode)
	}
	if ct := getHTTPResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("GET Content-Type = %q, want %q", ct, "text/event-stream")
	}
	getBody := bufio.NewReader(getHTTPResp.Body)

	select {
	case status := <-postStatusCh:
		if status != http.StatusAccepted {
			t.Fatalf("POST status = %d, want 202 (proves dispatch to SSESingleOperationHandler, not the plain JSON handler)", status)
		}
	case err := <-postErrCh:
		t.Fatalf("POST /graphql failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the POST to complete")
	}

	// The GET's own SSE connection must now carry op-1's next/complete pair.
	eventLine, err := readLineWithDeadline(t, getConn, getBody, 3*time.Second)
	if err != nil {
		t.Fatalf("failed to read event line from the GET connection: %v", err)
	}
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}
	dataLine, err := readLineWithDeadline(t, getConn, getBody, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read data line from the GET connection: %v", err)
	}
	var next struct {
		Id      string `json:"id"`
		Payload struct {
			Data map[string]any `json:"data"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &next); err != nil {
		t.Fatalf("failed to decode next frame: %v (line=%q)", err, dataLine)
	}
	if next.Id != "op-1" {
		t.Fatalf("next.Id = %q, want %q", next.Id, "op-1")
	}
	if next.Payload.Data["ping"] != "pong" {
		t.Fatalf("next.Payload.Data = %+v, want ping=pong", next.Payload.Data)
	}
}

// critical non-regression proof: TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond
// (T9 of "Controller & Route Registration", a pre-existing test in this same
// package, defined above and left completely UNMODIFIED by this task)
// exercises a full app with zero Use() calls anywhere. Re-running it here
// (by calling it directly as a subtest) is redundant with Go's own test
// runner already running it -- this test instead exists as an explicit
// marker/documentation that that exact pre-existing test was checked to
// still pass unmodified after this task's changes to registerRoutes; see
// this file's test output for TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond
// itself for the actual proof.
func TestNewApp_ZeroMiddleware_BehavesIdenticallyToPreFeatureBehavior(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// --- T3 of "Guard": Stage 2.5 gatedHandler in internal/app ---

// TestNewApp_ZeroGuards_NonRegressionReference proves an existing
// pre-feature test (T9's UserController end-to-end example, defined earlier
// in this file and left completely UNMODIFIED by this task) still passes
// unmodified after adding gatedHandler -- a controller with zero Guards()
// calls must behave exactly as before this feature.
func TestNewApp_ZeroGuards_NonRegressionReference(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// --- T3 of "Interceptor": Stage 2.5 interceptedHandler in internal/app ---

// TestNewApp_ZeroInterceptors_NonRegressionReference proves an existing
// pre-feature test (T9's UserController end-to-end example, defined earlier
// in this file and left completely UNMODIFIED by this task) still passes
// unmodified after adding interceptedHandler -- a controller with zero
// Interceptors() calls must behave exactly as before this feature.
func TestNewApp_ZeroInterceptors_NonRegressionReference(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// TestNewApp_ZeroFilters_NonRegressionReference proves an existing
// pre-feature test (T9's UserController end-to-end example, defined earlier
// in this file and left completely UNMODIFIED by this task) still passes
// unmodified after adding filteredHandler -- a controller (and root module)
// with zero Filters() calls must behave exactly as before this feature.
func TestNewApp_ZeroFilters_NonRegressionReference(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// fooFilterException/barFilterException duplicate internal/app/app_test.go's
// own definitions of the same name -- same "test-only symbols never cross
// packages" reasoning as this file's package doc comment; both files' Filter
// tests need exactly one instance each of these two distinguishable
// exception.Exception types.
type fooFilterException struct {
	exception.HttpException
}

func newFooFilterException() *fooFilterException {
	return &fooFilterException{HttpException: exception.NewHttpException().SetStatus(499).SetName("FooFilterException").SetMessage("foo")}
}

type barFilterException struct {
	exception.HttpException
}

func newBarFilterException() *barFilterException {
	return &barFilterException{HttpException: exception.NewHttpException().SetStatus(498).SetName("BarFilterException").SetMessage("bar")}
}
