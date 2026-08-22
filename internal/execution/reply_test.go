package execution

import "testing"

func TestReply_Json_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	type payload struct{ Name string }
	value := payload{Name: "gonest"}

	if err := res.Json(value); err != nil {
		t.Fatalf("Json() returned error: %v", err)
	}

	if fake.jsonValue != value {
		t.Fatalf("expected responder.JSON to receive %+v, got %+v", value, fake.jsonValue)
	}
}

func TestReply_Status_IsChainableAndSetsCode(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	returned := res.Status(201)

	if returned != res {
		t.Fatalf("expected Status() to return the same *Reply for chaining")
	}
	if fake.statusCode != 201 {
		t.Fatalf("expected responder.SetStatus(201), got %d", fake.statusCode)
	}
}

func TestReply_Status_Json_Chained(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	if err := res.Status(200).Json(map[string]string{"ok": "true"}); err != nil {
		t.Fatalf("chained Status().Json() returned error: %v", err)
	}

	if fake.statusCode != 200 {
		t.Fatalf("expected status 200, got %d", fake.statusCode)
	}
	if fake.jsonValue == nil {
		t.Fatalf("expected JSON value to be set via chained call")
	}
}

func TestReply_StatusCode_ReadsCurrentStatus(t *testing.T) {
	fake := newFakeResponder()
	fake.statusCode = 404
	_, res := New(fake)

	if got := res.StatusCode(); got != 404 {
		t.Fatalf("expected StatusCode() to return %d, got %d", 404, got)
	}
}

func TestReply_SetHeader_WritesToResponder(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	res.SetHeader("X-Custom", "value")

	if got := fake.headers["X-Custom"]; got != "value" {
		t.Fatalf("expected responder header %q, got %q", "value", got)
	}
}

// TestReply_Html_DelegatesToResponder proves Html(s) is a one-line
// delegation to the underlying Responder's own HTML method -- request-
// response-split feature, renamed from HTML (all-caps).
func TestReply_Html_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	if err := res.Html("<h1>hello</h1>"); err != nil {
		t.Fatalf("Html() returned error: %v", err)
	}

	if fake.htmlValue != "<h1>hello</h1>" {
		t.Fatalf("expected responder.HTML to receive %q, got %q", "<h1>hello</h1>", fake.htmlValue)
	}
}

// TestReply_Text_SetsPlainTextContentType proves Text(s) forces
// Content-Type: text/plain before writing -- request-response-split
// feature, replaces SendString (which set no Content-Type).
func TestReply_Text_SetsPlainTextContentType(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	if err := res.Text("OK"); err != nil {
		t.Fatalf("Text() returned error: %v", err)
	}

	if got := fake.headers["Content-Type"]; got != "text/plain" {
		t.Fatalf("expected Content-Type header %q, got %q", "text/plain", got)
	}
}

// TestReply_Redirect_DefaultsTo302 proves Redirect(url) with no status
// argument uses http.StatusFound (302) -- NestJS's own @Redirect() default,
// the reference this framework mirrors (see .specs/insight/REDIRECT.md).
func TestReply_Redirect_DefaultsTo302(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	if err := res.Redirect("https://example.com/target"); err != nil {
		t.Fatalf("Redirect() returned error: %v", err)
	}

	if got := fake.headers["Location"]; got != "https://example.com/target" {
		t.Fatalf("expected Location header %q, got %q", "https://example.com/target", got)
	}
	if fake.statusCode != 302 {
		t.Fatalf("expected default status 302, got %d", fake.statusCode)
	}
	if fake.sentString != "" {
		t.Fatalf("expected empty body, got %q", fake.sentString)
	}
}

// TestReply_Redirect_StatusOverridesDefault proves a passed status
// overrides the 302 default.
func TestReply_Redirect_StatusOverridesDefault(t *testing.T) {
	tests := []int{301, 307, 308}
	for _, status := range tests {
		fake := newFakeResponder()
		_, res := New(fake)

		if err := res.Redirect("/other", status); err != nil {
			t.Fatalf("Redirect() returned error: %v", err)
		}
		if fake.statusCode != status {
			t.Fatalf("expected status %d, got %d", status, fake.statusCode)
		}
	}
}

// TestReply_Request_ReturnsOriginatingRequest proves Reply holds a
// reference back to the *Request that originated it (context.md's Decision
// D2).
func TestReply_Request_ReturnsOriginatingRequest(t *testing.T) {
	req, res := New(newFakeResponder())

	if got := res.Request(); got != req {
		t.Fatalf("expected Request() to return the same *Request New() built, got %v want %v", got, req)
	}
}
