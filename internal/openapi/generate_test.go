package openapi

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/schema"
)

// --- fixtures -------------------------------------------------------------

type addressEntity struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

type userEntity struct {
	Id        int64           `json:"id"`
	Tags      []string        `json:"tags"`
	Scores    []int           `json:"scores"`
	Addresses []addressEntity `json:"addresses"`
	Address   addressEntity   `json:"address"`
	Schema    map[string]any  `json:"schema"`
}

// newUserSchema reproduces INSIGHT.md's "exemplo de Array e Object
// aninhados" UserEntity/AddressEntity shape using the CURRENT
// Array()/Items(fn)/Object(fn) builder API (schema.go, array.go,
// object.go) -- INSIGHT.md's own code block in that section predates T0/T1
// and uses an older API shape; this reproduces the same STRUCTURE (nested
// Array/Object, addressSchema reused across Addresses and Address) against
// the API that actually exists today.
func newUserSchema(tst *testing.T) (*schema.Schema, *schema.Schema) {
	var t userEntity
	base := reflect.ValueOf(&t).Pointer()
	m := schema.New(reflect.TypeOf(t), base)
	tst.Cleanup(func() { schema.Deregister(reflect.TypeOf(t)) })

	var addr addressEntity
	addrBase := reflect.ValueOf(&addr).Pointer()
	addressSchema := schema.New(reflect.TypeOf(addr), addrBase)
	tst.Cleanup(func() { schema.Deregister(reflect.TypeOf(addr)) })
	addressSchema.Description("Endereco")
	addressSchema.Property(&addr.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
	addressSchema.Property(&addr.City).String().Required().Description("Cidade")
	addressSchema.Property(&addr.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP")

	m.Description("Entidade de usuario com campos aninhados")
	m.Property(&t.Id).Integer().Required().Description("ID do usuario").Examples(int64(1))

	m.Property(&t.Tags).Array().Items(func(am *schema.ArraySchema) {
		am.String().Min(1).Max(50)
	}).Required().Description("Tags do usuario").Examples("admin", "beta")

	m.Property(&t.Scores).Array().Items(func(am *schema.ArraySchema) {
		am.Integer().Min(0).Max(100)
	}).Required().Description("Notas do usuario")

	m.Property(&t.Addresses).Array().Items(func(am *schema.ArraySchema) {
		am.Object(addressSchema)
	}).Required().Min(1).Description("Enderecos do usuario")

	m.Property(&t.Address).Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
	}).Required().Description("Endereco principal")

	m.Property(&t.Schema).Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
	}).Nullable().Description("Metadados abertos do usuario")

	return m, addressSchema
}

// --- SG-04: every route appears in paths -----------------------------------

func TestGenerate_RouteInRootModule_AppearsInPaths(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/users")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	item, ok := doc.paths["/users/:id"]
	if !ok {
		t.Fatalf("paths missing key %q, got %v", "/users/:id", doc.paths)
	}
	if _, ok := item["get"]; !ok {
		t.Fatalf("path item missing method %q, got %v", "get", item)
	}
}

