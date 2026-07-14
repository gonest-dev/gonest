package validate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/metadata"
)

// --- fixtures -----------------------------------------------------------
//
// Modeled on INSIGHT.md's "exemplo para definição de metadados em
// estruturas" and "exemplo de Array e Object aninhados" sections -- not a
// full reproduction (that's T4's job), but enough to cover: a Required
// string, a Required numeric with Min/Max, a Boolean, an Array of strings
// with item Min/Max, an Array quantity Min, an Object via a nested
// *Metadata ref, and an AdditionalProperties open object.

// AddressEntity mirrors INSIGHT.md's AddressEntity.
type AddressEntity struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

// UserProperties mirrors INSIGHT.md's UserEntity (trimmed) -- used across
// most tests in this file. Each test that needs its OWN metadata
// registration builds a fresh anonymous struct type instead (registry
// panics on duplicate registration for the same type), so UserProperties
// itself is registered exactly ONCE, in TestMain-equivalent init below.
type UserProperties struct {
	Id        int64           `json:"id"`
	Name      string          `json:"name"`
	Age       int64           `json:"age"`
	IsActive  bool            `json:"isActive"`
	Nickname  *string         `json:"nickname"`
	Tags      []string        `json:"tags"`
	Addresses []AddressEntity `json:"addresses"`
	Address   AddressEntity   `json:"address"`
	Metadata  map[string]any  `json:"metadata"`
}

var addressMetadata *metadata.Metadata

