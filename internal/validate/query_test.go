package validate

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// --- P3 fixtures ---------------------------------------------------------
//
// ListUsersQuery mirrors INSIGHT.md's settled MustQuery[*ListUsersQuery]
// call shape, 2 query fields so tests can exercise the "2+ query params"
// Done-when item plainly. Registered once, package-level, distinct from
// every other fixture type in this package (registry panics on duplicate
// registration for the same Go type).
type ListUsersQuery struct {
	Term string `query:"term"`
	Page int64  `query:"page"`
}

var listUsersQuerySchema = func() *schema.Schema {
	f := &ListUsersQuery{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Term).String().Required()
	m.Property(&f.Page).Integer().Required().Min(1)
	return m
}()

// CustomQueryFixture exercises Custom(fn) on a query field: the raw string
// "abc" would never parse as a number via coerceParamString, but a Custom
// fn that treats it as an opaque code (not a numeric coercion) can still
// succeed -- proving Custom receives the RAW STRING, not a coerced value
// (spec.md's P3.3).
type CustomQueryFixture struct {
	Code string `query:"code"`
}

func decodeQueryCode(raw any) (any, error) {
	s, ok := raw.(string)
	if !ok {
		return nil, errors.New("expected a string")
	}
	return "CODE:" + s, nil
}

var customQueryFixtureSchema = func() *schema.Schema {
	f := &CustomQueryFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Code).Custom(decodeQueryCode)
	return m
}()

// MismatchedQuery is a type NO schema was ever built for -- used to prove
// MustQuery panics BEFORE reading any query value when handed a schema
// built for a DIFFERENT type (AD-019's resolveSchema check).
type MismatchedQuery struct {
	Id int64 `query:"id"`
}

// --- test helpers ----------------------------------------------------------

// queryFakeResponder is a minimal Responder for unit tests that don't need
// a real fiber.Ctx -- queries come from a plain map.
type queryFakeResponder struct {
	queries map[string]string
}

func (f *queryFakeResponder) JSON(v any) error                      { return nil }
func (f *queryFakeResponder) SetStatus(code int)                    {}
func (f *queryFakeResponder) GetStatus() int                        { return 200 }
func (f *queryFakeResponder) GetMethod() string                     { return "GET" }
func (f *queryFakeResponder) GetPath() string                       { return "" }
func (f *queryFakeResponder) GetHeader(name string) string          { return "" }
func (f *queryFakeResponder) SetHeaderValue(name, value string)     {}
func (f *queryFakeResponder) GetParam(name string) string           { return "" }
func (f *queryFakeResponder) RawBody() []byte                       { return nil }
func (f *queryFakeResponder) Queries() map[string]string            { return f.queries }
func (f *queryFakeResponder) HTML(s string) error                   { return nil }
func (f *queryFakeResponder) SendString(s string) error             { return nil }
func (f *queryFakeResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }
func (f *queryFakeResponder) WriteStream(fn func(w *bufio.Writer))  {}

// newQueryCtx builds a *execution.Request carrying the given query map,
// mirroring how a real dispatched request would look.
func newQueryCtx(queries map[string]string) *execution.Request {
	req, _ := execution.New(&queryFakeResponder{queries: queries})
	return req
}

// --- unit tests --------------------------------------------------------

func TestMustQuery_HappyPath_TwoParams(t *testing.T) {
	ctx := newQueryCtx(map[string]string{
		"term": "gonest",
		"page": "2",
	})

	result := mustParseQuery[ListUsersQuery](ctx, listUsersQuerySchema)

	if result.Term != "gonest" {
		t.Fatalf("expected Term %q, got %q", "gonest", result.Term)
	}
	if result.Page != 2 {
		t.Fatalf("expected Page 2, got %d", result.Page)
	}
}

// TestMustQuery_MissingRequiredAndOutOfRange_BothCollected proves a
// Required query key that's absent AND a separate present-but-out-of-range
// value in the SAME request both surface as violations (collect-all, not
// fail-fast) -- spec.md's P3's Independent Test.
func TestMustQuery_MissingRequiredAndOutOfRange_BothCollected(t *testing.T) {
	// "term" absent entirely (Required), "page" present but below Min(1).
	ctx := newQueryCtx(map[string]string{
		"page": "0",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "term") {
			t.Fatalf("expected a violation for field %q (required, absent), got %+v", "term", vs)
		}
		if !hasFieldViolation(vs, "page") {
			t.Fatalf("expected a violation for field %q (below Min), got %+v", "page", vs)
		}
		if len(vs) < 2 {
			t.Fatalf("expected at least 2 violations (collect-all, not fail-fast), got %d: %+v", len(vs), vs)
		}
	}()

	mustParseQuery[ListUsersQuery](ctx, listUsersQuerySchema)
}

func TestMustQuery_WrongTypeParam_ProducesViolation(t *testing.T) {
	ctx := newQueryCtx(map[string]string{
		"term": "gonest",
		"page": "not-a-number",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "page") {
			t.Fatalf("expected a violation for field %q (fails to coerce), got %+v", "page", vs)
		}
	}()

	mustParseQuery[ListUsersQuery](ctx, listUsersQuerySchema)
}

func TestMustQuery_CustomFunc_ReceivesRawString_NotCoerced(t *testing.T) {
	ctx := newQueryCtx(map[string]string{"code": "abc"})

	result := mustParseQuery[CustomQueryFixture](ctx, customQueryFixtureSchema)

	if result.Code != "CODE:abc" {
		t.Fatalf("expected Custom-decoded Code %q, got %q", "CODE:abc", result.Code)
	}
}

func TestMustQuery_MismatchedSchema_PanicsBeforeReadingAnyQuery(t *testing.T) {
	ctx := newQueryCtx(map[string]string{"id": "5"})

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

	mustParseQuery[MismatchedQuery](ctx, listUsersQuerySchema) // built for ListUsersQuery, not MismatchedQuery
}

// --- real HTTP dispatch ---------------------------------------------------

func TestMustQuery_RealHTTPDispatch_HappyPath(t *testing.T) {
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) (err error) {
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		result := mustParseQuery[ListUsersQuery](ctx, listUsersQuerySchema)
		return c.JSON(map[string]any{"term": result.Term, "page": result.Page})
	})

	req := httptest.NewRequest(http.MethodGet, "/users?term=gonest&page=3", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var parsed struct {
		Term string `json:"term"`
		Page int64  `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if parsed.Term != "gonest" || parsed.Page != 3 {
		t.Fatalf("expected term=gonest page=3, got %+v", parsed)
	}
}

func TestMustQuery_RealHTTPDispatch_MissingRequiredAndOutOfRange(t *testing.T) {
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) (err error) {
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		mustParseQuery[ListUsersQuery](ctx, listUsersQuerySchema)
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users?page=0", nil)
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
	if !hasFieldViolation(parsed.Details, "term") {
		t.Fatalf("expected term violation in real HTTP response, got %+v", parsed.Details)
	}
	if !hasFieldViolation(parsed.Details, "page") {
		t.Fatalf("expected page violation in real HTTP response, got %+v", parsed.Details)
	}
}

func TestMustQuery_RealHTTPDispatch_CustomFunc(t *testing.T) {
	app := fiber.New()
	app.Get("/codes", func(c fiber.Ctx) (err error) {
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		result := mustParseQuery[CustomQueryFixture](ctx, customQueryFixtureSchema)
		return c.JSON(map[string]any{"code": result.Code})
	})

	req := httptest.NewRequest(http.MethodGet, "/codes?code=xyz", nil)
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
