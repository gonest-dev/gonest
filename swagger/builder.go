// gonest/swagger/builder.go
package swagger

import (
	"reflect"
	"strings"
)

// DocumentBuilder helps build OpenAPI documents
type DocumentBuilder struct {
	doc *OpenAPIDocument
}

// NewDocumentBuilder creates a new document builder
func NewDocumentBuilder() *DocumentBuilder {
	return &DocumentBuilder{
		doc: &OpenAPIDocument{
			OpenAPI: OpenAPIVersion,
			Paths:   make(map[string]PathItem),
			Info: Info{
				Title:   "API",
				Version: "1.0.0",
			},
		},
	}
}

// SetInfo sets API info
func (b *DocumentBuilder) SetInfo(title, description, version string) *DocumentBuilder {
	b.doc.Info.Title = title
	b.doc.Info.Description = description
	b.doc.Info.Version = version
	return b
}

// SetContact sets contact information
func (b *DocumentBuilder) SetContact(name, url, email string) *DocumentBuilder {
	b.doc.Info.Contact = &Contact{
		Name:  name,
		URL:   url,
		Email: email,
	}
	return b
}

// SetLicense sets license information
func (b *DocumentBuilder) SetLicense(name, url string) *DocumentBuilder {
	b.doc.Info.License = &License{
		Name: name,
		URL:  url,
	}
	return b
}

// AddServer adds a server
func (b *DocumentBuilder) AddServer(url, description string) *DocumentBuilder {
	b.doc.Servers = append(b.doc.Servers, Server{
		URL:         url,
		Description: description,
	})
	return b
}

// AddTag adds a tag
func (b *DocumentBuilder) AddTag(name, description string) *DocumentBuilder {
	b.doc.Tags = append(b.doc.Tags, Tag{
		Name:        name,
		Description: description,
	})
	return b
}

// AddBearerAuth adds Bearer authentication
func (b *DocumentBuilder) AddBearerAuth() *DocumentBuilder {
	if b.doc.Components == nil {
		b.doc.Components = &Components{
			SecuritySchemes: make(map[string]SecurityScheme),
		}
	}

	b.doc.Components.SecuritySchemes["bearer"] = SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT Authorization header using the Bearer scheme",
	}

	return b
}

// AddAPIKeyAuth adds API Key authentication
func (b *DocumentBuilder) AddAPIKeyAuth(name, in string) *DocumentBuilder {
	if b.doc.Components == nil {
		b.doc.Components = &Components{
			SecuritySchemes: make(map[string]SecurityScheme),
		}
	}

	b.doc.Components.SecuritySchemes["apiKey"] = SecurityScheme{
		Type:        "apiKey",
		Name:        name,
		In:          in,
		Description: "API Key authentication",
	}

	return b
}

// AddPath adds a path with operation
func (b *DocumentBuilder) AddPath(path, method string, operation *Operation) *DocumentBuilder {
	pathItem, exists := b.doc.Paths[path]
	if !exists {
		pathItem = PathItem{}
	}

	switch strings.ToUpper(method) {
	case "GET":
		pathItem.Get = operation
	case "POST":
		pathItem.Post = operation
	case "PUT":
		pathItem.Put = operation
	case "PATCH":
		pathItem.Patch = operation
	case "DELETE":
		pathItem.Delete = operation
	case "OPTIONS":
		pathItem.Options = operation
	case "HEAD":
		pathItem.Head = operation
	}

	b.doc.Paths[path] = pathItem
	return b
}

// AddSchema adds a schema to components
func (b *DocumentBuilder) AddSchema(name string, schema *Schema) *DocumentBuilder {
	if b.doc.Components == nil {
		b.doc.Components = &Components{
			Schemas: make(map[string]*Schema),
		}
	}
	if b.doc.Components.Schemas == nil {
		b.doc.Components.Schemas = make(map[string]*Schema)
	}

	b.doc.Components.Schemas[name] = schema
	return b
}

// Build returns the final document
func (b *DocumentBuilder) Build() *OpenAPIDocument {
	return b.doc
}

// SchemaFromStruct creates a schema from a Go struct
func SchemaFromStruct(v any) *Schema {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return &Schema{Type: "object"}
	}

	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
		Required:   []string{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]

		// Get field schema
		fieldSchema := schemaFromType(field.Type)

		// Add description from comment or tag
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}

		schema.Properties[fieldName] = fieldSchema

		// Check if required
		if required := field.Tag.Get("required"); required == "true" {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema
}

func schemaFromType(t reflect.Type) *Schema {
	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		return &Schema{
			Type:  "array",
			Items: schemaFromType(t.Elem()),
		}
	case reflect.Struct:
		return SchemaFromStruct(reflect.New(t).Elem().Interface())
	case reflect.Ptr:
		return schemaFromType(t.Elem())
	default:
		return &Schema{Type: "object"}
	}
}

// NewOperation creates a new operation
func NewOperation(summary, description string) *Operation {
	return &Operation{
		Summary:     summary,
		Description: description,
		Responses:   make(map[string]Response),
		Parameters:  []Parameter{},
	}
}

// WithTag adds tags to operation
func (o *Operation) WithTag(tags ...string) *Operation {
	o.Tags = append(o.Tags, tags...)
	return o
}

// WithParameter adds a parameter
func (o *Operation) WithParameter(name, in, description string, required bool, schema *Schema) *Operation {
	o.Parameters = append(o.Parameters, Parameter{
		Name:        name,
		In:          in,
		Description: description,
		Required:    required,
		Schema:      schema,
	})
	return o
}

// WithRequestBody adds a request body
func (o *Operation) WithRequestBody(description string, required bool, schema *Schema) *Operation {
	o.RequestBody = &RequestBody{
		Description: description,
		Required:    required,
		Content: map[string]MediaType{
			"application/json": {Schema: schema},
		},
	}
	return o
}

// WithResponse adds a response
func (o *Operation) WithResponse(statusCode, description string, schema *Schema) *Operation {
	response := Response{
		Description: description,
	}

	if schema != nil {
		response.Content = map[string]MediaType{
			"application/json": {Schema: schema},
		}
	}

	o.Responses[statusCode] = response
	return o
}

// WithSecurity adds security requirement
func (o *Operation) WithSecurity(name string, scopes ...string) *Operation {
	o.Security = append(o.Security, map[string][]string{
		name: scopes,
	})
	return o
}