func TestGenerate_RouteInImportedModule_Recursive_AppearsInPaths(t *testing.T) {
	subController := controller.New(func(c *controller.Controller) {
		c.Path("/sub")
		c.Route(route.HttpPost, "/", func(r *route.Route) {})
	})
	subController.Declare()

	subModule := module.New(func(m *module.Module) {
		m.Controllers(subController)
	})

	root := module.New(func(m *module.Module) {
		m.Imports(subModule)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	item, ok := doc.paths["/sub/"]
	if !ok {
		t.Fatalf("paths missing key from imported module, got %v", doc.paths)
	}
	if _, ok := item["post"]; !ok {
		t.Fatalf("path item missing method post, got %v", item)
	}
}

func TestGenerate_ExcludedRoute_DoesNotAppearInPaths(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/hidden")
		c.Route(route.HttpGet, "/", func(r *route.Route) {
			r.ExcludeFromDocs()
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	if _, ok := doc.paths["/hidden/"]; ok {
		t.Fatalf("excluded route should not appear in paths, got %v", doc.paths)
	}
}

func TestGenerate_UndocumentedRoute_StillAppearsWithInferredState(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/plain")
		c.Route(route.HttpDelete, "/:id", func(r *route.Route) {})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	item, ok := doc.paths["/plain/:id"]
	if !ok {
		t.Fatalf("undocumented route missing from paths, got %v", doc.paths)
	}
	op, ok := item["delete"].(map[string]any)
	if !ok {
		t.Fatalf("path item missing method delete, got %v", item)
	}
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		t.Fatalf("op missing responses, got %v", op)
	}
	if _, ok := responses["200"]; !ok {
		t.Fatalf("undocumented route should infer default status 200, got %v", responses)
	}
}

// --- SG-05: dedup by pointer identity ---------------------------------------

func TestGenerate_SharedSchema_RegisteredExactlyOnce(t *testing.T) {
	userMeta, addressMeta := newUserSchema(t)

	c := controller.New(func(c *controller.Controller) {
		c.Path("/users")
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.RequestBody(userMeta)
			r.Response(201, func(response *route.Response) { response.Schema(userMeta) })
		})
		c.Route(route.HttpGet, "/address", func(r *route.Route) {
			r.Response(200, func(response *route.Response) { response.Schema(addressMeta) })
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	count := 0
	for name := range doc.schemas {
		if name == "addressEntity" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 addressEntity schema entry, got %d (schemas=%v)", count, keys(doc.schemas))
	}

	// userEntity should also be registered exactly once (referenced twice:
	// RequestBody + Response(201)).
	userCount := 0
	for name := range doc.schemas {
		if name == "userEntity" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("expected exactly 1 userEntity schema entry, got %d", userCount)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- SG-06: schema shape per branch family ----------------------------------

type stringBranchEntity struct {
	Name string `json:"name"`
}

func TestSchemaFor_String_MinMaxPattern(t *testing.T) {
	var t1 stringBranchEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.Name).String().Min(2).Max(10).Pattern(`^[a-z]+$`).Required()

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	if schema["type"] != "string" {
		t.Fatalf("type = %v, want string", schema["type"])
	}
	if schema["minLength"] != 2 {
		t.Fatalf("minLength = %v, want 2", schema["minLength"])
	}
	if schema["maxLength"] != 10 {
		t.Fatalf("maxLength = %v, want 10", schema["maxLength"])
	}
	if schema["pattern"] != `^[a-z]+$` {
		t.Fatalf("pattern = %v, want ^[a-z]+$", schema["pattern"])
	}
}

type numericBranchEntity struct {
	Age int `json:"age"`
}

func TestSchemaFor_Integer_MinMaxFormat(t *testing.T) {
	var t1 numericBranchEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.Age).Integer().Min(0).Max(150)

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	if schema["type"] != "integer" {
		t.Fatalf("type = %v, want integer", schema["type"])
	}
	if schema["format"] != "int64" {
		t.Fatalf("format = %v, want int64", schema["format"])
	}
	if schema["minimum"] != 0 {
		t.Fatalf("minimum = %v, want 0", schema["minimum"])
	}
	if schema["maximum"] != 150 {
		t.Fatalf("maximum = %v, want 150", schema["maximum"])
	}
}

type boolBranchEntity struct {
	Active bool `json:"active"`
}

func TestSchemaFor_Boolean(t *testing.T) {
	var t1 boolBranchEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.Active).Boolean()

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	if schema["type"] != "boolean" {
		t.Fatalf("type = %v, want boolean", schema["type"])
	}
	if _, ok := schema["format"]; ok {
		t.Fatalf("boolean schema should not have format, got %v", schema)
	}
}

type dateBranchEntity struct {
	CreatedAt string `json:"createdAt"`
	BirthDate string `json:"birthDate"`
}

func TestSchemaFor_DateTime_And_Date(t *testing.T) {
	var t1 dateBranchEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.CreatedAt).DateTime()
	m.Property(&t1.BirthDate).Date()

	doc := New("3.1.0", nil)
	for _, pb := range m.OwnProperties() {
		schema := schemaFor(pb, doc, map[*schema.Schema]bool{})
		if schema["type"] != "string" {
			t.Fatalf("type = %v, want string", schema["type"])
		}
		format := schema["format"]
		if format != "date-time" && format != "date" {
			t.Fatalf("format = %v, want date-time or date", format)
		}
	}
}

func TestSchemaFor_Array_InlineItem(t *testing.T) {
	var t1 userEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	t.Cleanup(func() { schema.Deregister(reflect.TypeOf(t1)) })
	m.Property(&t1.Tags).Array().Items(func(am *schema.ArraySchema) {
		am.String().Min(1).Max(50)
	}).Min(1).Max(5)

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	if schema["type"] != "array" {
		t.Fatalf("type = %v, want array", schema["type"])
	}
	if schema["minItems"] != 1 || schema["maxItems"] != 5 {
		t.Fatalf("minItems/maxItems = %v/%v, want 1/5", schema["minItems"], schema["maxItems"])
	}
	items, ok := schema["items"].(map[string]any)
	if !ok {
		t.Fatalf("items missing or wrong type: %v", schema["items"])
	}
	if items["type"] != "string" {
		t.Fatalf("items.type = %v, want string", items["type"])
	}
	if items["minLength"] != 1 || items["maxLength"] != 50 {
		t.Fatalf("items min/max = %v/%v, want 1/50", items["minLength"], items["maxLength"])
	}
}

func TestSchemaFor_Array_RefItem(t *testing.T) {
	userMeta, _ := newUserSchema(t)
	doc := New("3.1.0", nil)

	var addressesPB *schema.PropertyBuilder
	for _, pb := range userMeta.OwnProperties() {
		if pb.Field().Name == "Addresses" {
			addressesPB = pb
		}
	}
	if addressesPB == nil {
		t.Fatal("could not find Addresses property")
	}

	schema := schemaFor(addressesPB, doc, map[*schema.Schema]bool{})
	if schema["type"] != "array" {
		t.Fatalf("type = %v, want array", schema["type"])
	}
	items, ok := schema["items"].(map[string]any)
	if !ok {
		t.Fatalf("items missing or wrong type: %v", schema["items"])
	}
	ref, ok := items["$ref"].(string)
	if !ok || ref == "" {
		t.Fatalf("items.$ref missing, got %v", items)
	}
	if _, ok := doc.schemas["addressEntity"]; !ok {
		t.Fatalf("addressEntity should have been registered, got %v", keys(doc.schemas))
	}
}

func TestSchemaFor_Object_Ref(t *testing.T) {
	userMeta, _ := newUserSchema(t)
	doc := New("3.1.0", nil)

	var addressPB *schema.PropertyBuilder
	for _, pb := range userMeta.OwnProperties() {
		if pb.Field().Name == "Address" {
			addressPB = pb
		}
	}
	if addressPB == nil {
		t.Fatal("could not find Address property")
	}

	schema := schemaFor(addressPB, doc, map[*schema.Schema]bool{})
	ref, ok := schema["$ref"].(string)
	if !ok || ref == "" {
		t.Fatalf("expected bare $ref schema, got %v", schema)
	}
	if len(schema) != 1 {
		t.Fatalf("Object-with-ref schema should ONLY have $ref, got %v", schema)
	}
}

// TestSchemaFor_Object_Ref_Nullable covers the gap left by
// TestSchemaFor_Object_Ref (Required, not Nullable) and
// TestSchemaFor_Object_AdditionalProperties (Nullable, but no SchemaRef):
// a field that is BOTH Object(...).Schema(ref) (SchemaRef set) AND
// Nullable(). schemaFor's "object" case (generate.go) special-cases this
// combination -- since a bare "$ref" cannot carry a sibling "nullable"/type
// marker, it wraps the ref in "anyOf": [{"$ref": ...}, {"type": "null"}]
// instead of returning the bare $ref schema that TestSchemaFor_Object_Ref
// asserts for the non-nullable case.
func TestSchemaFor_Object_Ref_Nullable(t *testing.T) {
	_, addressSchema := newUserSchema(t)

	type ownerEntity struct {
		Address addressEntity `json:"address"`
	}
	var owner ownerEntity
	ownerBase := reflect.ValueOf(&owner).Pointer()
	m := schema.New(reflect.TypeOf(owner), ownerBase)
	t.Cleanup(func() { schema.Deregister(reflect.TypeOf(owner)) })

	m.Property(&owner.Address).Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
	}).Nullable().Description("Endereco opcional")

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]

	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	want := map[string]any{
		"anyOf": []any{
			map[string]any{"$ref": "#/components/schemas/addressEntity"},
			map[string]any{"type": "null"},
		},
	}
	if !reflect.DeepEqual(schema, want) {
		t.Fatalf("schema = %#v, want %#v", schema, want)
	}
}

