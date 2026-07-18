package graphqlgen_test

import (
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/graphqlgen"
	"gonest.dev/gonest/internal/schema"
)

type scalarEntity struct {
	Email1 string
	Email2 string
	Plain  string
}

func newScalarTestSchema(t *testing.T) (*scalarEntity, *schema.Schema) {
	t.Helper()
	zero := &scalarEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(reflect.ValueOf(zero).Pointer()))
	return zero, m
}

func TestNativeScalarName_KnownFormat(t *testing.T) {
	name, ok := graphqlgen.NativeScalarName("email")
	if !ok || name != "Email" {
		t.Fatalf("NativeScalarName(%q) = (%q, %v), want (Email, true)", "email", name, ok)
	}
}

func TestNativeScalarName_BareStringHasNoScalar(t *testing.T) {
	if _, ok := graphqlgen.NativeScalarName(""); ok {
		t.Fatal("NativeScalarName(\"\") = ok, want false (bare string has no custom scalar)")
	}
}

func TestCollectScalars_DedupsByName_TwoFieldsSameFormat(t *testing.T) {
	zero, m := newScalarTestSchema(t)
	m.Property(&zero.Email1).Email()
	m.Property(&zero.Email2).Email()
	m.Property(&zero.Plain).String()

	got := graphqlgen.CollectScalars(m.OwnProperties())
	if len(got) != 1 || got[0] != "Email" {
		t.Fatalf("CollectScalars() = %v, want exactly [Email]", got)
	}
}
