package pipe_test

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/pipe"
)

// TestNewDoesNotExecuteFn proves pipe.New(fn) defers fn -- same pattern as
// provider.New/controller.New/module.New. If fn ran eagerly, the flag would
// already be true right after New returns.
func TestNewDoesNotExecuteFn(t *testing.T) {
	ran := false
	p := pipe.New(func(p *pipe.Pipe) {
		ran = true
	})

	if p == nil {
		t.Fatal("expected New to return a non-nil *Pipe")
	}
	if ran {
		t.Fatal("expected fn passed to New not to run until explicitly invoked")
	}
}

// TestHandlerAcceptsValidSignatureInt proves Handler accepts
// func(ctx *httpctx.Context, raw string) T for T = int.
func TestHandlerAcceptsValidSignatureInt(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) int {
			return 0
		})
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic for valid signature, got: %v", r)
		}
	}()
	p.Declare()
}

// TestHandlerAcceptsValidSignatureString proves Handler accepts the same
// shape for a different T (string), confirming the validation is generic
// over T, not hardcoded to one return type.
func TestHandlerAcceptsValidSignatureString(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) string {
			return raw
		})
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic for valid signature, got: %v", r)
		}
	}()
	p.Declare()
}

// TestHandlerPanicsOnWrongParamCount proves Handler panics at declaration
// time (clear message) when fn doesn't take exactly (ctx, raw string).
func TestHandlerPanicsOnWrongParamCount(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context) int {
			return 0
		})
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Handler signature, got none")
		}
	}()
	p.Declare()
}

// TestHandlerPanicsOnWrongParamTypes proves Handler panics when the param
// types don't match (ctx *httpctx.Context, raw string).
func TestHandlerPanicsOnWrongParamTypes(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(raw string, ctx *httpctx.Context) int {
			return 0
		})
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Handler signature, got none")
		}
	}()
	p.Declare()
}

// TestHandlerPanicsOnWrongReturnCount proves Handler panics when fn returns
// zero or more than one value.
func TestHandlerPanicsOnWrongReturnCount(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) (int, error) {
			return 0, nil
		})
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Handler signature, got none")
		}
	}()
	p.Declare()
}

// TestHandlerPanicsOnNonFunc proves Handler panics when fn isn't a func at
// all.
func TestHandlerPanicsOnNonFunc(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(42)
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-func Handler argument, got none")
		}
	}()
	p.Declare()
}