func TestSchemaFor_Object_AdditionalProperties(t *testing.T) {
	userMeta, _ := newUserSchema(t)
	doc := New("3.1.0", nil)

	var metaPB *schema.PropertyBuilder
	for _, pb := range userMeta.OwnProperties() {
		if pb.Field().Name == "Schema" {
			metaPB = pb
		}
	}
	if metaPB == nil {
		t.Fatal("could not find Schema property")
	}

	schema := schemaFor(metaPB, doc, map[*schema.Schema]bool{})
	if schema["type"] != "object" && !containsNull(schema["type"], "object") {
		t.Fatalf("type = %v, want object (possibly nullable)", schema["type"])
	}
	if schema["additionalProperties"] != true {
		t.Fatalf("additionalProperties = %v, want true", schema["additionalProperties"])
	}
}

func containsNull(v any, want string) bool {
	arr, ok := v.([]string)
	if !ok {
		return false
	}
	found := false
	for _, s := range arr {
		if s == want {
			found = true
		}
	}
	return found
}

// --- Required/Nullable ------------------------------------------------------

type reqNullableEntity struct {
	Required string `json:"required"`
	Nullable string `json:"nullable"`
}

func TestSchemaFor_Nullable_WrapsTypeArray(t *testing.T) {
	var t1 reqNullableEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.Nullable).String().Nullable()

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	arr, ok := schema["type"].([]string)
	if !ok {
		t.Fatalf("type should be []string when Nullable, got %T: %v", schema["type"], schema["type"])
	}
	if !containsNull(arr, "null") || !containsNull(arr, "string") {
		t.Fatalf("type array = %v, want [string null]", arr)
	}
}

