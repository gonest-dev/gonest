package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/filter"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/interceptor"
	"gonest.dev/gonest/internal/middleware"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/schema"
	"gonest.dev/gonest/internal/validate"
)

// --- "Pipeline Ordering" T1 (Milestone 3 closing task) ---
//
// This is the first test in the whole suite to combine ALL FIVE pipeline
// stages -- global Middleware, controller Middleware, Guard, Interceptor,
// param validation, Filter -- on the SAME route, mirroring INSIGHT.md's
// full "aplicando tudo no controller" UserController example. Every prior
// feature (Middleware T4, Guard T3, Interceptor T3/L-011 correction,
// Filter T4) already proved its own slice of the order in isolation; none
// combined all five. buildPipelineOrderingApp is shared by every subtest
// below so the controller/module/route shape stays identical across happy
// path, guard-rejects, and param-panics scenarios -- only the Guard's
// returned bool and the requested param differ per subtest.
//
// param-query-validation's T3 removed the whole Pipe mechanism (context.md's
// Decision 3) -- this suite originally exercised a custom Pipe
// (pipelineParamPipe) registered via Route.Param to prove a mid-pipeline
// panic (invalid param) is caught by a matching Filter. That capability now
// lives in PropertyBuilder.Custom(fn) (context.md's Decision 4): a
// validate.MustParams[T] field with Custom(fn) set can panic from inside
// fn, same as the old Pipe's Handler could, and the panic propagates through
// validate.MustParams same as any other handler-body panic would.

// pipelineIDParams is the struct-based path-param shape validated via
// validate.MustParams[T] for this suite's route -- its ID field's Custom(fn)
// parses raw as a positive int, panicking with
// exception.NewBadRequestException when raw is not a valid positive int,
// exactly like the removed pipelineParamPipe/INSIGHT.md's own
// custom-transform example did.
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
// can panic) -- the combined shape T1 exercises. order records every
// stage's execution via closures, the same shared-slice technique used by
// this package's own Middleware/Guard/Interceptor T4 tests
// (TestNewApp_MiddlewareGuardInterceptorHandler_OrderedSequence).
// guardAllows controls whether the Guard lets the request through.
// withFilter controls whether the *exception.BadRequestException Filter is
// registered on the controller at all, so the "param panics, no Filter"
// subtest can prove fallback to the default {name,message,details} format.
func buildPipelineOrderingApp(t *testing.T, order *[]string, guardAllows, withFilter bool) *module.Module {
	t.Helper()

	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *execution.Context, next middleware.Next) {
			*order = append(*order, "global-middleware")
			next(ctx)
		})
	})
	controllerMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *execution.Context, next middleware.Next) {
			*order = append(*order, "controller-middleware")
			next(ctx)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *execution.Context) bool {
			*order = append(*order, "guard")
			return guardAllows
		})
	})
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *execution.Context, next interceptor.Next) {
			*order = append(*order, "interceptor-before")
			next(ctx)
			*order = append(*order, "interceptor-after")
		})
	})

	ctrl := controller.New(func(c *controller.Controller) {
		c.Use(controllerMw)
		c.Guards(g)
		c.Interceptors(it)
		if withFilter {
			f := filter.New(func(f *filter.Filter) {
				f.Catch(&exception.BadRequestException{}, func(ctx *execution.Context, exc *exception.BadRequestException) {
					*order = append(*order, "filter")
					ctx.Status(400).Json(map[string]string{"caught": "pipeline-filter"})
				})
			})
			c.Filters(f)
		}
		c.Route(route.HttpGet, "/pipeline/:id", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				*order = append(*order, "handler-before-pipe")
				p := validate.MustParams[*pipelineIDParams](ctx, pipelineIDParamsSchema)
				*order = append(*order, "pipe")
				ctx.Json(map[string]any{"id": p.ID})
			})
		})
	})

	return module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(ctrl)
	})
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
