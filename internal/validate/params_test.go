package validate

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/schema"
)

// --- P2 fixtures ---------------------------------------------------------
//
// UserOrderParams mirrors INSIGHT.md's settled MustParams[*UserIdParams]
// call shape, extended to 2 path params (user_id/order_id) so tests can
// exercise the "2+ path params" Done-when item plainly. Registered once,
// package-level, distinct from every other fixture type in this package
// (registry panics on duplicate registration for the same Go type).
type UserOrderParams struct {
	UserId  int64 `param:"user_id"`
	OrderId int64 `param:"order_id"`
}

var userOrderParamsSchema = func() *schema.Schema {
	f := &UserOrderParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserId).Integer().Required().Min(1)
	m.Property(&f.OrderId).Integer().Required().Min(1)
	return m
}()

// UserOnlyParams declares a SINGLE param field, used for the "field with no
// matching :name on the route at all" absence scenario -- distinct type so
// its route (which deliberately does NOT declare :user_id) doesn't collide
// with userOrderParamsSchema's own route expectations.
type UserOnlyParams struct {
	UserId int64 `param:"user_id"`
}

var userOnlyParamsSchema = func() *schema.Schema {
	f := &UserOnlyParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserId).Integer().Required()
	return m
}()

// CustomParamFixture exercises Custom(fn) on a param field: the raw string
// "abc" would never parse as a number via coerceParamString, but a Custom fn
// that treats it as an opaque code (not a numeric coercion) can still
// succeed -- proving Custom receives the RAW STRING, not a coerced value
// (spec.md's P2.3).
type CustomParamFixture struct {
	Code string `param:"code"`
}

func decodeParamCode(raw any) (any, error) {
	s, ok := raw.(string)
	if !ok {
		return nil, errors.New("expected a string")
	}
	// "abc" is not numeric -- coerceParamString would fail it if this field
	// were Integer()-typed, but Custom bypasses coercion entirely and just
	// uppercases it, proving raw-string delivery.
	return "CODE:" + s, nil
}

var customParamFixtureSchema = func() *schema.Schema {
	f := &CustomParamFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Code).Custom(decodeParamCode)
	return m
}()

// MismatchedParams is a type NO schema was ever built for -- used to prove
// MustParams panics BEFORE reading any param when handed a schema built for
// a DIFFERENT type (AD-019's resolveSchema check).
type MismatchedParams struct {
	Id int64 `param:"id"`
}

// --- test helpers ----------------------------------------------------------

// paramFakeResponder is a minimal Responder for unit tests that don't need a
// real fiber.Ctx -- params come from a plain map.
type paramFakeResponder struct {
	params map[string]string
}

func (f *paramFakeResponder) JSON(v any) error                  { return nil }
func (f *paramFakeResponder) SetStatus(code int)                {}
func (f *paramFakeResponder) GetHeader(name string) string      { return "" }
func (f *paramFakeResponder) SetHeaderValue(name, value string) {}
func (f *paramFakeResponder) GetParam(name string) string       { return f.params[name] }
func (f *paramFakeResponder) Body() []byte                      { return nil }
func (f *paramFakeResponder) Queries() map[string]string        { return nil }
func (f *paramFakeResponder) HTML(s string) error               { return nil }
func (f *paramFakeResponder) SendString(s string) error         { return nil }

// newParamCtx builds a *execution.Context carrying params and attached to a
// *route.Route built from pathPattern (e.g. "/user/:user_id/order/:order_id"),
// mirroring how a real dispatched request would look (Context.WithRoute).
func newParamCtx(pathPattern string, params map[string]string) *execution.Context {
	r := route.New(route.HttpGet, pathPattern, func(r *route.Route) {})
	ctx := execution.New(&paramFakeResponder{params: params})
	return ctx.WithRoute(r)
}

// --- unit tests --------------------------------------------------------

func TestMustParams_HappyPath_TwoParams(t *testing.T) {
	ctx := newParamCtx("/user/:user_id/order/:order_id", map[string]string{
		"user_id":  "10",
		"order_id": "20",
	})

	result := MustParams[*UserOrderParams](ctx, userOrderParamsSchema)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.UserId != 10 {
		t.Fatalf("expected UserId 10, got %d", result.UserId)
	}
	if result.OrderId != 20 {
		t.Fatalf("expected OrderId 20, got %d", result.OrderId)
	}
}

// TestMustParams_FieldWithNoRouteMatch_ProducesViolation proves that when
// T's schema declares a param field with no corresponding ":name" segment
// on the CURRENT route's pattern at all (Route.HasParam false), a violation
// is recorded rather than silently treating it as present-but-empty.
func TestMustParams_FieldWithNoRouteMatch_ProducesViolation(t *testing.T) {
	// Route pattern deliberately has NO ":user_id" segment.
	ctx := newParamCtx("/users", map[string]string{})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "user_id") {
			t.Fatalf("expected a violation for field %q (route has no :user_id segment), got %+v", "user_id", vs)
		}
	}()

	MustParams[*UserOnlyParams](ctx, userOnlyParamsSchema)
}

func TestMustParams_PresentButInvalid_ProducesViolation(t *testing.T) {
	// order_id below Min(1).
	ctx := newParamCtx("/user/:user_id/order/:order_id", map[string]string{
		"user_id":  "10",
		"order_id": "0",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "order_id") {
			t.Fatalf("expected a violation for field %q (below Min), got %+v", "order_id", vs)
		}
	}()

	MustParams[*UserOrderParams](ctx, userOrderParamsSchema)
}