func TestGenerate_Required_ReflectedInSchemaRequiredArray(t *testing.T) {
	userMeta, _ := newUserSchema(t)

	c := controller.New(func(c *controller.Controller) {
		c.Path("/users")
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.RequestBody(userMeta)
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	schema, ok := doc.schemas["userEntity"].(map[string]any)
	if !ok {
		t.Fatalf("userEntity schema missing, got %v", keys(doc.schemas))
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required missing or wrong type: %v", schema["required"])
	}
	if !containsNull(required, "id") {
		t.Fatalf("required = %v, want it to contain %q", required, "id")
	}
}

// --- Custom(fn) --------------------------------------------------------------

type customEntity struct {
	Weird string `json:"weird"`
}

func TestSchemaFor_Custom_NoTypeOrFormat(t *testing.T) {
	var t1 customEntity
	base := reflect.ValueOf(&t1).Pointer()
	m := schema.New(reflect.TypeOf(t1), base)
	m.Property(&t1.Weird).Custom(func(raw any) (any, error) { return raw, nil }).Description("weird field").Required()

	doc := New("3.1.0", nil)
	pb := m.OwnProperties()[0]
	schema := schemaFor(pb, doc, map[*schema.Schema]bool{})

	if _, ok := schema["type"]; ok {
		t.Fatalf("Custom field should have no type, got %v", schema)
	}
	if _, ok := schema["format"]; ok {
		t.Fatalf("Custom field should have no format, got %v", schema)
	}
	if schema["description"] != "weird field" {
		t.Fatalf("description = %v, want %q", schema["description"], "weird field")
	}
}

// --- Tags/BearerAuth override resolution ------------------------------------

func TestGenerate_Tags_RouteOverridesController(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/x")
		c.Tags("controller-tag")
		c.Route(route.HttpGet, "/override", func(r *route.Route) {
			r.Tags("route-tag")
		})
		c.Route(route.HttpGet, "/inherit", func(r *route.Route) {})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	overrideOp := doc.paths["/x/override"]["get"].(map[string]any)
	overrideTags, _ := overrideOp["tags"].([]string)
	if len(overrideTags) != 1 || overrideTags[0] != "route-tag" {
		t.Fatalf("override tags = %v, want [route-tag]", overrideTags)
	}

	inheritOp := doc.paths["/x/inherit"]["get"].(map[string]any)
	inheritTags, _ := inheritOp["tags"].([]string)
	if len(inheritTags) != 1 || inheritTags[0] != "controller-tag" {
		t.Fatalf("inherit tags = %v, want [controller-tag]", inheritTags)
	}
}

func TestGenerate_BearerAuth_RouteOverridesController(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/secure")
		c.BearerAuth()
		c.Route(route.HttpGet, "/public", func(r *route.Route) {
			// no override: inherits controller's bearer auth = true
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	op := doc.paths["/secure/public"]["get"].(map[string]any)
	if _, ok := op["security"]; !ok {
		t.Fatalf("op should inherit controller's bearer auth as security, got %v", op)
	}
}

func TestGenerate_BearerAuth_NoAuth_OmitsSecurityKey(t *testing.T) {
	c := controller.New(func(c *controller.Controller) {
		c.Path("/open")
		c.Route(route.HttpGet, "/", func(r *route.Route) {})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	op := doc.paths["/open/"]["get"].(map[string]any)
	if _, ok := op["security"]; ok {
		t.Fatalf("op should not have security key when no bearer auth set, got %v", op)
	}
}

// --- full INSIGHT.md UserEntity/AddressEntity reproduction ------------------

func TestGenerate_UserEntityAddressEntity_FullReproduction(t *testing.T) {
	userMeta, addressMeta := newUserSchema(t)

	c := controller.New(func(c *controller.Controller) {
		c.Path("/users")
		c.Tags("users")
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.Summary("Create user")
			r.RequestBody(userMeta)
			r.Response(201, func(response *route.Response) { response.Schema(userMeta) })
			r.Response(400)
		})
		c.Route(route.HttpGet, "/:id/address", func(r *route.Route) {
			r.Response(200, func(response *route.Response) { response.Schema(addressMeta) })
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	// exactly one addressEntity entry despite 2 references (nested in
	// userEntity's Addresses/Address AND the direct /address route response)
	addrCount := 0
	for name := range doc.schemas {
		if name == "addressEntity" {
			addrCount++
		}
	}
	if addrCount != 1 {
		t.Fatalf("expected exactly 1 addressEntity, got %d", addrCount)
	}

	postOp, ok := doc.paths["/users/"]["post"].(map[string]any)
	if !ok {
		t.Fatalf("POST /users/ missing, got %v", doc.paths)
	}
	if postOp["summary"] != "Create user" {
		t.Fatalf("summary = %v, want %q", postOp["summary"], "Create user")
	}
	reqBody, ok := postOp["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("requestBody missing, got %v", postOp)
	}
	content, ok := reqBody["content"].(map[string]any)
	if !ok {
		t.Fatalf("requestBody.content missing, got %v", reqBody)
	}
	if _, ok := content["application/json"]; !ok {
		t.Fatalf("requestBody.content missing application/json, got %v", content)
	}

	responses, ok := postOp["responses"].(map[string]any)
	if !ok {
		t.Fatalf("responses missing, got %v", postOp)
	}
	if _, ok := responses["201"]; !ok {
		t.Fatalf("responses missing 201, got %v", responses)
	}
	resp400, ok := responses["400"].(map[string]any)
	if !ok {
		t.Fatalf("responses missing 400, got %v", responses)
	}
	if resp400["description"] != "Bad Request" {
		t.Fatalf("bodyless 400 response description = %v, want %q", resp400["description"], "Bad Request")
	}
	resp400Content, ok := resp400["content"].(map[string]any)
	if !ok {
		t.Fatalf("bodyless 400 response should default to an HttpException content, got %v", resp400)
	}
	resp400Body, ok := resp400Content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("bodyless 400 response content missing application/json, got %v", resp400Content)
	}
	resp400Schema, ok := resp400Body["schema"].(map[string]any)
	if !ok || resp400Schema["$ref"] != "#/components/schemas/HttpException" {
		t.Fatalf("bodyless 400 response schema = %v, want $ref to HttpException", resp400Body["schema"])
	}
}

// --- Document() --------------------------------------------------------------

func TestDocument_ProducesValidJSONMarshalableOutput(t *testing.T) {
	userMeta, _ := newUserSchema(t)

	c := controller.New(func(c *controller.Controller) {
		c.Path("/users")
		c.BearerAuth()
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.RequestBody(userMeta)
			r.Response(201, func(response *route.Response) { response.Schema(userMeta) })
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", func(b *OpenAPI) {
		b.Title("Test API").Version("1.0.0")
	})
	Generate(doc, root)

	out := doc.Document()

	if out["openapi"] != "3.1.0" {
		t.Fatalf("openapi = %v, want 3.1.0", out["openapi"])
	}
	info, ok := out["info"].(map[string]any)
	if !ok || info["title"] != "Test API" {
		t.Fatalf("info.title wrong, got %v", out["info"])
	}
	components, ok := out["components"].(map[string]any)
	if !ok {
		t.Fatalf("components missing, got %v", out)
	}
	securitySchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatalf("securitySchemes missing when bearer auth used, got %v", components)
	}
	if _, ok := securitySchemes["bearerAuth"]; !ok {
		t.Fatalf("bearerAuth securityScheme missing, got %v", securitySchemes)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("marshaled output is empty")
	}
}

// --- circular module import --------------------------------------------------

func TestGenerate_CircularModuleImport_Terminates(t *testing.T) {
	controllerA := controller.New(func(c *controller.Controller) {
		c.Path("/a")
		c.Route(route.HttpGet, "/", func(r *route.Route) {})
	})
	controllerA.Declare()

	controllerB := controller.New(func(c *controller.Controller) {
		c.Path("/b")
		c.Route(route.HttpGet, "/", func(r *route.Route) {})
	})
	controllerB.Declare()

	// moduleA and moduleB import each other (circular). Module.New(fn) can't
	// reference the other module before it exists, so both are constructed
	// with a nil fn and wired directly via Imports/Controllers (plain
	// append operations, safe to call outside New's deferred fn).
	moduleA := module.New(nil)
	moduleB := module.New(nil)

	moduleA.Controllers(controllerA)
	moduleA.Imports(moduleB)

	moduleB.Controllers(controllerB)
	moduleB.Imports(moduleA)

	doc := New("3.1.0", nil)

	done := make(chan struct{})
	go func() {
		Generate(doc, moduleA)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Generate did not terminate on circular module import")
	}

	if _, ok := doc.paths["/a/"]; !ok {
		t.Fatalf("moduleA's controller route missing, got %v", doc.paths)
	}
	if _, ok := doc.paths["/b/"]; !ok {
		t.Fatalf("moduleB's controller route missing, got %v", doc.paths)
	}
}

// TestGenerate_ResponseDescription_OverridesDefault proves
// response.Description(...) wins over both defaultErrorResponse's
// auto-derived http.StatusText description AND a schema-carrying
// Response's own empty default description.
func TestGenerate_ResponseDescription_OverridesDefault(t *testing.T) {
	okSchema := schema.New(reflect.TypeOf(addressEntity{}), 0)

	c := controller.New(func(c *controller.Controller) {
		c.Path("/posts")
		c.Route(route.HttpGet, "/:post_id", func(r *route.Route) {
			r.Response(200, func(response *route.Response) {
				response.Schema(okSchema)
				response.Description("The post")
			})
			r.Response(404, func(response *route.Response) {
				response.Description("Cannot find a post using post_id")
			})
		})
	})
	c.Declare()

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	op := doc.paths["/posts/:post_id"]["get"].(map[string]any)
	responses := op["responses"].(map[string]any)

	resp200 := responses["200"].(map[string]any)
	if resp200["description"] != "The post" {
		t.Fatalf("200 description = %v, want %q", resp200["description"], "The post")
	}

	resp404 := responses["404"].(map[string]any)
	if resp404["description"] != "Cannot find a post using post_id" {
		t.Fatalf("404 description = %v, want %q", resp404["description"], "Cannot find a post using post_id")
	}
	if _, ok := resp404["content"]; !ok {
		t.Fatalf("404 should still default to HttpException content, got %v", resp404)
	}
}

// TestGenerate_FormBody_MultipartFormDataWithFileField proves Route.FormBody
// documents the requestBody as multipart/form-data (not application/json,
// unlike RequestBody) -- an object schema combining m's own properties
// (keyed by their "form" tag) with one {"type":"string","format":"binary"}
// property per fileFields entry, inline (never registered/$ref'd into
// components.schemas, unlike a JSON body's whole schema).
func TestGenerate_FormBody_MultipartFormDataWithFileField(t *testing.T) {
	type uploadForm struct {
		Description string `form:"description"`
	}
	var f uploadForm
	m := schema.New(reflect.TypeOf(f), reflect.ValueOf(&f).Pointer())
	t.Cleanup(func() { schema.Deregister(reflect.TypeOf(f)) })
	m.Property(&f.Description).String().Required()

	c := controller.New(func(c *controller.Controller) {
		c.Path("/posts")
		c.Route(route.HttpPost, "/:post_id/attachment", func(r *route.Route) {
			r.FormBody(m, "file")
		})
	})
	c.Declare()

	root := module.New(func(mod *module.Module) {
		mod.Controllers(c)
	})
	if _, err := root.Assemble(); err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	doc := New("3.1.0", nil)
	Generate(doc, root)

	op := doc.paths["/posts/:post_id/attachment"]["post"].(map[string]any)
	reqBody, ok := op["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("requestBody missing, got %v", op)
	}
	content, ok := reqBody["content"].(map[string]any)
	if !ok {
		t.Fatalf("requestBody.content missing, got %v", reqBody)
	}
	multipart, ok := content["multipart/form-data"].(map[string]any)
	if !ok {
		t.Fatalf("expected multipart/form-data content, got %v", content)
	}
	if _, isJSON := content["application/json"]; isJSON {
		t.Fatalf("expected NO application/json content alongside multipart/form-data, got %v", content)
	}

	bodySchema, ok := multipart["schema"].(map[string]any)
	if !ok {
		t.Fatalf("multipart/form-data.schema missing, got %v", multipart)
	}
	if bodySchema["type"] != "object" {
		t.Fatalf("schema.type = %v, want %q", bodySchema["type"], "object")
	}
	properties, ok := bodySchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema.properties missing, got %v", bodySchema)
	}

	descProp, ok := properties["description"].(map[string]any)
	if !ok {
		t.Fatalf("expected a %q property, got %v", "description", properties)
	}
	if descProp["type"] != "string" {
		t.Fatalf("description.type = %v, want %q", descProp["type"], "string")
	}

	fileProp, ok := properties["file"].(map[string]any)
	if !ok {
		t.Fatalf("expected a %q property, got %v", "file", properties)
	}
	if fileProp["type"] != "string" || fileProp["format"] != "binary" {
		t.Fatalf("file property = %v, want {type: string, format: binary}", fileProp)
	}

	// The form schema must NOT have been registered into components.schemas
	// -- it's inline, request-specific, unlike a JSON body's own $ref'd shape.
	for name := range doc.schemas {
		if name == "uploadForm" {
			t.Fatalf("form body schema should not be registered into components.schemas, found %q", name)
		}
	}
}
