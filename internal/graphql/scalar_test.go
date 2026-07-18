package graphql_test

import (
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/graphql"
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
	name, ok := graphql.NativeScalarName("email")
	if !ok || name != "Email" {
		t.Fatalf("NativeScalarName(%q) = (%q, %v), want (Email, true)", "email", name, ok)
	}
}

func TestNativeScalarName_BareStringHasNoScalar(t *testing.T) {
	if _, ok := graphql.NativeScalarName(""); ok {
		t.Fatal("NativeScalarName(\"\") = ok, want false (bare string has no custom scalar)")
	}
}

func TestCollectScalars_DedupsByName_TwoFieldsSameFormat(t *testing.T) {
	zero, m := newScalarTestSchema(t)
	m.Property(&zero.Email1).Email()
	m.Property(&zero.Email2).Email()
	m.Property(&zero.Plain).String()

	got, err := graphql.CollectScalars(m.OwnProperties())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "Email" {
		t.Fatalf("CollectScalars() = %v, want exactly [Email]", got)
	}
}

type customScalarEntity struct {
	ObjectID1 string
	ObjectID2 string
	Unnamed   string
}

func newCustomScalarTestSchema(t *testing.T) (*customScalarEntity, *schema.Schema) {
	t.Helper()
	zero := &customScalarEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(reflect.ValueOf(zero).Pointer()))
	return zero, m
}

func TestCollectScalars_DedupsNamedGraphqlScalar_TwoFieldsSameName(t *testing.T) {
	zero, m := newCustomScalarTestSchema(t)
	decode := func(raw any) (any, error) { return raw, nil }
	m.Property(&zero.ObjectID1).Custom(decode).GraphqlScalar("ObjectID")
	m.Property(&zero.ObjectID2).Custom(decode).GraphqlScalar("ObjectID")

	got, err := graphql.CollectScalars(m.OwnProperties())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "ObjectID" {
		t.Fatalf("CollectScalars() = %v, want exactly [ObjectID]", got)
	}
}

func TestCollectScalars_CustomWithoutGraphqlScalar_ReturnsError(t *testing.T) {
	zero, m := newCustomScalarTestSchema(t)
	m.Property(&zero.Unnamed).Custom(func(raw any) (any, error) { return raw, nil })

	_, err := graphql.CollectScalars(m.OwnProperties())
	if err == nil {
		t.Fatal("expected an error for Custom(fn) without GraphqlScalar(name), got nil")
	}
}