func init() {
	// addressMetadata built via metadata.New directly (not through the
	// generic NewMetadata[T] root wrapper, which lives in gonest.go and
	// would create an import cycle from this internal package back to the
	// root) -- same low-level construction internal/metadata's own tests use
	// (see registry_test.go's own metadata.New(typ, uintptr(unsafe.Pointer(zero))) pattern).
	addr := &AddressEntity{}
	addressMetadata = metadata.New(reflect.TypeOf(*addr), uintptr(unsafe.Pointer(addr)))
	addressMetadata.Property(&addr.Street).String().Required()
	addressMetadata.Property(&addr.City).String().Required()
	addressMetadata.Property(&addr.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`)

	u := &UserProperties{}
	userMetadata := metadata.New(reflect.TypeOf(*u), uintptr(unsafe.Pointer(u)))
	userMetadata.Property(&u.Id).Integer().Required()
	userMetadata.Property(&u.Name).String().Required().Min(1).Max(50)
	userMetadata.Property(&u.Age).Integer().Required().Min(0).Max(130)
	userMetadata.Property(&u.IsActive).Boolean().Required()
	userMetadata.Property(&u.Nickname).String().Nullable()
	userMetadata.Property(&u.Tags).Array().Items(func(m *metadata.ArrayMetadata) {
		m.String().Min(1).Max(50)
	})
	userMetadata.Property(&u.Addresses).Array().Items(func(m *metadata.ArrayMetadata) {
		m.Object(addressMetadata)
		m.Min(1)
	})
	userMetadata.Property(&u.Address).Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
		om.Required()
	})
	userMetadata.Property(&u.Metadata).Object(func(om *metadata.ObjectMetadata) {
		om.AdditionalProperties()
	}).Nullable()
}

// --- test helpers ---------------------------------------------------------

func newCtx(body []byte) *execution.Context {
	return execution.New(&fakeResponder{body: body})
}

type fakeResponder struct {
	body []byte
}

func (f *fakeResponder) JSON(v any) error                  { return nil }
func (f *fakeResponder) SetStatus(code int)                {}
func (f *fakeResponder) GetHeader(name string) string      { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string) {}
func (f *fakeResponder) GetParam(name string) string       { return "" }
func (f *fakeResponder) Body() []byte                      { return f.body }
func (f *fakeResponder) Queries() map[string]string        { return nil }

func expectBadRequest(t *testing.T, r any) *exception.BadRequestException {
	t.Helper()
	exc, ok := r.(*exception.BadRequestException)
	if !ok {
		t.Fatalf("expected panic value *exception.BadRequestException, got %T (%v)", r, r)
	}
	return exc
}

func violationsOf(t *testing.T, exc *exception.BadRequestException) []violation {
	t.Helper()
	v, ok := exc.Details().([]violation)
	if !ok {
		t.Fatalf("expected Details() to be []violation, got %T", exc.Details())
	}
	return v
}

func hasFieldViolation(vs []violation, field string) bool {
	for _, v := range vs {
		if v.Field == field {
			return true
		}
	}
	return false
}

// validBody returns a fully-valid JSON body satisfying every UserProperties
// constraint declared above.
func validBody() []byte {
	b, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"nickname": nil,
		"tags":     []string{"admin", "beta"},
		"addresses": []map[string]any{
			{"street": "Rua A, 123", "city": "São Paulo", "zip": "01310-100"},
		},
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
		"metadata": map[string]any{"any": "thing"},
	})
	return b
}

// --- P3: happy path --------------------------------------------------------

func TestMustJsonBody_HappyPath_ReturnsPopulatedValue(t *testing.T) {
	ctx := newCtx(validBody())

	result := MustJsonBody[*UserProperties](ctx)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Name != "John Doe" {
		t.Fatalf("expected Name %q, got %q", "John Doe", result.Name)
	}
	if result.Age != 30 {
		t.Fatalf("expected Age 30, got %d", result.Age)
	}
	if !result.IsActive {
		t.Fatal("expected IsActive true")
	}
	if len(result.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(result.Tags))
	}
	if result.Address.City != "Rio de Janeiro" {
		t.Fatalf("expected nested Address.City to be set, got %q", result.Address.City)
	}
}

// --- P4: validation failures -----------------------------------------------

func TestMustJsonBody_MalformedJSON_PanicsWithOneViolation(t *testing.T) {
	ctx := newCtx([]byte(`{not valid json`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if len(vs) != 1 {
			t.Fatalf("expected exactly 1 violation for malformed JSON, got %d: %+v", len(vs), vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_EmptyBody_TreatedAsParseFailure(t *testing.T) {
	ctx := newCtx([]byte{})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		expectBadRequest(t, r)
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_MissingRequiredField_RecordsViolation(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		// "id" omitted -- Required
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "id") {
			t.Fatalf("expected a violation for field %q, got %+v", "id", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_OutOfRangeValue_RecordsViolation(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(999), // Max(130) violated
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "age") {
			t.Fatalf("expected a violation for field %q, got %+v", "age", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_FractionalValueOnIntegerField_RecordsViolation(t *testing.T) {
	// SPEC_DEVIATION documented on validatePrimitive: a JSON number with a
	// non-zero fractional part (e.g. 30.5) posted for an Integer()/Int32()
	// field is a type violation, not silently truncated. This proves that
	// code path (f != float64(int64(f))) actually fires.
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      30.5, // fractional -- Age is Integer()
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "age") {
			t.Fatalf("expected a violation for field %q, got %+v", "age", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_MultipleViolations_AllCollected(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		// "id" missing (required)
		"name":     "John Doe",
		"age":      int64(999), // out of range
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "id") {
			t.Fatalf("expected violation for %q among %+v", "id", vs)
		}
		if !hasFieldViolation(vs, "age") {
			t.Fatalf("expected violation for %q among %+v", "age", vs)
		}
		if len(vs) < 2 {
			t.Fatalf("expected at least 2 violations (collect-all), got %d: %+v", len(vs), vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_NullableRequiredField_NullAccepted(t *testing.T) {
	// Address is Required via Object(); Nickname is Nullable (not Required).
	// This test targets Decision/AC P4.5: Nullable + null accepted even if
	// Required. We reuse Address's own Zip field (Required, String) inside
	// AddressEntity by declaring a small dedicated fixture instead, since
	// UserProperties's Address itself is Required-Object, not a nullable
	// primitive. See nullableRequiredFixture below.
	ctx := newCtx(nullableRequiredValidBody())

	result := MustJsonBody[*NullableRequiredFixture](ctx)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Nickname != nil {
		t.Fatalf("expected Nickname to remain nil, got %v", *result.Nickname)
	}
}

// NullableRequiredFixture: a field that is BOTH Required() and Nullable() --
// spec.md's P4.5 / Edge Cases: null is an acceptable VALUE despite Required.
type NullableRequiredFixture struct {
	Nickname *string `json:"nickname"`
}

var nullableRequiredFixtureMetadata = func() *metadata.Metadata {
	f := &NullableRequiredFixture{}
	m := metadata.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Nickname).String().Required().Nullable()
	return m
}()

func nullableRequiredValidBody() []byte {
	b, _ := json.Marshal(map[string]any{"nickname": nil})
	return b
}

func TestMustJsonBody_RequiredNotNullable_NullRejected(t *testing.T) {
	// Sanity check the inverse: Required WITHOUT Nullable, null value -> violation.
	ctx := newCtx(mustNotNullableBody())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "name") {
			t.Fatalf("expected violation for %q, got %+v", "name", vs)
		}
	}()

	MustJsonBody[*RequiredNotNullableFixture](ctx)
}

type RequiredNotNullableFixture struct {
	Name string `json:"name"`
}

var requiredNotNullableFixtureMetadata = func() *metadata.Metadata {
	f := &RequiredNotNullableFixture{}
	m := metadata.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Name).String().Required()
	return m
}()

func mustNotNullableBody() []byte {
	b, _ := json.Marshal(map[string]any{"name": nil})
	return b
}

// --- P5: recursive array/object validation ---------------------------------

func TestMustJsonBody_ArrayItemViolation_IdentifiesIndex(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"tags":     []string{"ok", ""}, // second item violates Min(1)
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "tags[1]") {
			t.Fatalf("expected violation identifying index, got %+v", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_ArrayQuantityViolation_IdentifiesField(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":        int64(1),
		"name":      "John Doe",
		"age":       int64(30),
		"isActive":  true,
		"addresses": []any{}, // violates Min(1) quantity on the array itself
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "addresses") {
			t.Fatalf("expected violation for field %q (not an item), got %+v", "addresses", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_ObjectRefViolation_IdentifiesNestedPath(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "invalid-zip",
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "address.zip") {
			t.Fatalf("expected violation for nested path %q, got %+v", "address.zip", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

func TestMustJsonBody_AdditionalProperties_NoStructuralValidation(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
		"metadata": map[string]any{"whatever": 123, "nested": map[string]any{"x": true}},
	})
	ctx := newCtx(body)

	result := MustJsonBody[*UserProperties](ctx)

	if result == nil {
		t.Fatal("expected non-nil result (AdditionalProperties should not fail structural validation)")
	}
}

func TestMustJsonBody_CombinedArrayAndObjectViolations_BothReported(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"id":       int64(1),
		"name":     "John Doe",
		"age":      int64(30),
		"isActive": true,
		"tags":     []string{""}, // bad item
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "bad", // bad nested field
		},
	})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "tags[0]") {
			t.Fatalf("expected array item violation, got %+v", vs)
		}
		if !hasFieldViolation(vs, "address.zip") {
			t.Fatalf("expected nested object violation, got %+v", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

// --- Edge cases --------------------------------------------------------

func TestMustJsonBody_UnregisteredType_PanicsBeforeTouchingBody(t *testing.T) {
	type NeverRegistered struct {
		X string `json:"x"`
	}
	ctx := newCtx([]byte(`{not valid json at all`)) // deliberately malformed too

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		if _, ok := r.(*exception.BadRequestException); ok {
			t.Fatal("expected a plain string panic about missing metadata, not a BadRequestException (proves body was never touched)")
		}
	}()

	MustJsonBody[*NeverRegistered](ctx)
}

func TestMustJsonBody_NonObjectTopLevel_DegradesToAllRequiredMissing(t *testing.T) {
	ctx := newCtx([]byte(`[1,2,3]`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "id") {
			t.Fatalf("expected required field violations for non-object body, got %+v", vs)
		}
	}()

	MustJsonBody[*UserProperties](ctx)
}

// --- real HTTP dispatch (L-012 precedent) -----------------------------------

func TestMustJsonBody_RealHTTPDispatch_HappyPath(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c})
		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(r)
			}
		}()
		result := MustJsonBody[*UserProperties](ctx)
		return c.JSON(map[string]any{"name": result.Name})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMustJsonBody_RealHTTPDispatch_MultipleViolations(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c})
		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(r)
			}
		}()
		MustJsonBody[*UserProperties](ctx)
		return c.SendStatus(http.StatusOK)
	})

	body, _ := json.Marshal(map[string]any{
		// id missing, age out of range
		"name":     "John Doe",
		"age":      int64(999),
		"isActive": true,
		"address": map[string]any{
			"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

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
	if !hasFieldViolation(parsed.Details, "id") || !hasFieldViolation(parsed.Details, "age") {
		t.Fatalf("expected both id and age violations in real HTTP response, got %+v", parsed.Details)
	}
}

func TestMustJsonBody_RealHTTPDispatch_MalformedJSON(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c})
		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(r)
			}
		}()
		MustJsonBody[*UserProperties](ctx)
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader([]byte(`{bad json`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

// httpFiberResponder is a minimal Responder implementation wrapping a real
// fiber.Ctx, local to this test file (internal/adapter/fiber's own
// fiberResponder is unexported, and importing that package here purely for
// its Body() plumbing would add a needless dependency for a test-only
// concern -- route dispatch itself is not under test in this package, only
// that Context.Body() sourced from a REAL fiber.Ctx flows correctly into
// MustJsonBody, matching L-012's precedent).
type httpFiberResponder struct {
	c fiber.Ctx
}

func (r *httpFiberResponder) JSON(v any) error                  { return r.c.JSON(v) }
func (r *httpFiberResponder) SetStatus(code int)                { r.c.Status(code) }
func (r *httpFiberResponder) GetHeader(name string) string      { return r.c.Get(name) }
func (r *httpFiberResponder) SetHeaderValue(name, value string) { r.c.Set(name, value) }
func (r *httpFiberResponder) GetParam(name string) string       { return r.c.Params(name) }
func (r *httpFiberResponder) Body() []byte                      { return r.c.Body() }
func (r *httpFiberResponder) Queries() map[string]string        { return r.c.Queries() }
