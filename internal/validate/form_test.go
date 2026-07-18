package validate

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// --- P0/P1 fixtures ---------------------------------------------------------

// CreatePostForm mirrors a typical "1 file + regular form fields" upload --
// Title validated via the SAME form:"..." tag family Params/Query/JSON each
// have their own version of (AD-022's Multipart Form Streaming feature).
type CreatePostForm struct {
	Title string `form:"title"`
}

var createPostFormSchema = func() *schema.Schema {
	f := &CreatePostForm{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Title).String().Required()
	return m
}()

// CustomFormFixture exercises Custom(fn) on a form:"..." field -- same
// raw-string-delivery convention param/query's own Custom(fn) already
// proved (spec.md's P3).
type CustomFormFixture struct {
	Code string `form:"code"`
}

var customFormFixtureSchema = func() *schema.Schema {
	f := &CustomFormFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Code).Custom(func(raw any) (any, error) {
		s, _ := raw.(string)
		return "CODE:" + s, nil
	})
	return m
}()

// --- test helpers ------------------------------------------------------

// formFakeResponder is a minimal test-only execution.Responder whose
// BodyStream() returns a real io.Reader over an in-memory multipart body --
// enough to prove the field/file DISPATCH logic (this file's own job, per
// tasks.md's T3), NOT the true-streaming claim itself (that needs a real
// HTTP round-trip, T5/T6's job).
type formFakeResponder struct {
	stream   io.Reader
	boundary string
}

func (f *formFakeResponder) JSON(v any) error                  { return nil }
func (f *formFakeResponder) SetStatus(code int)                {}
func (f *formFakeResponder) GetStatus() int                    { return 200 }
func (f *formFakeResponder) GetMethod() string                 { return "GET" }
func (f *formFakeResponder) GetPath() string                   { return "" }
func (f *formFakeResponder) GetHeader(name string) string      { return "" }
func (f *formFakeResponder) SetHeaderValue(name, value string) {}
func (f *formFakeResponder) GetParam(name string) string       { return "" }
func (f *formFakeResponder) RawBody() []byte                   { return nil }
func (f *formFakeResponder) Queries() map[string]string        { return nil }
func (f *formFakeResponder) HTML(s string) error               { return nil }
func (f *formFakeResponder) SendString(s string) error         { return nil }
func (f *formFakeResponder) BodyStream() (io.Reader, string, bool) {
	if f.stream == nil {
		return nil, "", false
	}
	return f.stream, f.boundary, true
}
func (f *formFakeResponder) WriteStream(fn func(w *bufio.Writer)) {}

// buildMultipartBody writes fields (in order) and files (name -> content)
// via the real mime/multipart.Writer, returning the built body + its
// boundary -- exercises this package's own parsing against a genuinely
// stdlib-encoded multipart body, not a hand-rolled fixture.
func buildMultipartBody(t *testing.T, fields map[string]string, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	for name, value := range fields {
		if err := mw.WriteField(name, value); err != nil {
			t.Fatalf("WriteField(%q) failed: %v", name, err)
		}
	}
	for name, content := range files {
		fw, err := mw.CreateFormFile(name, name+".txt")
		if err != nil {
			t.Fatalf("CreateFormFile(%q) failed: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("failed writing file content: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart.Writer.Close failed: %v", err)
	}
	return body, mw.Boundary()
}

func newFormCtx(body *bytes.Buffer, boundary string) *execution.Request {
	req, _ := execution.New(&formFakeResponder{stream: body, boundary: boundary})
	return req
}

// --- unit tests --------------------------------------------------------

func TestParseFormBody_HappyPath_FieldAndFile(t *testing.T) {
	body, boundary := buildMultipartBody(t,
		map[string]string{"title": "Hello World"},
		map[string]string{"attachment": "file contents here"},
	)
	ctx := newFormCtx(body, boundary)

	var gotFilename string
	var gotContent []byte
	result, err := parseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		gotFilename = f.Filename()
		b, readErr := io.ReadAll(f.Reader())
		if readErr != nil {
			return readErr
		}
		gotContent = b
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Title != "Hello World" {
		t.Fatalf("Title = %q, want %q", result.Title, "Hello World")
	}
	if gotFilename != "attachment.txt" {
		t.Fatalf("onFile's Filename() = %q, want %q", gotFilename, "attachment.txt")
	}
	if string(gotContent) != "file contents here" {
		t.Fatalf("onFile's Reader() content = %q, want %q", string(gotContent), "file contents here")
	}
}

func TestParseFormBody_MissingRequiredField_ReturnsViolation(t *testing.T) {
	body, boundary := buildMultipartBody(t, map[string]string{}, map[string]string{})
	ctx := newFormCtx(body, boundary)

	_, err := parseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		t.Fatal("onFile should not be called -- no file part in this body")
		return nil
	})

	exc := expectBadRequest(t, err)
	vs := violationsOf(t, exc)
	if !hasFieldViolation(vs, "title") {
		t.Fatalf("expected a violation for field %q, got %+v", "title", vs)
	}
}

func TestParseFormBody_OnFileError_AbortsWithBadRequest(t *testing.T) {
	body, boundary := buildMultipartBody(t,
		map[string]string{"title": "Hello World"},
		map[string]string{"attachment": "whatever"},
	)
	ctx := newFormCtx(body, boundary)

	_, err := parseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		return errors.New("file too large")
	})

	exc := expectBadRequest(t, err)
	vs := violationsOf(t, exc)
	if !hasFieldViolation(vs, "attachment") {
		t.Fatalf("expected a violation for field %q, got %+v", "attachment", vs)
	}
	found := false
	for _, v := range vs {
		if v.Message == "file too large" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected onFile's error message reachable in violations, got %+v", vs)
	}
}

func TestParseFormBody_MalformedMultipartBody_ReturnsOneViolation(t *testing.T) {
	// A body that is NOT valid multipart at all for the given boundary.
	ctx := newFormCtx(bytes.NewBufferString("not a real multipart body"), "some-boundary")

	_, err := parseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		t.Fatal("onFile should not be called for a malformed body")
		return nil
	})

	exc := expectBadRequest(t, err)
	vs := violationsOf(t, exc)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a malformed multipart body, got %d: %+v", len(vs), vs)
	}
}

func TestParseFormBody_CustomFunc_ReceivesRawString_NotCoerced(t *testing.T) {
	body, boundary := buildMultipartBody(t, map[string]string{"code": "abc"}, map[string]string{})
	ctx := newFormCtx(body, boundary)

	result, err := parseForm[CustomFormFixture](ctx, customFormFixtureSchema, func(f *execution.FormFile) error {
		t.Fatal("onFile should not be called -- no file part in this body")
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Code != "CODE:abc" {
		t.Fatalf("expected Custom-decoded Code %q, got %q", "CODE:abc", result.Code)
	}
}

func TestMustFormBody_PanicsOnError(t *testing.T) {
	body, boundary := buildMultipartBody(t, map[string]string{}, map[string]string{})
	ctx := newFormCtx(body, boundary)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		expectBadRequest(t, r)
	}()

	mustParseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		return nil
	})
}

func TestParseFormBody_StreamUnavailable_Panics(t *testing.T) {
	ctx, _ := execution.New(&formFakeResponder{}) // stream == nil -> BodyStream() reports ok=false

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

	parseForm[CreatePostForm](ctx, createPostFormSchema, func(f *execution.FormFile) error {
		t.Fatal("onFile should not be called -- stream never became available")
		return nil
	})
}
