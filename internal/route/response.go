package route

import (
	"strings"

	"gonest.dev/gonest/internal/schema"
)

// RouteResponse is the per-status response builder passed to Route.Response's
// optional callback -- lets a route configure that status's documented
// body schema and/or description in one place (INSIGHT.md's
// `route.Response(201, func(response *gonest.RouteResponse) { response.Schema(...) })`
// shape), instead of two unrelated method calls per status. Named
// RouteResponse (not plain Response) since request-response-split feature
// claimed gonest.Response for the write-side of the (req, res) HTTP pair --
// this type is unrelated to that one, purely an OpenAPI documentation
// builder.
type RouteResponse struct {
	schemaValue *schema.Schema

	description    string
	descriptionSet bool
}

// Schema sets this response's documented body schema and returns r so
// calls can chain.
func (r *RouteResponse) Schema(m *schema.Schema) *RouteResponse {
	r.schemaValue = m
	return r
}

// SchemaValue returns the schema set via Schema, and whether Schema was
// ever called -- the bool distinguishes "documented, no body" from
// "documented with a body".
func (r *RouteResponse) SchemaValue() (*schema.Schema, bool) {
	return r.schemaValue, r.schemaValue != nil
}

// Description sets a custom description for this response, overriding
// whatever description generation would otherwise pick (empty string for
// a schema-carrying response, or the auto-derived http.StatusText default
// for an undocumented 4xx/5xx response -- see internal/openapi's
// buildResponses/defaultErrorResponse). Returns r so calls can chain. Takes
// at least one word (word, words...) rather than a bare `...string`, same
// reasoning as route.Route.Summary's own doc comment -- joined with a
// single space so a long description can be split across multiple Go
// string literals without manual concatenation.
func (r *RouteResponse) Description(word string, words ...string) *RouteResponse {
	r.description = strings.Join(append([]string{word}, words...), " ")
	r.descriptionSet = true
	return r
}

// DescriptionText returns the description set via Description, and
// whether Description was ever called.
func (r *RouteResponse) DescriptionText() (string, bool) {
	return r.description, r.descriptionSet
}
