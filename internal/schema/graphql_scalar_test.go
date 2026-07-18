package schema_test

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

type graphqlScalarEntity struct {
	ObjectID string
}

func newGraphqlScalarTestSchema(t *testing.T) (*graphqlScalarEntity, *schema.Schema) {
	t.Helper()
	zero := &graphqlScalarEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

func TestPropertyBuilder_GraphqlScalarValue_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newGraphqlScalarTestSchema(t)
	pb := m.Property(&zero.ObjectID)

	name, ok := pb.GraphqlScalarValue()
	if ok || name != "" {
		t.Fatalf("GraphqlScalarValue() = (%q, %v), want (\"\", false)", name, ok)
	}
}

func TestPropertyBuilder_GraphqlScalar_StoresName_RetrievableViaGraphqlScalarValue(t *testing.T) {
	zero, m := newGraphqlScalarTestSchema(t)
	pb := m.Property(&zero.ObjectID)

	sentinel := pb.GraphqlScalar("ObjectID")
	if sentinel != pb {
		t.Fatal("GraphqlScalar() should return the bare *PropertyBuilder itself, same precedent as Custom()")
	}

	name, ok := pb.GraphqlScalarValue()
	if !ok || name != "ObjectID" {
		t.Fatalf("GraphqlScalarValue() = (%q, %v), want (\"ObjectID\", true)", name, ok)
	}
}
