package validate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// --- fixtures -----------------------------------------------------------
//
// Modeled on INSIGHT.md's "exemplo para definição de metadados em
// estruturas" and "exemplo de Array e Object aninhados" sections -- not a
// full reproduction (that's T4's job), but enough to cover: a Required
// string, a Required numeric with Min/Max, a Boolean, an Array of strings
// with item Min/Max, an Array quantity Min, an Object via a nested
// *Schema ref, and an AdditionalProperties open object.

// AddressEntity mirrors INSIGHT.md's AddressEntity.
type AddressEntity struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

// UserProperties mirrors INSIGHT.md's UserEntity (trimmed) -- used across
// most tests in this file. Each test that needs its OWN schema
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
	Schema    map[string]any  `json:"schema"`
}

var addressSchema *schema.Schema

// userSchema is package-level (not init()-local) since every MustJsonBody
// call site in this file must now pass it explicitly (AD-019) instead of
// relying on a global type-keyed registry lookup.
var userSchema *schema.Schema

func init() {
	// addressSchema built via schema.New directly (not through the
	// generic NewSchema[T] root wrapper, which lives in gonest.go and
	// would create an import cycle from this internal package back to the
	// root) -- same low-level construction internal/schema's own tests use
	// (see registry_test.go's own schema.New(typ, uintptr(unsafe.Pointer(zero))) pattern).
	addr := &AddressEntity{}
	addressSchema = schema.New(reflect.TypeOf(*addr), uintptr(unsafe.Pointer(addr)))
	addressSchema.Property(&addr.Street).String().Required()
	addressSchema.Property(&addr.City).String().Required()
	addressSchema.Property(&addr.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`)

	u := &UserProperties{}
	userSchema = schema.New(reflect.TypeOf(*u), uintptr(unsafe.Pointer(u)))
	userSchema.Property(&u.Id).Integer().Required()
	userSchema.Property(&u.Name).String().Required().Min(1).Max(50)
	userSchema.Property(&u.Age).Integer().Required().Min(0).Max(130)
	userSchema.Property(&u.IsActive).Boolean().Required()
	userSchema.Property(&u.Nickname).String().Nullable()
	userSchema.Property(&u.Tags).Array().Items(func(m *schema.ArraySchema) {
		m.String().Min(1).Max(50)
	})
	userSchema.Property(&u.Addresses).Array().Items(func(m *schema.ArraySchema) {
		m.Object(addressSchema)
		m.Min(1)
	})
	userSchema.Property(&u.Address).Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
		om.Required()
	})
	userSchema.Property(&u.Schema).Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
	}).Nullable()
}

// --- test helpers ---------------------------------------------------------

func newCtx(body []byte) *execution.Request {
	req, _ := execution.New(&fakeResponder{body: body})
	req.WithSources(nil, nil, nil, execution.NewBodySource(req, nil, nil))
	return req
}

type fakeResponder struct {
	body []byte
}

func (f *fakeResponder) JSON(v any) error                      { return nil }
func (f *fakeResponder) SetStatus(code int)                    {}
func (f *fakeResponder) GetStatus() int                        { return 200 }
func (f *fakeResponder) GetMethod() string                     { return "GET" }
func (f *fakeResponder) GetPath() string                       { return "" }
func (f *fakeResponder) GetHeader(name string) string          { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string)     {}
func (f *fakeResponder) GetParam(name string) string           { return "" }
func (f *fakeResponder) RawBody() []byte                       { return f.body }
func (f *fakeResponder) Queries() map[string]string            { return nil }
func (f *fakeResponder) HTML(s string) error                   { return nil }
func (f *fakeResponder) SendString(s string) error             { return nil }
func (f *fakeResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }
func (f *fakeResponder) WriteStream(fn func(w *bufio.Writer))  {}

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
		"schema": map[string]any{"any": "thing"},
	})
	return b
}

// --- P3: happy path --------------------------------------------------------

func TestMustJsonBody_HappyPath_ReturnsPopulatedValue(t *testing.T) {
	ctx := newCtx(validBody())

	result := mustParseJSON[UserProperties](ctx, userSchema)

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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
}

func TestMustJsonBody_NullableRequiredField_NullAccepted(t *testing.T) {
	// Address is Required via Object(); Nickname is Nullable (not Required).
	// This test targets Decision/AC P4.5: Nullable + null accepted even if
	// Required. We reuse Address's own Zip field (Required, String) inside
	// AddressEntity by declaring a small dedicated fixture instead, since
	// UserProperties's Address itself is Required-Object, not a nullable
	// primitive. See nullableRequiredFixture below.
	ctx := newCtx(nullableRequiredValidBody())

	result := mustParseJSON[NullableRequiredFixture](ctx, nullableRequiredFixtureSchema)

	if result.Nickname != nil {
		t.Fatalf("expected Nickname to remain nil, got %v", *result.Nickname)
	}
}

// NullableRequiredFixture: a field that is BOTH Required() and Nullable() --
// spec.md's P4.5 / Edge Cases: null is an acceptable VALUE despite Required.
type NullableRequiredFixture struct {
	Nickname *string `json:"nickname"`
}

var nullableRequiredFixtureSchema = func() *schema.Schema {
	f := &NullableRequiredFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
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

	mustParseJSON[RequiredNotNullableFixture](ctx, requiredNotNullableFixtureSchema)
}

type RequiredNotNullableFixture struct {
	Name string `json:"name"`
}

var requiredNotNullableFixtureSchema = func() *schema.Schema {
	f := &RequiredNotNullableFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
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
		"schema": map[string]any{"whatever": 123, "nested": map[string]any{"x": true}},
	})
	ctx := newCtx(body)

	mustParseJSON[UserProperties](ctx, userSchema)
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

	mustParseJSON[UserProperties](ctx, userSchema)
}

// --- Edge cases --------------------------------------------------------

// TestMustJsonBody_MismatchedSchema_PanicsBeforeTouchingBody proves
// resolveSchema panics (AD-019) when the *schema.Schema passed in was built
// for a DIFFERENT type than T -- the modern equivalent of the old
// registry-miss panic, now a compile-time-impossible-to-omit argument
// instead of a runtime lookup failure.
func TestMustJsonBody_MismatchedSchema_PanicsBeforeTouchingBody(t *testing.T) {
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
			t.Fatal("expected a plain string panic about schema mismatch, not a BadRequestException (proves body was never touched)")
		}
	}()

	mustParseJSON[NeverRegistered](ctx, userSchema) // userSchema was built for UserProperties, not NeverRegistered
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

	mustParseJSON[UserProperties](ctx, userSchema)
}

// --- schema-sanitize-refine: Sanitize (pre-processing) ----------------------

// sanitizeCustomEntity exercises Sanitize alone and Sanitize+Custom
// combined (schema-sanitize-refine feature).
type sanitizeCustomEntity struct {
	Cpf  string `json:"cpf"`
	Code string `json:"code"`
}

var sanitizeCustomSchema *schema.Schema

func init() {
	e := &sanitizeCustomEntity{}
	sanitizeCustomSchema = schema.New(reflect.TypeOf(*e), uintptr(unsafe.Pointer(e)))
	sanitizeCustomSchema.Property(&e.Cpf).String().
		Min(11).Max(11).Pattern(`^\d{11}$`).
		Sanitize(func(raw any) any {
			s, _ := raw.(string)
			return strings.TrimSpace(s)
		}).
		Required()
	sanitizeCustomSchema.Property(&e.Code).String().
		Sanitize(func(raw any) any {
			s, _ := raw.(string)
			return strings.ToUpper(s)
		}).
		Custom(func(raw any) (any, error) {
			s, _ := raw.(string)
			if s != "V1" {
				return nil, fmt.Errorf("expected sanitized value %q, got %q", "V1", s)
			}
			return s, nil
		})
}

func TestSanitize_TrimsBeforeMinMaxPattern_AcceptsPaddedValidCpf(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"cpf": "  12345678901  ", "code": "v1"})
	ctx := newCtx(body)

	result := mustParseJSON[sanitizeCustomEntity](ctx, sanitizeCustomSchema)

	if result.Cpf != "12345678901" {
		t.Fatalf("Cpf = %q, want %q (trimmed)", result.Cpf, "12345678901")
	}
}

func TestSanitize_TrimsBeforeMinMaxPattern_RejectsPaddedTooShortValue(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"cpf": "  123  ", "code": "v1"})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "cpf") {
			t.Fatalf("expected a violation on 'cpf' (too short after trim), got %+v", vs)
		}
	}()

	mustParseJSON[sanitizeCustomEntity](ctx, sanitizeCustomSchema)
}

func TestSanitize_ComposesWithCustom_CustomReceivesSanitizedValue(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"cpf": "12345678901", "code": "v1"})
	ctx := newCtx(body)

	result := mustParseJSON[sanitizeCustomEntity](ctx, sanitizeCustomSchema)

	if result.Code != "V1" {
		t.Fatalf("Code = %q, want %q (Custom received the sanitized/uppercased value)", result.Code, "V1")
	}
}

// --- schema-sanitize-refine: Refine (cross-field post-processing) ----------

// refineEntity exercises Refine/OwnRefines in an end-to-end ParseInto
// dispatch (schema-sanitize-refine feature) -- password/confirmPassword,
// spec.md's own API Sketch example.
type refineEntity struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

var refineSchema *schema.Schema

func init() {
	e := &refineEntity{}
	refineSchema = schema.New(reflect.TypeOf(*e), uintptr(unsafe.Pointer(e)))
	refineSchema.Property(&e.Password).String().Min(8).Required()
	refineSchema.Property(&e.ConfirmPassword).String().Min(8).Required()

	refineSchema.Refine(func(dst any) (string, error) {
		d := dst.(*refineEntity)
		if d.Password != d.ConfirmPassword {
			return "confirmPassword", fmt.Errorf("must match password")
		}
		return "", nil
	})
}

func TestRefine_MatchingFields_Passes(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"password": "hunter22", "confirmPassword": "hunter22"})
	ctx := newCtx(body)

	result := mustParseJSON[refineEntity](ctx, refineSchema)

	if result.Password != "hunter22" || result.ConfirmPassword != "hunter22" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRefine_MismatchedFields_RecordsViolationOnNamedField(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"password": "hunter22", "confirmPassword": "different"})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "confirmPassword") {
			t.Fatalf("expected a violation on 'confirmPassword', got %+v", vs)
		}
	}()

	mustParseJSON[refineEntity](ctx, refineSchema)
}

func TestRefine_NeverRunsWhenIndividualFieldValidationAlreadyFailed(t *testing.T) {
	// Password below Min(8) -- individual field validation must fail BEFORE
	// Refine ever runs. ConfirmPassword is individually valid (>=8 chars)
	// but deliberately MISMATCHED -- if Refine incorrectly ran anyway, it
	// would add its own "confirmPassword" violation; asserting it does NOT
	// appear proves Refine never ran.
	body, _ := json.Marshal(map[string]any{"password": "short", "confirmPassword": "longenough1"})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "password") {
			t.Fatalf("expected a violation on 'password' (Min(8)), got %+v", vs)
		}
		if hasFieldViolation(vs, "confirmPassword") {
			t.Fatalf("Refine's own violation must NOT appear when field validation already failed, got %+v", vs)
		}
	}()

	mustParseJSON[refineEntity](ctx, refineSchema)
}

// multiRefineEntity proves multiple Refine registrations all run
// (collect-all, D5) even when more than one fails.
type multiRefineEntity struct {
	A string `json:"a"`
	B string `json:"b"`
	C string `json:"c"`
}

var multiRefineSchema *schema.Schema

func init() {
	e := &multiRefineEntity{}
	multiRefineSchema = schema.New(reflect.TypeOf(*e), uintptr(unsafe.Pointer(e)))
	multiRefineSchema.Property(&e.A).String()
	multiRefineSchema.Property(&e.B).String()
	multiRefineSchema.Property(&e.C).String()

	multiRefineSchema.Refine(func(dst any) (string, error) {
		d := dst.(*multiRefineEntity)
		if d.A != d.B {
			return "b", fmt.Errorf("must match a")
		}
		return "", nil
	})
	multiRefineSchema.Refine(func(dst any) (string, error) {
		d := dst.(*multiRefineEntity)
		if d.A != d.C {
			return "c", fmt.Errorf("must match a")
		}
		return "", nil
	})
}

func TestRefine_MultipleRegistered_AllRunAndCollectViolations(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"a": "x", "b": "y", "c": "z"})
	ctx := newCtx(body)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "b") || !hasFieldViolation(vs, "c") {
			t.Fatalf("expected violations on BOTH 'b' and 'c' (collect-all), got %+v", vs)
		}
		if len(vs) != 2 {
			t.Fatalf("expected exactly 2 violations, got %d: %+v", len(vs), vs)
		}
	}()

	mustParseJSON[multiRefineEntity](ctx, multiRefineSchema)
}

// --- schema-value-support: Value-schema (no struct) -------------------------

// cpfSchema reproduces spec.md's own API Sketch example (schema-value-
// support feature) -- a standalone string value, no struct wrapping it.
// Registered once here (package-level, same precedent as userSchema/
// addressSchema above) since schema.Register panics on duplicate
// registration for the same reflect.Type across this file's many Test
// functions.
var cpfSchema *schema.Schema

func init() {
	cpfSchema, _ = schema.NewValue(reflect.TypeOf(""))
	cpfSchema.ValueProperty().String().Min(11).Max(11).Pattern(`^\d{11}$`).Required()
}

func TestMustJsonBody_ValueSchema_HappyPath_ReturnsPopulatedValue(t *testing.T) {
	ctx := newCtx([]byte(`"12345678901"`))

	result := mustParseJSON[string](ctx, cpfSchema)

	if result != "12345678901" {
		t.Fatalf("mustParseJSON[string](cpfSchema) = %q, want %q", result, "12345678901")
	}
}

func TestMustJsonBody_ValueSchema_PatternViolation_RecordsViolation(t *testing.T) {
	ctx := newCtx([]byte(`"not-a-cpf"`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if len(vs) == 0 {
			t.Fatal("expected at least one violation for a Pattern mismatch")
		}
	}()

	mustParseJSON[string](ctx, cpfSchema)
}

func TestMustJsonBody_ValueSchema_MismatchedSchema_Panics(t *testing.T) {
	// int64Schema was built for int64, cpfSchema (string) is passed here on
	// purpose -- resolveSchema's mismatch panic (Kind-agnostic reflect.Type
	// comparison) must still fire for a Value-schema, same as it already
	// does for a struct-shaped one (TestMustJsonBody_MismatchedSchema_
	// PanicsBeforeTouchingBody above).
	ctx := newCtx([]byte(`42`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		if _, ok := r.(*exception.BadRequestException); ok {
			t.Fatal("expected a plain string panic about schema mismatch, not a BadRequestException")
		}
	}()

	mustParseJSON[int64](ctx, cpfSchema) // cpfSchema was built for string, not int64
}

// --- real HTTP dispatch (L-012 precedent) -----------------------------------

func TestMustJsonBody_RealHTTPDispatch_HappyPath(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) (err error) {
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		result := mustParseJSON[UserProperties](ctx, userSchema)
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
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		mustParseJSON[UserProperties](ctx, userSchema)
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
		ctx, _ := execution.New(&httpFiberResponder{c: c})
		ctx.WithSources(nil, nil, nil, execution.NewBodySource(ctx, nil, nil))
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
		mustParseJSON[UserProperties](ctx, userSchema)
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
func (r *httpFiberResponder) GetStatus() int                    { return r.c.Response().StatusCode() }
func (r *httpFiberResponder) GetMethod() string                 { return r.c.Method() }
func (r *httpFiberResponder) GetPath() string                   { return r.c.Path() }
func (r *httpFiberResponder) GetHeader(name string) string      { return r.c.Get(name) }
func (r *httpFiberResponder) SetHeaderValue(name, value string) { r.c.Set(name, value) }
func (r *httpFiberResponder) GetParam(name string) string       { return r.c.Params(name) }
func (r *httpFiberResponder) RawBody() []byte                   { return r.c.Body() }
func (r *httpFiberResponder) Queries() map[string]string        { return r.c.Queries() }
func (r *httpFiberResponder) HTML(s string) error {
	r.c.Type("html")
	return r.c.SendString(s)
}
func (r *httpFiberResponder) SendString(s string) error { return r.c.SendString(s) }
func (r *httpFiberResponder) BodyStream() (io.Reader, string, bool) {
	return nil, "", false
}
func (r *httpFiberResponder) WriteStream(fn func(w *bufio.Writer)) {
	r.c.RequestCtx().SetBodyStreamWriter(fn)
}
