package openapi

import (
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/schema"
)

// routableController is a locally-declared interface used to type-assert
// module.ControllerRef values down to the methods Generate needs
// (PathPrefix/OwnRoutes/OwnTags/HasBearerAuth) but that module.ControllerRef
// itself does not expose -- same pattern internal/app/app.go's own
// routableController interface already established for the same structural
// reason (PathPrefix/OwnRoutes are route-collection concerns, not something
// module.Module needs to know about a registered controller). Already
// implemented by *controller.Controller.
type routableController interface {
	PathPrefix() string
	OwnRoutes() []*route.Route
	OwnTags() []string
	HasBearerAuth() bool
}

// Generate walks root + root.ImportedModules() (recursively, cycle-safe via
// a visited-module set -- Modules CAN import each other in diamond/circular
// shapes) and every Module's OwnControllers()/Controller's OwnRoutes(),
// building doc.paths/doc.schemas as it goes (design.md's Components: "internal/
// openapi.Generate"). doc.paths/doc.schemas/doc.schemaNames are lazily
// initialized here if nil, so a fresh *OpenAPI (from New) works
// without any extra setup.
func Generate(doc *OpenAPI, root *module.Module) {
	if doc.paths == nil {
		doc.paths = map[string]map[string]any{}
	}
	if doc.schemas == nil {
		doc.schemas = map[string]any{}
	}
	if doc.schemaNames == nil {
		doc.schemaNames = map[*schema.Schema]string{}
	}

	visitedModules := map[*module.Module]bool{}
	walkModule(root, visitedModules, doc)
}

// walkModule visits m exactly once (visitedModules prevents infinite
// recursion on circular/diamond module imports -- design.md's Error Handling
// Strategy), walking its own controllers before recursing into
// m.ImportedModules().
func walkModule(m *module.Module, visitedModules map[*module.Module]bool, doc *OpenAPI) {
	if m == nil || visitedModules[m] {
		return
	}
	visitedModules[m] = true

	for _, cref := range m.OwnControllers() {
		rc, ok := cref.(routableController)
		if !ok {
			continue
		}
		walkController(rc, doc)
	}

	for _, imported := range m.ImportedModules() {
		walkModule(imported, visitedModules, doc)
	}
}

// walkController builds one path item entry per non-excluded route owned by
// c, keyed by c.PathPrefix()+route.Path() (the SAME concatenation convention
// internal/app/app.go's own registerRoutes already uses for
// fullPath := prefix + r.Path(), confirmed by reading that file directly --
// no separator inserted, matching whatever the controller/route declared
// literally).
func walkController(c routableController, doc *OpenAPI) {
	for _, r := range c.OwnRoutes() {
		if r.IsExcludedFromDocs() {
			continue
		}
		walkRoute(c, r, doc)
	}
}

// walkRoute builds ONE path item entry for r, resolving Tags/BearerAuth
// inheritance (route's own value wins when set, else falls back to the
// controller's -- design.md's Tech Decisions: "resolution deferred to
// generation time", since Route has no back-reference to its owning
// Controller).
func walkRoute(c routableController, r *route.Route, doc *OpenAPI) {
	fullPath := c.PathPrefix() + r.Path()
	method := r.Method().String()
	methodKey := toLowerAscii(method)

	op := map[string]any{}

	if s := r.SummaryText(); s != "" {
		op["summary"] = s
	}
	if s := r.DescriptionText(); s != "" {
		op["description"] = s
	}
	if s := r.OperationIdText(); s != "" {
		op["operationId"] = s
	}

	tags := resolveTags(c, r)
	if len(tags) > 0 {
		op["tags"] = tags
	}

	if r.IsDeprecated() {
		op["deprecated"] = true
	}

	if resolveBearerAuth(c, r) {
		op["security"] = []any{
			map[string]any{"bearerAuth": []any{}},
		}
	}

	visiting := map[*schema.Schema]bool{}

	var parameters []any
	if pathParams, ok := r.PathParamsSchema(); ok {
		parameters = append(parameters, paramsToParameters(pathParams, "path", "param", doc, visiting)...)
	}
	if queryParams, ok := r.QueryParamsSchema(); ok {
		parameters = append(parameters, paramsToParameters(queryParams, "query", "query", doc, visiting)...)
	}
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}

	if reqBody, ok := r.RequestBodySchema(); ok {
		op["requestBody"] = map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": refSchema(reqBody, doc, visiting),
				},
			},
		}
	}

	op["responses"] = buildResponses(r, doc, visiting)

	if doc.paths[fullPath] == nil {
		doc.paths[fullPath] = map[string]any{}
	}
	doc.paths[fullPath][methodKey] = op
}

