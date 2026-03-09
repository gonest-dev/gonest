// gonest/swagger/generator.go
package swagger

import (
	"reflect"
	"strings"

	"github.com/gonest-dev/gonest/core/common"
)

// GenerateFromApplication generates OpenAPI document from NestApplication
func GenerateFromApplication(_ *common.NestApplication, opts ...DocumentOption) *OpenAPIDocument {
	builder := NewDocumentBuilder()

	// Apply options
	for _, opt := range opts {
		opt(builder)
	}

	// Get all routes from application
	// Note: This requires adding a method to NestApplication
	// For now, we'll work with what we have

	return builder.Build()
}

// GenerateFromRoutes generates OpenAPI document from route definitions
func GenerateFromRoutes(routes []common.RouteDefinition, opts ...DocumentOption) *OpenAPIDocument {
	builder := NewDocumentBuilder()

	// Apply options
	for _, opt := range opts {
		opt(builder)
	}

	// Process each route
	for _, route := range routes {
		operation := routeToOperation(route)
		builder.AddPath(route.Path, route.Method, operation)
	}

	return builder.Build()
}

// routeToOperation converts a RouteDefinition to an Operation
func routeToOperation(route common.RouteDefinition) *Operation {
	operation := &Operation{
		Responses:  make(map[string]Response),
		Parameters: []Parameter{},
	}

	// Extract metadata if available
	if route.Metadata != nil {
		// Summary
		if summary, ok := route.Metadata["summary"].(string); ok {
			operation.Summary = summary
		}

		// Description
		if description, ok := route.Metadata["description"].(string); ok {
			operation.Description = description
		}

		// Tags
		if tags, ok := route.Metadata["tags"].([]string); ok {
			operation.Tags = tags
		}

		// Operation ID
		if operationID, ok := route.Metadata["operationId"].(string); ok {
			operation.OperationID = operationID
		}

		// Parameters (from controller params)
		if params, ok := route.Metadata["params"].([]any); ok {
			for _, p := range params {
				if param := paramToSwaggerParam(p); param != nil {
					operation.Parameters = append(operation.Parameters, *param)
				}
			}
		}

		// Request body
		if requestBody, ok := route.Metadata["requestBody"].(*RequestBody); ok {
			operation.RequestBody = requestBody
		}

		// Responses
		if responses, ok := route.Metadata["responses"].(map[string]Response); ok {
			operation.Responses = responses
		}

		// Security
		if security, ok := route.Metadata["security"].([]map[string][]string); ok {
			operation.Security = security
		}
	}

	// Add default 200 response if none provided
	if len(operation.Responses) == 0 {
		operation.Responses["200"] = Response{
			Description: "Successful response",
		}
	}

	// Extract path parameters from route path
	operation.Parameters = append(operation.Parameters, extractPathParams(route.Path)...)

	return operation
}

// extractPathParams extracts path parameters from route path
func extractPathParams(path string) []Parameter {
	var params []Parameter
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			params = append(params, Parameter{
				Name:     paramName,
				In:       "path",
				Required: true,
				Schema: &Schema{
					Type: "string",
				},
			})
		}
	}

	return params
}

// paramToSwaggerParam converts controller param config to swagger parameter
func paramToSwaggerParam(p any) *Parameter {
	// Use reflection to extract param info
	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	param := &Parameter{
		Schema: &Schema{Type: "string"},
	}

	// Try to extract common fields
	if nameField := v.FieldByName("Name"); nameField.IsValid() {
		param.Name = nameField.String()
	}

	if typeField := v.FieldByName("Type"); typeField.IsValid() {
		typeStr := typeField.String()
		switch typeStr {
		case "query":
			param.In = "query"
		case "header":
			param.In = "header"
		case "path":
			param.In = "path"
		case "cookie":
			param.In = "cookie"
		}
	}

	if requiredField := v.FieldByName("Required"); requiredField.IsValid() {
		param.Required = requiredField.Bool()
	}

	return param
}

// DocumentOption configures document generation
type DocumentOption func(*DocumentBuilder)

// WithInfo sets document info
func WithInfo(title, description, version string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.SetInfo(title, description, version)
	}
}

// WithServer adds a server
func WithServer(url, description string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.AddServer(url, description)
	}
}

// WithTag adds a tag
func WithTag(name, description string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.AddTag(name, description)
	}
}

// WithBearerAuth adds bearer authentication
func WithBearerAuth() DocumentOption {
	return func(b *DocumentBuilder) {
		b.AddBearerAuth()
	}
}

// WithAPIKeyAuth adds API key authentication
func WithAPIKeyAuth(name, in string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.AddAPIKeyAuth(name, in)
	}
}

// WithContact sets contact information
func WithContact(name, url, email string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.SetContact(name, url, email)
	}
}

// WithLicense sets license information
func WithLicense(name, url string) DocumentOption {
	return func(b *DocumentBuilder) {
		b.SetLicense(name, url)
	}
}
