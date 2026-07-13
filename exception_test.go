package gonest

import (
	"net/http"
	"testing"
)

// FooExampleError reproduces INSIGHT.md's dev-defined-exception example
// (around line 130) verbatim, adapted per SPEC_DEVIATION: INSIGHT.md uses
// gonest.HttpStatusBadRequest, a named HttpStatus constant that design.md
// explicitly scoped OUT of this feature (see spec.md's Out of Scope), so
// this test uses the equivalent net/http.StatusBadRequest int literal
// instead.
type FooExampleError struct {
	HttpException
}

// NewFooExampleError mirrors INSIGHT.md's constructor shape exactly, using
// the root-aliased HttpException/NewHttpException.
func NewFooExampleError(details any) *FooExampleError {
	return &FooExampleError{
		HttpException: NewHttpException(http.StatusBadRequest, "FooExampleError", "lorem ipsum dolor met", details),
	}
}

// TestFooExampleError_RootAlias_InsightCallShape proves INSIGHT.md's
// dev-defined-exception example compiles and works end-to-end through the
// root gonest package's Exception/HttpException/NewHttpException aliases.
func TestFooExampleError_RootAlias_InsightCallShape(t *testing.T) {
	err := NewFooExampleError(map[string]any{"field": "bar"})

	if err.Status() != http.StatusBadRequest {
		t.Fatalf("Status() = %d, want %d", err.Status(), http.StatusBadRequest)
	}
	if err.Name() != "FooExampleError" {
		t.Fatalf("Name() = %q, want %q", err.Name(), "FooExampleError")
	}
	if err.Message() != "lorem ipsum dolor met" {
		t.Fatalf("Message() = %q, want %q", err.Message(), "lorem ipsum dolor met")
	}

	var _ Exception = err
}

// TestNewNotFoundException_RootAlias_PanicRecoverRoundTrip proves a
// root-aliased built-in exception constructor (NewNotFoundException) can be
// panicked with and recovered via a type assertion back to
// *NotFoundException through the root gonest package.
func TestNewNotFoundException_RootAlias_PanicRecoverRoundTrip(t *testing.T) {
	defer func() {
		r := recover()
		exc, ok := r.(*NotFoundException)
		if !ok {
			t.Fatalf("recover() type assertion to *NotFoundException failed, got %T", r)
		}
		if exc.Status() != http.StatusNotFound {
			t.Fatalf("Status() = %d, want %d", exc.Status(), http.StatusNotFound)
		}
	}()

	panic(NewNotFoundException(map[string]any{"userId": "abc123"}))
}

// TestHttpException_RootAlias_SatisfiesException proves the root-aliased
// HttpException/Exception types keep their structural-satisfaction
// relationship: a type embedding gonest.HttpException satisfies
// gonest.Exception without ever naming it.
func TestHttpException_RootAlias_SatisfiesException(t *testing.T) {
	var exc Exception = NewFooExampleError(nil)

	if exc.Details() != nil {
		t.Fatalf("Details() = %v, want nil", exc.Details())
	}
}