// resolveTags implements the override resolution design.md's Tech Decisions
// table describes: route's OWN tags win entirely when Tags was called on the
// route (even with an empty/different set), else the controller's own tags
// are inherited.
func resolveTags(c routableController, r *route.Route) []string {
	if tags, set := r.OwnTags(); set {
		return tags
	}
	return c.OwnTags()
}

// resolveBearerAuth implements the same override resolution as resolveTags,
// for the bearer-auth boolean.
func resolveBearerAuth(c routableController, r *route.Route) bool {
	if value, set := r.HasBearerAuth(); set {
		return value
	}
	return c.HasBearerAuth()
}

// buildResponses converts r.Responses() (status -> *schema.Schema, nil
// value meaning "documented, no body") into the OpenAPI responses map, plus
// a synthesized default status (via r.Code()) when Response was never called
// at all (spec.md's Decision 4 -- undocumented routes still appear, using
// whatever is inferable). A 4xx/5xx status documented with no explicit
// schema (e.g. `r.Response(http.StatusNotFound)`) defaults to
// exception.Schema instead of a bare empty description -- HttpException is
// the framework's single default carrier for every built-in and dev-defined
// exception, so it is always a truthful (if generic) description of what
// that status actually returns. r.ResponseDescription(status, ...) always
// wins over both of those defaults, for any status.
func buildResponses(r *route.Route, doc *OpenAPI, visiting map[*schema.Schema]bool) map[string]any {
	responses := r.Responses()
	descriptions := r.ResponseDescriptions()
	out := map[string]any{}

	if len(responses) == 0 {
		out[strconv.Itoa(r.Code())] = map[string]any{"description": ""}
		return out
	}

	for status, m := range responses {
		key := strconv.Itoa(status)
		var entry map[string]any

		if m == nil && status >= http.StatusBadRequest {
			entry = defaultErrorResponse(status, doc, visiting)
		} else {
			entry = map[string]any{"description": ""}
			if m != nil {
				entry["content"] = map[string]any{
					"application/json": map[string]any{
						"schema": refSchema(m, doc, visiting),
					},
				}
			}
		}

		if desc, ok := descriptions[status]; ok {
			entry["description"] = desc
		}
		out[key] = entry
	}

	return out
}

// defaultErrorResponse builds the default Response Object for an
// undocumented 4xx/5xx status: description = http.StatusText(status),
// content = exception.Schema plus an example value shaped after that
// status's own built-in exception name where one exists (NotFoundException,
// BadRequestException, etc. -- see internal/exception/builtin.go), falling
// back to a name derived from the status text itself (e.g. 500 ->
// "InternalServerErrorException") for every other status.
func defaultErrorResponse(status int, doc *OpenAPI, visiting map[*schema.Schema]bool) map[string]any {
	return map[string]any{
		"description": http.StatusText(status),
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": refSchema(exception.Schema, doc, visiting),
				"example": map[string]any{
					"name":    defaultExceptionName(status),
					"message": http.StatusText(status),
					"details": nil,
				},
			},
		},
	}
}

// defaultExceptionName returns the framework's own built-in exception name
// for status, if one exists, else a name synthesized from
// http.StatusText(status) (e.g. "Internal Server Error" ->
// "InternalServerErrorException").
func defaultExceptionName(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BadRequestException"
	case http.StatusUnauthorized:
		return "UnauthorizedException"
	case http.StatusForbidden:
		return "ForbiddenException"
	case http.StatusNotFound:
		return "NotFoundException"
	case http.StatusConflict:
		return "ConflictException"
	default:
		return pascalCase(http.StatusText(status)) + "Exception"
	}
}

// pascalCase joins s's whitespace-separated words together, capitalizing
// each (e.g. "Internal Server Error" -> "InternalServerError").
func pascalCase(s string) string {
	fields := strings.Fields(s)
	for i, f := range fields {
		fields[i] = strings.ToUpper(f[:1]) + strings.ToLower(f[1:])
	}
	return strings.Join(fields, "")
}

