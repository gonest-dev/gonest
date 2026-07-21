package app

import "testing"

func TestAppOptions_ZeroValue(t *testing.T) {
	var opts Options
	if opts.BufferLogs != false {
		t.Fatalf("zero-value Options.BufferLogs = %v, want false", opts.BufferLogs)
	}
	if opts.LogLevels != nil {
		t.Fatalf("zero-value Options.LogLevels = %v, want nil", opts.LogLevels)
	}
}

func TestOnListen_NilSafe(t *testing.T) {
	var f OnListen = nil
	called := false
	if f != nil {
		f()
		called = true
	}
	if called {
		t.Fatalf("nil OnListen must not be called")
	}
}

func TestOnListen_Invocable(t *testing.T) {
	called := false
	var f OnListen = func() { called = true }
	if f != nil {
		f()
	}
	if !called {
		t.Fatalf("non-nil OnListen must be invoked")
	}
}
