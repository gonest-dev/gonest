package validate

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

type graphqlArgsEntity struct {
	Id     int64   `json:"id"`
	Amount float64 `json:"amount"`
}

func newGraphqlArgsTestSchema(t *testing.T) (*graphqlArgsEntity, *schema.Schema) {
	t.Helper()
	zero := &graphqlArgsEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestNewGraphqlArgsSource_NormalizesNativeIntArg is a regression test for
// a real bug found via a live .examples/blog-graphql dispatch: graphql-go
// decodes a GraphQL `Int` literal argument into a native Go `int`
// (confirmed via its own coerceInt/ParseLiteral source, not fabricated),
// but validatePrimitive's "integer"/"number" case does a hard
// `raw.(float64)` assertion -- the SAME shape encoding/json's own decode
// produces for REST JSON bodies. Left unnormalized, EVERY integer/float
// GraphQL arg failed validation with a silent "expected number" violation
// (surfacing as an EMPTY BadRequestException message, since
// NewBadRequestException's default Message() is "" when never set).
func TestNewGraphqlArgsSource_NormalizesNativeIntArg(t *testing.T) {
	zero, m := newGraphqlArgsTestSchema(t)
	m.Property(&zero.Id).Integer().Required()
	m.Property(&zero.Amount).Float().Required()

	// int and float32 -- exactly the shapes graphql-go's own Int/Float
	// scalar coercion produces, NOT the float64 encoding/json would.
	src := NewGraphqlArgsSource(map[string]any{
		"id":     int(42),
		"amount": float32(3.5),
	})

	var dst graphqlArgsEntity
	if err := src.ParseInto(&dst, m); err != nil {
		t.Fatalf("ParseInto() error = %v, want nil (native int/float32 args must normalize to the shape validatePrimitive expects)", err)
	}
	if dst.Id != 42 {
		t.Fatalf("dst.Id = %d, want 42", dst.Id)
	}
	if dst.Amount != 3.5 {
		t.Fatalf("dst.Amount = %v, want 3.5", dst.Amount)
	}
}

// TestNewGraphqlArgsSource_NormalizesNestedIntArgs proves normalization
// recurses into nested maps/slices too (an arg that is itself an object or
// array of objects, each carrying native int/float32 values).
func TestNewGraphqlArgsSource_NormalizesNestedIntArgs(t *testing.T) {
	got := normalizeGraphqlValue(map[string]any{
		"list": []any{
			map[string]any{"n": int(7)},
			int32(9),
		},
		"f": float32(1.5),
	})

	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("normalizeGraphqlValue() = %T, want map[string]any", got)
	}
	list, ok := m["list"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("m[\"list\"] = %#v, want a 2-element []any", m["list"])
	}
	nested, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("list[0] = %T, want map[string]any", list[0])
	}
	if _, ok := nested["n"].(float64); !ok {
		t.Fatalf("nested[\"n\"] = %T, want float64", nested["n"])
	}
	if _, ok := list[1].(float64); !ok {
		t.Fatalf("list[1] = %T, want float64", list[1])
	}
	if _, ok := m["f"].(float64); !ok {
		t.Fatalf("m[\"f\"] = %T, want float64", m["f"])
	}
}
