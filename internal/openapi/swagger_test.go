package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/adapter/fiber"
	"github.com/gonest-dev/gonest/internal/app"
	"github.com/gonest-dev/gonest/internal/module"
)

// --- renderSwaggerUIHTML (unit) --------------------------------------------

// TestRenderSwaggerUIHTML_InterpolatesOptions proves the 3 SwaggerOptions
// fields (JsonDocumentUrl/PersistAuth/DocExpansion) are genuinely
// interpolated into the rendered HTML's inline SwaggerUIBundle(...)
// initializer, not hardcoded.
func TestRenderSwaggerUIHTML_InterpolatesOptions(t *testing.T) {
	options := SwaggerOptions{
		JsonDocumentUrl: "/openapi.json",
		PersistAuth:     true,
		DocExpansion:    "none",
	}

	html := renderSwaggerUIHTML(options)

	if !strings.Contains(html, "/openapi.json") {
		t.Fatalf("expected rendered HTML to contain JsonDocumentUrl %q, got:\n%s", options.JsonDocumentUrl, html)
	}
	// html/template's JS-context escaper pads the interpolated value with
	// spaces for safety (e.g. "persistAuthorization:  true ,") -- match on
	// the field name + bare "true" token rather than an exact substring.
	if !strings.Contains(html, "persistAuthorization:") || !strings.Contains(html, "true") {
		t.Fatalf("expected rendered HTML to bake PersistAuth=true into the JS initializer, got:\n%s", html)
	}
	if !strings.Contains(html, `docExpansion:`) || !strings.Contains(html, `"none"`) {
		t.Fatalf("expected rendered HTML to bake DocExpansion=%q into the JS initializer, got:\n%s", options.DocExpansion, html)
	}
	if !strings.Contains(html, "SwaggerUIBundle") {
		t.Fatalf("expected rendered HTML to load Swagger UI's bundle, got:\n%s", html)
	}
}

// TestRenderSwaggerUIHTML_DifferentOptions_ProduceDifferentOutput proves the
// interpolation is not a fixed/hardcoded string -- changing options changes
// the rendered output.
func TestRenderSwaggerUIHTML_DifferentOptions_ProduceDifferentOutput(t *testing.T) {
	a := renderSwaggerUIHTML(SwaggerOptions{JsonDocumentUrl: "/a.json", PersistAuth: false, DocExpansion: "list"})
	b := renderSwaggerUIHTML(SwaggerOptions{JsonDocumentUrl: "/b.json", PersistAuth: true, DocExpansion: "full"})

	if a == b {
		t.Fatal("expected different SwaggerOptions to produce different rendered HTML")
	}
	if !strings.Contains(a, "/a.json") || strings.Contains(a, "/b.json") {
		t.Fatalf("expected first render to contain only /a.json, got:\n%s", a)
	}
	if !strings.Contains(b, "/b.json") || strings.Contains(b, "/a.json") {
		t.Fatalf("expected second render to contain only /b.json, got:\n%s", b)
	}
}

// --- SetupSwagger (real HTTP dispatch) -------------------------------------

// TestSetupSwagger_RealHTTPDispatch reproduces INSIGHT.md's own bootstrap
// example ordering: build a *app.App via NewApp, call SetupSwagger BEFORE
// Listen (never invoked here -- app.Test dispatches without binding a real
// port), then dispatch real HTTP requests to both registered routes via the
// underlying Fiber adapter's own app.Test mechanism (same pattern
// internal/app's own tests and this package's neighboring tests use).
func TestSetupSwagger_RealHTTPDispatch(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	a, err := app.NewApp[fiber.FiberApp](root, app.AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := a.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("a.Adapter() is not a *fiber.FiberApp: %T", a.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	doc := New("3.1.0", func(b *OpenAPI) {
		b.Title("Example API")
		b.Version("1.0.0")
	})

	options := SwaggerOptions{
		JsonDocumentUrl: "/openapi.json",
		PersistAuth:     true,
		DocExpansion:    "none",
	}

	if err := SetupSwagger(a, "/docs", doc, options); err != nil {
		t.Fatalf("SetupSwagger() error = %v", err)
	}

	t.Run("GET JsonDocumentUrl returns doc.Document() as JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("Content-Type = %q, want prefix %q", ct, "application/json")
		}

		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}

		want := doc.Document()
		if body["openapi"] != want["openapi"] {
			t.Fatalf("body[openapi] = %v, want %v", body["openapi"], want["openapi"])
		}
		info, ok := body["info"].(map[string]any)
		if !ok {
			t.Fatalf("body[info] is not a map: %v", body["info"])
		}
		if info["title"] != "Example API" {
			t.Fatalf("body[info][title] = %v, want %q", info["title"], "Example API")
		}
		if info["version"] != "1.0.0" {
			t.Fatalf("body[info][version] = %v, want %q", info["version"], "1.0.0")
		}
	})

	t.Run("GET uiPath returns HTML referencing JsonDocumentUrl/DocExpansion", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want prefix %q", ct, "text/html")
		}

		body := make([]byte, 0)
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil {
				break
			}
		}
		html := string(body)

		if !strings.Contains(html, options.JsonDocumentUrl) {
			t.Fatalf("expected HTML body to reference %q, got:\n%s", options.JsonDocumentUrl, html)
		}
		if !strings.Contains(html, "none") {
			t.Fatalf("expected HTML body to reference configured docExpansion %q, got:\n%s", options.DocExpansion, html)
		}
	})
}

// TestSetupSwagger_EmptyDocument_StillReturnsValidJSON proves spec.md's Edge
// Case: a doc with zero routes/schemas (Generate never called) still serves
// a valid, if mostly-empty, OpenAPI document shape -- no panic.
func TestSetupSwagger_EmptyDocument_StillReturnsValidJSON(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	a, err := app.NewApp[fiber.FiberApp](root, app.AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := a.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("a.Adapter() is not a *fiber.FiberApp: %T", a.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	doc := New("3.1.0", nil)

	if err := SetupSwagger(a, "/docs", doc, SwaggerOptions{JsonDocumentUrl: "/openapi.json"}); err != nil {
		t.Fatalf("SetupSwagger() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if _, ok := body["paths"]; !ok {
		t.Fatal(`expected body to contain "paths" even for an empty document`)
	}
	if _, ok := body["components"]; !ok {
		t.Fatal(`expected body to contain "components" even for an empty document`)
	}
}