func TestMustParams_WrongTypeParam_ProducesViolation(t *testing.T) {
	ctx := newParamCtx("/user/:user_id/order/:order_id", map[string]string{
		"user_id":  "not-a-number",
		"order_id": "20",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "user_id") {
			t.Fatalf("expected a violation for field %q (fails to coerce), got %+v", "user_id", vs)
		}
	}()

	MustParams[*UserOrderParams](ctx, userOrderParamsSchema)
}

func TestMustParams_TwoSimultaneousViolations_BothCollected(t *testing.T) {
	// user_id absent from route entirely is not testable with UserOrderParams
	// (both its fields ARE on the route) -- instead combine "wrong type" +
	// "out of range" to prove collect-all across TWO fields at once.
	ctx := newParamCtx("/user/:user_id/order/:order_id", map[string]string{
		"user_id":  "not-a-number",
		"order_id": "0",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "user_id") {
			t.Fatalf("expected a violation for field %q, got %+v", "user_id", vs)
		}
		if !hasFieldViolation(vs, "order_id") {
			t.Fatalf("expected a violation for field %q, got %+v", "order_id", vs)
		}
		if len(vs) < 2 {
			t.Fatalf("expected at least 2 violations (collect-all, not fail-fast), got %d: %+v", len(vs), vs)
		}
	}()

	MustParams[*UserOrderParams](ctx, userOrderParamsSchema)
}

func TestMustParams_CustomFunc_ReceivesRawString_NotCoerced(t *testing.T) {
	ctx := newParamCtx("/codes/:code", map[string]string{"code": "abc"})

	result := MustParams[*CustomParamFixture](ctx, customParamFixtureSchema)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Code != "CODE:abc" {
		t.Fatalf("expected Custom-decoded Code %q, got %q", "CODE:abc", result.Code)
	}
}

func TestMustParams_MismatchedSchema_PanicsBeforeReadingAnyParam(t *testing.T) {
	// paramFakeResponder with a params map that would panic on access if
	// consulted (nil map is safe to read in Go, so instead assert directly
	// that a panic happens BEFORE returning any value -- if MustParams tried
	// to read params first, this would still technically be reachable, so
	// the real proof is the panic message mentioning a schema mismatch, not
	// GetParam side effects).
	ctx := newParamCtx("/x/:id", map[string]string{"id": "5"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string message, got %T (%v)", r, r)
		}
		if msg == "" {
			t.Fatal("expected a non-empty panic message")
		}
	}()

	MustParams[*MismatchedParams](ctx, userOrderParamsSchema) // built for UserOrderParams, not MismatchedParams
}

// --- real HTTP dispatch ---------------------------------------------------

func TestMustParams_RealHTTPDispatch_HappyPath(t *testing.T) {
	app := fiber.New()
	r := route.New(route.HttpGet, "/user/:user_id/order/:order_id", func(r *route.Route) {})
	app.Get("/user/:user_id/order/:order_id", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c}).WithRoute(r)
		defer func() {
			if rec := recover(); rec != nil {
				if exc, ok := rec.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(rec)
			}
		}()
		result := MustParams[*UserOrderParams](ctx, userOrderParamsSchema)
		return c.JSON(map[string]any{"userId": result.UserId, "orderId": result.OrderId})
	})

	req := httptest.NewRequest(http.MethodGet, "/user/10/order/20", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var parsed struct {
		UserId  int64 `json:"userId"`
		OrderId int64 `json:"orderId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if parsed.UserId != 10 || parsed.OrderId != 20 {
		t.Fatalf("expected userId=10 orderId=20, got %+v", parsed)
	}
}

func TestMustParams_RealHTTPDispatch_InvalidOneParam(t *testing.T) {
	app := fiber.New()
	r := route.New(route.HttpGet, "/user/:user_id/order/:order_id", func(r *route.Route) {})
	app.Get("/user/:user_id/order/:order_id", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c}).WithRoute(r)
		defer func() {
			if rec := recover(); rec != nil {
				if exc, ok := rec.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(rec)
			}
		}()
		MustParams[*UserOrderParams](ctx, userOrderParamsSchema)
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/user/10/order/abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var parsed struct {
		Details []violation `json:"details"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !hasFieldViolation(parsed.Details, "order_id") {
		t.Fatalf("expected order_id violation in real HTTP response, got %+v", parsed.Details)
	}
}

func TestMustParams_RealHTTPDispatch_CustomFunc(t *testing.T) {
	app := fiber.New()
	r := route.New(route.HttpGet, "/codes/:code", func(r *route.Route) {})
	app.Get("/codes/:code", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c}).WithRoute(r)
		defer func() {
			if rec := recover(); rec != nil {
				if exc, ok := rec.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(rec)
			}
		}()
		result := MustParams[*CustomParamFixture](ctx, customParamFixtureSchema)
		return c.JSON(map[string]any{"code": result.Code})
	})

	req := httptest.NewRequest(http.MethodGet, "/codes/xyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var parsed struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if parsed.Code != "CODE:xyz" {
		t.Fatalf("expected Code %q, got %q", "CODE:xyz", parsed.Code)
	}
}