// paramsToParameters converts every OwnProperties() entry of m into one
// OpenAPI Parameter Object, keyed by tagName's resolution (design.md's
// Components: "each property in that Schema becomes one parameter object").
// contextTag is "param" for path params, "query" for query params -- a
// path/query DTO field typically has no "json" tag at all (it's bound via
// MustParams/MustQuery against the "param"/"query" tag, not JSON), so
// falling back straight to the bare Go field name (e.g. "UserID" instead of
// "user_id") would document a name that doesn't match what actually binds
// the request.
func paramsToParameters(m *schema.Schema, in string, contextTag string, doc *OpenAPI, visiting map[*schema.Schema]bool) []any {
	var out []any
	for _, p := range m.OwnProperties() {
		name := tagName(p, contextTag)
		out = append(out, map[string]any{
			"name":     name,
			"in":       in,
			"required": p.IsRequired(),
			"schema":   schemaFor(p, doc, visiting),
		})
	}
	return out
}

// tagName resolves a PropertyBuilder's display name: the "json" tag wins
// first (the convention every request/response body schema is keyed by),
// then contextTag (the tag that actually binds path/query params at
// runtime -- "param"/"query", passed empty by callers where it doesn't
// apply), and only once neither tag is present does it fall back to the Go
// field name. Mirrors internal/validate's own tagKey/tagKeyVisible
// resolution (validate.go) closely enough for parameter/schema naming
// purposes -- Generate does not need the "-" (hidden) skip behavior
// validate's tagKeyVisible has, since a field that reaches OwnProperties()
// was deliberately registered via Property.
func tagName(p *schema.PropertyBuilder, contextTag string) string {
	field := p.Field()
	if name, ok := tagValue(field, "json"); ok {
		return name
	}
	if contextTag != "" {
		if name, ok := tagValue(field, contextTag); ok {
			return name
		}
	}
	return field.Name
}

// tagValue extracts tag's value off field (splitting on the first comma to
// drop options like ",omitempty"), reporting false when the tag is absent,
// empty, or "-".
func tagValue(field reflect.StructField, tag string) (string, bool) {
	raw := field.Tag.Get(tag)
	if raw == "" || raw == "-" {
		return "", false
	}
	name := raw
	for i, c := range raw {
		if c == ',' {
			name = raw[:i]
			break
		}
	}
	if name == "" {
		return "", false
	}
	return name, true
}

