// swagger.go serves an already-built *OpenApiDocument over HTTP: a JSON
// endpoint (doc.Document(), already json.Marshal-able per generate.go) plus
// a minimal Swagger UI HTML page loading its assets from a CDN (no vendored
// static assets -- see .specs/features/swagger-ui-setup/spec.md's Out of
// Scope). Purely serving/wiring: no new generation logic lives here.
package openapi

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/gonest-dev/gonest/internal/app"
	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/route"
)

// SwaggerOptions is INSIGHT.md's exact field set for SetupSwagger's options
// argument (the "# exemplo de bootstrap completo" section's
// gonest.SwaggerOptions{JsonDocumentUrl, PersistAuth, DocExpansion} call
// shape) -- nothing more, per spec.md's Out of Scope.
type SwaggerOptions struct {
	// JsonDocumentUrl is the path SetupSwagger registers the generated
	// OpenAPI JSON document on, and the URL the served Swagger UI page's own
	// JS initializer is configured to fetch its spec from.
	JsonDocumentUrl string
	// PersistAuth maps to Swagger UI's own persistAuthorization initializer
	// option -- whether a bearer/API-key token entered in the UI survives a
	// page reload.
	PersistAuth bool
	// DocExpansion maps to Swagger UI's own docExpansion initializer option
	// (e.g. "none", "list", "full") -- how far operations/tags are expanded
	// by default when the UI first loads.
	DocExpansion string
}

// SetupSwagger registers two routes directly on app's adapter (post-
// bootstrap, no DI/Module involvement -- the same app.Adapter().
// RegisterRoute(...) mechanism internal/app's own bootstrap already uses
// internally):
//
//   - GET options.JsonDocumentUrl -- serves doc.Document() as JSON.
//   - GET uiPath -- serves an HTML page loading Swagger UI via CDN,
//     configured to fetch the JSON document from options.JsonDocumentUrl
//     with persistAuthorization/docExpansion baked into the UI's own JS
//     initializer from options.PersistAuth/options.DocExpansion.
//
// Matches INSIGHT.md's own bootstrap example call shape:
// gonest.SetupSwagger(app, config.OpenApi.UiPath, doc, gonest.SwaggerOptions{...}).
// Intended to be called AFTER NewApp (so app.Adapter() exists) and BEFORE
// app.MustListen/Listen -- both routes are reachable once the server starts,
// per spec.md AC3.
func SetupSwagger(a *app.App, uiPath string, doc *OpenApiDocument, options SwaggerOptions) error {
	adapter := a.Adapter()

	if err := adapter.RegisterRoute(route.HttpGet, options.JsonDocumentUrl, func(ctx *execution.Context) {
		ctx.Json(doc.Document()) //nolint:errcheck // best-effort write, mirrors other Handler bodies in this codebase
	}); err != nil {
		return err
	}

	html := renderSwaggerUIHTML(options)
	if err := adapter.RegisterRoute(route.HttpGet, uiPath, func(ctx *execution.Context) {
		ctx.HTML(html) //nolint:errcheck // best-effort write, mirrors other Handler bodies in this codebase
	}); err != nil {
		return err
	}

	return nil
}

// swaggerUITemplate is a minimal HTML page loading Swagger UI's bundle/CSS
// from unpkg's CDN (no vendored static assets, per spec.md's Out of Scope --
// SPEC_DEVIATION: the prompt did not pin an exact CDN URL/version, so a
// recent, widely-used swagger-ui-dist release is chosen here; any later
// feature that needs offline/vendored assets is free to swap this constant).
// The 3 SwaggerOptions values are genuinely interpolated via text/template
// (html/template, so JsonDocumentUrl/DocExpansion are HTML/JS-escaped rather
// than pasted in raw) -- not hardcoded.
const swaggerUITemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: {{.JsonDocumentUrl}},
        dom_id: '#swagger-ui',
        persistAuthorization: {{.PersistAuth}},
        docExpansion: {{.DocExpansion}},
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>
`

// renderSwaggerUIHTML builds the Swagger UI HTML page for options, with
// JsonDocumentUrl/PersistAuth/DocExpansion baked into the inline
// SwaggerUIBundle(...) initializer. Uses html/template rather than plain
// string formatting so the interpolated values are safely escaped for their
// JS-literal context.
func renderSwaggerUIHTML(options SwaggerOptions) string {
	tmpl := template.Must(template.New("swagger-ui").Parse(swaggerUITemplate))

	var sb strings.Builder
	if err := tmpl.Execute(&sb, options); err != nil {
		// html/template.Execute only fails on a template/data mismatch, which
		// swaggerUITemplate's own fixed shape against SwaggerOptions rules
		// out at compile-reachable runtime -- a panic here surfaces a real
		// programming error immediately rather than silently serving a
		// half-rendered page.
		panic(fmt.Sprintf("gonest: failed to render Swagger UI HTML: %v", err))
	}
	return sb.String()
}
