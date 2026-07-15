package exception

import (
	"encoding/json"
	"errors"
	"testing"
)

// fooExampleError mirrors INSIGHT.md's exact dev-defined-exception pattern
// (`type FooExampleError struct { gonest.HttpException }`) -- a struct that
// embeds HttpException BY VALUE and does nothing else. It exists purely to
// prove Exception is satisfied structurally, with zero explicit
// implementation, by anything that embeds HttpException.
type fooExampleError struct {
	HttpException
}

func TestNewHttpException_AccessorsReturnWhatWasPassed(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		excName     string
		message     string
		details     any
		wantDetails any
	}{
		{
			name:        "typical values",
			status:      404,
			excName:     "NotFoundException",
			message:     "resource not found",
			details:     map[string]string{"id": "123"},
			wantDetails: map[string]string{"id": "123"},
		},
		{
			name:        "nil details accepted, not synthesized",
			status:      400,
			excName:     "BadRequestException",
			message:     "bad input",
			details:     nil,
			wantDetails: nil,
		},
		{
			name:        "zero status accepted without complaint",
			status:      0,
			excName:     "",
			message:     "",
			details:     nil,
			wantDetails: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewHttpException().SetStatus(tt.status).SetName(tt.excName).SetMessage(tt.message).SetDetails(tt.details)

			if got := e.Status(); got != tt.status {
				t.Errorf("Status() = %v, want %v", got, tt.status)
			}
			if got := e.Name(); got != tt.excName {
				t.Errorf("Name() = %v, want %v", got, tt.excName)
			}
			if got := e.Message(); got != tt.message {
				t.Errorf("Message() = %v, want %v", got, tt.message)
			}
			got := e.Details()
			if tt.wantDetails == nil {
				if got != nil {
					t.Errorf("Details() = %v, want nil", got)
				}
				return
			}
			gotMap, ok := got.(map[string]string)
			if !ok {
				t.Fatalf("Details() = %#v, want map[string]string", got)
			}
			wantMap := tt.wantDetails.(map[string]string)
			if len(gotMap) != len(wantMap) {
				t.Errorf("Details() = %v, want %v", gotMap, wantMap)
			}
			for k, v := range wantMap {
				if gotMap[k] != v {
					t.Errorf("Details()[%q] = %v, want %v", k, gotMap[k], v)
				}
			}
		})
	}
}

// TestFooExampleError_SatisfiesException proves that a locally-defined type
// embedding HttpException by value -- with no explicit method declarations
// of its own -- satisfies the Exception interface via promoted methods. This
// is a real type assertion, not "it compiles": var _ Exception = ... below
// would already fail to compile if promotion didn't work, and the runtime
// assertion confirms it holds for a value produced at runtime too.
func TestFooExampleError_SatisfiesException(t *testing.T) {
	var _ Exception = fooExampleError{}

	v := fooExampleError{
		HttpException: NewHttpException().SetStatus(404).SetName("FooExampleError").SetMessage("example"),
	}

	got, ok := any(v).(Exception)
	if !ok {
		t.Fatalf("fooExampleError does not satisfy Exception via promoted methods")
	}
	if got.Status() != 404 {
		t.Errorf("Status() = %v, want 404", got.Status())
	}
	if got.Name() != "FooExampleError" {
		t.Errorf("Name() = %v, want FooExampleError", got.Name())
	}
}

// TestNonExceptionValues_DoNotSatisfyException proves the negative: an
// ordinary error and a bare int -- the kinds of values a real panic() might
// carry -- do NOT satisfy Exception. This matters because future panic
// recovery code will discriminate "structured exception" from "anything
// else" purely via this type assertion.
func TestNonExceptionValues_DoNotSatisfyException(t *testing.T) {
	tests := []struct {
		name string
		v    any
	}{
		{"plain error", errors.New("x")},
		{"bare int", 42},
		{"bare string", "not an exception"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.v.(Exception); ok {
				t.Errorf("%#v unexpectedly satisfies Exception", tt.v)
			}
		})
	}
}

func TestNewHttpException_DefaultsStatusTo500(t *testing.T) {
	e := NewHttpException()
	if got := e.Status(); got != 500 {
		t.Fatalf("Status() = %d, want 500 (default)", got)
	}
	if got := e.Name(); got != "" {
		t.Fatalf("Name() = %q, want empty", got)
	}
	if got := e.Message(); got != "" {
		t.Fatalf("Message() = %q, want empty", got)
	}
	if got := e.Details(); got != nil {
		t.Fatalf("Details() = %v, want nil", got)
	}
}

func TestHttpException_SetMethods_ChainAndReturnIndependentCopies(t *testing.T) {
	base := NewHttpException()
	withStatus := base.SetStatus(404)
	withName := withStatus.SetName("Foo")

	if base.Status() != 500 {
		t.Fatalf("base.Status() = %d, want 500 (SetStatus must not mutate receiver)", base.Status())
	}
	if withStatus.Name() != "" {
		t.Fatalf("withStatus.Name() = %q, want empty (SetName on a different copy must not affect this one)", withStatus.Name())
	}
	if withName.Status() != 404 || withName.Name() != "Foo" {
		t.Fatalf("withName = {status:%d, name:%q}, want {404, Foo}", withName.Status(), withName.Name())
	}
}

func TestHttpException_MarshalJSON_OmitsStatusIncludesRest(t *testing.T) {
	e := NewHttpException().SetStatus(409).SetName("Foo").SetMessage("bar").SetDetails(map[string]string{"k": "v"})

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := decoded["status"]; ok {
		t.Fatalf("decoded body contains \"status\", want it omitted: %s", b)
	}
	if decoded["name"] != "Foo" || decoded["message"] != "bar" {
		t.Fatalf("decoded = %v, want name=Foo message=bar", decoded)
	}
}

func TestHttpException_MarshalJSON_PromotedThroughEmbedding(t *testing.T) {
	v := fooExampleError{HttpException: NewHttpException().SetName("FooExampleError").SetMessage("boom")}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(fooExampleError) error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["name"] != "FooExampleError" {
		t.Fatalf("decoded[name] = %v, want FooExampleError (MarshalJSON must be promoted through embedding)", decoded["name"])
	}
}

func TestEffectiveName_ReturnsSetNameWhenNonEmpty(t *testing.T) {
	e := fooExampleError{HttpException: NewHttpException().SetName("ExplicitName")}
	if got := EffectiveName(e); got != "ExplicitName" {
		t.Fatalf("EffectiveName() = %q, want ExplicitName", got)
	}
}

func TestEffectiveName_FallsBackToConcreteTypeName_WhenNameUnset(t *testing.T) {
	v := &fooExampleError{HttpException: NewHttpException()} // SetName never called
	if got := EffectiveName(v); got != "fooExampleError" {
		t.Fatalf("EffectiveName() = %q, want fooExampleName type name \"fooExampleError\"", got)
	}
}
