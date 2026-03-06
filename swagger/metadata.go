// gonest/swagger/metadata.go
package swagger

import "github.com/gonest-dev/gonest/core"

// WithSwagger adds Swagger metadata to a route
func WithSwagger(summary, description string) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}
		route.Metadata["summary"] = summary
		route.Metadata["description"] = description
	}
}

// WithTags adds tags to route
func WithTags(tags ...string) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}
		route.Metadata["tags"] = tags
	}
}

// WithOperationID sets operation ID
func WithOperationID(id string) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}
		route.Metadata["operationId"] = id
	}
}

// WithRequestBody adds request body metadata
func WithRequestBody(description string, schema *Schema) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}
		route.Metadata["requestBody"] = &RequestBody{
			Description: description,
			Required:    true,
			Content: map[string]MediaType{
				"application/json": {Schema: schema},
			},
		}
	}
}

// WithResponse adds response metadata
func WithResponse(statusCode, description string, schema *Schema) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}

		responses, ok := route.Metadata["responses"].(map[string]Response)
		if !ok {
			responses = make(map[string]Response)
			route.Metadata["responses"] = responses
		}

		response := Response{
			Description: description,
		}

		if schema != nil {
			response.Content = map[string]MediaType{
				"application/json": {Schema: schema},
			}
		}

		responses[statusCode] = response
	}
}

// WithSecurity adds security requirement
func WithSecurity(name string, scopes ...string) func(*core.RouteDefinition) {
	return func(route *core.RouteDefinition) {
		if route.Metadata == nil {
			route.Metadata = make(map[string]any)
		}

		security, ok := route.Metadata["security"].([]map[string][]string)
		if !ok {
			security = make([]map[string][]string, 0)
		}

		security = append(security, map[string][]string{
			name: scopes,
		})

		route.Metadata["security"] = security
	}
}

// ApplySwaggerMetadata applies all metadata to a route
func ApplySwaggerMetadata(route *core.RouteDefinition, opts ...func(*core.RouteDefinition)) {
	for _, opt := range opts {
		opt(route)
	}
}
