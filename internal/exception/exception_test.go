package exception

import (
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
			e := NewHttpException(tt.status, tt.excName, tt.message, tt.details)

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
		HttpException: NewHttpException(404, "FooExampleError", "example", nil),
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