// refSchema builds the schema for a whole *schema.Schema used as a
// requestBody/response body (NOT a *PropertyBuilder) -- always a top-level
// object schema, registered/deduped via registerSchema and referenced by
// $ref (a request/response body is always the FULL registered shape, never
// inlined, matching how ItemRef()/SchemaRef() are handled by schemaFor for
// nested fields).
func refSchema(m *schema.Schema, doc *OpenAPI, visiting map[*schema.Schema]bool) map[string]any {
	name := registerSchema(m, doc, visiting)
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

// schemaFor is the recursive core: one *schema.PropertyBuilder -> one
// OpenAPI Schema Object (as map[string]any), dispatching on p.KindValue()
// exactly like internal/validate's own validateValue does (same source of
// truth, different destination -- schema instead of violation). Custom(fn)
// is checked FIRST, before the kind dispatch, mirroring validateValue's own
// short-circuit (validate.go).
func schemaFor(p *schema.PropertyBuilder, doc *OpenAPI, visiting map[*schema.Schema]bool) map[string]any {
	schema := map[string]any{}

	if _, isCustom := p.CustomFunc(); isCustom {
		// Custom fields have no inferable type/format (spec.md's Edge Cases)
		// -- only description/examples/nullable/required-adjacent fields are
		// emitted, no kind dispatch at all.
		addDescriptionAndExamples(schema, p)
		applyNullable(schema, p, false)
		return schema
	}

	switch p.KindValue() {
	case "string":
		schema["type"] = "string"
		if format := p.FormatValue(); format != "" {
			schema["format"] = format
		}
		if min, ok := p.MinValue(); ok {
			schema["minLength"] = min
		}
		if max, ok := p.MaxValue(); ok {
			schema["maxLength"] = max
		}
		if pattern := p.PatternValue(); pattern != "" {
			schema["pattern"] = pattern
		}
		applyNullable(schema, p, true)

	case "integer", "number":
		schema["type"] = p.KindValue()
		if format := p.FormatValue(); format != "" {
			schema["format"] = format
		}
		if min, ok := p.MinValue(); ok {
			schema["minimum"] = min
		}
		if max, ok := p.MaxValue(); ok {
			schema["maximum"] = max
		}
		applyNullable(schema, p, true)

	case "boolean":
		schema["type"] = "boolean"
		applyNullable(schema, p, true)

	case "array":
		schema["type"] = "array"
		if min, ok := p.MinValue(); ok {
			schema["minItems"] = min
		}
		if max, ok := p.MaxValue(); ok {
			schema["maxItems"] = max
		}
		if itemRef, ok := p.ItemRef(); ok {
			schema["items"] = refSchema(itemRef, doc, visiting)
		} else {
			schema["items"] = schemaFor(p.ItemBuilder(), doc, visiting)
		}
		applyNullable(schema, p, true)

	case "object":
		if ref, ok := p.SchemaRef(); ok {
			// The WHOLE schema is just a $ref, not wrapped in more object
			// structure (design.md's Data Models: "Object-with-ref example").
			//
			// SPEC_DEVIATION: when the field is ALSO Nullable(), OpenAPI 3.1
			// allows nullable-via-union using "type": ["null"] alongside a
			// sibling "$ref" is NOT valid per plain JSON Schema (a schema
			// object with "$ref" ignores sibling keywords in versions prior
			// to 2019-09, and even under 2020-12 semantics, mixing bare $ref
			// with type is fragile/awkward per the design.md task
			// instructions' own acknowledgement of this ambiguity). This
			// implementation wraps a nullable ref in "anyOf": [{"$ref":...},
			// {"type":"null"}] -- valid JSON Schema 2020-12 (which OpenAPI
			// 3.1 is built on), keeps the $ref intact and inspectable, and
			// avoids inventing a non-standard "nullable" sibling keyword
			// (removed from OpenAPI 3.1 in favor of type-arrays/anyOf).
			if p.IsNullable() {
				return map[string]any{
					"anyOf": []any{
						map[string]any{"$ref": "#/components/schemas/" + registerSchema(ref, doc, visiting)},
						map[string]any{"type": "null"},
					},
				}
			}
			return refSchema(ref, doc, visiting)
		}
		schema["type"] = "object"
		if p.IsAdditionalProperties() {
			schema["additionalProperties"] = true
		}
		applyNullable(schema, p, true)

	default:
		// No branch method was ever called on this PropertyBuilder (kind ==
		// "") -- nothing to dispatch against, same "no-op" stance
		// internal/validate's validatePrimitive default case already takes
		// for an unrecognized/unset kind.
	}

	addDescriptionAndExamples(schema, p)
	return schema
}

// applyNullable wraps schema's "type" entry into a 2-element []string
// including "null" (OpenAPI 3.1 / JSON Schema 2020-12 style) when
// p.IsNullable() is true and hasType is true (schema already has a "type"
// key to wrap). Bare Custom fields (hasType=false) have no type key to wrap
// at all -- Nullable on a Custom field is documented via its absence from
// the schema entirely (design.md/spec.md do not ask for a synthetic
// "nullable" marker on a field with no inferable shape).
func applyNullable(schema map[string]any, p *schema.PropertyBuilder, hasType bool) {
	if !hasType || !p.IsNullable() {
		return
	}
	t, ok := schema["type"].(string)
	if !ok {
		return
	}
	schema["type"] = []string{t, "null"}
}

// addDescriptionAndExamples adds description/examples to schema, if set --
// applies to EVERY kind's output, including Custom fields (design.md's Data
// Models: "Add description/examples ... to every kind's output, if set").
func addDescriptionAndExamples(schema map[string]any, p *schema.PropertyBuilder) {
	if desc := p.DescriptionText(); desc != "" {
		schema["description"] = desc
	}
	if examples := p.ExamplesList(); len(examples) > 0 {
		schema["examples"] = examples
	}
}

// registerSchema ensures m has EXACTLY ONE entry in doc.schemas (dedup via
// doc.schemaNames, pointer-keyed -- design.md's Data Models: "the SOLE dedup
// mechanism"), returning the schema's name for building a $ref string.
// doc.schemaNames is checked FIRST, before ever walking m's own properties,
// and the name is RESERVED in doc.schemaNames BEFORE recursing into m's
// properties -- both orderings are critical: checking first is what makes a
// diamond-shaped reference graph (two different routes both referencing the
// same *Schema) resolve to a single walk, and reserving before recursing
// is what prevents infinite recursion if m indirectly references itself
// (e.g. a self-referential tree-shaped struct).
func registerSchema(m *schema.Schema, doc *OpenAPI, visiting map[*schema.Schema]bool) string {
	if doc.schemas == nil {
		doc.schemas = map[string]any{}
	}
	if doc.schemaNames == nil {
		doc.schemaNames = map[*schema.Schema]string{}
	}

	if name, ok := doc.schemaNames[m]; ok {
		return name
	}

	name := m.TitleText()
	if name == "" {
		name = m.StructType().Name()
	}

	doc.schemaNames[m] = name

	properties := map[string]any{}
	var required []string

	for _, p := range m.OwnProperties() {
		key := tagName(p, "")
		properties[key] = schemaFor(p, doc, visiting)
		if p.IsRequired() {
			required = append(required, key)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"title":      name,
		"properties": properties,
	}
	if desc := m.DescriptionText(); desc != "" {
		schema["description"] = desc
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	doc.schemas[name] = schema

	return name
}

// Document assembles the FULL OpenAPI 3.1 JSON-ready structure
// (openapi/info/paths/components/security) from every field doc already
// holds -- both the document-level fields openapi.go's own builder methods
// populate (Title/Contact/etc) and the paths/schemas Generate populates.
// This is what a future "Swagger UI Setup" feature will json.Marshal.
func (doc *OpenAPI) Document() map[string]any {
	info := map[string]any{
		"title":   doc.TitleText(),
		"version": doc.VersionText(),
	}
	if desc := doc.DescriptionText(); desc != "" {
		info["description"] = desc
	}

	name, url, email := doc.ContactInfo()
	if name != "" || url != "" || email != "" {
		contact := map[string]any{}
		if name != "" {
			contact["name"] = name
		}
		if url != "" {
			contact["url"] = url
		}
		if email != "" {
			contact["email"] = email
		}
		info["contact"] = contact
	}

	licName, licUrl := doc.LicenseInfo()
	if licName != "" || licUrl != "" {
		license := map[string]any{}
		if licName != "" {
			license["name"] = licName
		}
		if licUrl != "" {
			license["url"] = licUrl
		}
		info["license"] = license
	}

	paths := doc.paths
	if paths == nil {
		paths = map[string]map[string]any{}
	}
	// paths is map[string]map[string]any -- convert to map[string]any so
	// json.Marshal sees a uniform any-valued tree (matches every other
	// section of this document), rather than mixing concrete map types.
	pathsAny := make(map[string]any, len(paths))
	for k, v := range paths {
		pathsAny[k] = v
	}

	schemas := doc.schemas
	if schemas == nil {
		schemas = map[string]any{}
	}

	components := map[string]any{
		"schemas": schemas,
	}

	if hasAnyBearerAuth(doc) {
		components["securitySchemes"] = map[string]any{
			"bearerAuth": map[string]any{
				"type":   "http",
				"scheme": "bearer",
			},
		}
	}

	out := map[string]any{
		"openapi":    doc.SpecVersion(),
		"info":       info,
		"paths":      pathsAny,
		"components": components,
	}

	if doc.HasBearerAuth() {
		out["security"] = []any{
			map[string]any{"bearerAuth": []any{}},
		}
	}

	return out
}

// hasAnyBearerAuth reports whether doc-level BearerAuth was set OR any
// generated path operation carries a bearer-auth security requirement
// (design.md's Components: "securitySchemes":{"bearerAuth":...} if doc has
// any bearer auth anywhere).
func hasAnyBearerAuth(doc *OpenAPI) bool {
	if doc.HasBearerAuth() {
		return true
	}
	for _, methods := range doc.paths {
		for _, opAny := range methods {
			op, ok := opAny.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := op["security"]; ok {
				return true
			}
		}
	}
	return false
}

// toLowerAscii lowercases s (ASCII only -- HTTP method strings are always
// ASCII, e.g. "GET" -> "get") without pulling in strings.ToLower purely for
// this one call site's sake being a meaningful savings; kept as a tiny local
// helper mirroring the scope of what's actually needed here.
func toLowerAscii(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
