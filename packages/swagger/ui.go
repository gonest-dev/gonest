// gonest/swagger/ui.go
package swagger

import (
	"encoding/json"
	"html/template"
	"strings"
)

// UIConfig configures Swagger UI
type UIConfig struct {
	Title     string
	SpecURL   string
	CustomCSS string
	CustomJS  string
}

// GenerateSwaggerUI generates Swagger UI HTML
func GenerateSwaggerUI(config *UIConfig) string {
	if config.Title == "" {
		config.Title = "API Documentation"
	}
	if config.SpecURL == "" {
		config.SpecURL = "/api-docs/swagger.json"
	}

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; padding: 0; }
        {{.CustomCSS}}
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "{{.SpecURL}}",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
        {{.CustomJS}}
    </script>
</body>
</html>`

	t := template.Must(template.New("swagger").Parse(tmpl))
	var result string

	// Execute template
	_ = t
	result = strings.ReplaceAll(tmpl, "{{.Title}}", config.Title)
	result = strings.ReplaceAll(result, "{{.CustomCSS}}", config.CustomCSS)
	result = strings.ReplaceAll(result, "{{.SpecURL}}", config.SpecURL)
	result = strings.ReplaceAll(result, "{{.CustomJS}}", config.CustomJS)

	return result
}

// ServeSwaggerJSON returns JSON representation of OpenAPI document
func ServeSwaggerJSON(doc *OpenAPIDocument) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}


