package exception

import (
	"reflect"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/schema"
)

// schemaShape mirrors HttpException.MarshalJSON's own output shape
// ({"name","message","details"}) -- HttpException's real fields are
// unexported, so a local mirror struct with exported json-tagged fields is
// what Schema is actually built from.
type schemaShape struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

// Schema is the canonical OpenAPI schema for any HttpException-embedding
// exception's JSON body. internal/openapi registers it by default for
// every route's undocumented (schema-less) 4xx/5xx Response entry, since
// HttpException is the framework's single default carrier for both
// built-in and dev-defined exceptions alike -- a route that never calls
// Response with its own error body schema still documents a real shape,
// not an empty description.
var Schema = newSchema(func(t *schemaShape, m *schema.Schema) {
	m.Title("HttpException")
	m.Property(&t.Name).String().Required()
	m.Property(&t.Message).String().Required()
	m.Property(&t.Details).Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
		om.Nullable()
	})
})

// newSchema is a package-local copy of the root gonest.NewSchema generic
// wrapper -- duplicated rather than imported, since importing the root
// gonest package from here would be an import cycle (gonest.go already
// imports internal/exception for its exported exception types/
// constructors).
func newSchema[T any](fn func(t *T, m *schema.Schema)) *schema.Schema {
	var zero T
	m := schema.New(reflect.TypeOf(zero), uintptr(unsafe.Pointer(&zero)))
	fn(&zero, m)
	return m
}
