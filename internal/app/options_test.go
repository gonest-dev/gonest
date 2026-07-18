package app

import "testing"

func TestLogLevelError_String(t *testing.T) {
	got := LogLevelError.String()
	want := "Error"
	if got != want {
		t.Fatalf("LogLevelError.String() = %q, want %q", got, want)
	}
}

func TestLogLevelWarn_String(t *testing.T) {
	got := LogLevelWarn.String()
	want := "Warn"
	if got != want {
		t.Fatalf("LogLevelWarn.String() = %q, want %q", got, want)
	}
}

func TestLogLevelLog_String(t *testing.T) {
	got := LogLevelLog.String()
	want := "Log"
	if got != want {
		t.Fatalf("LogLevelLog.String() = %q, want %q", got, want)
	}
}

func TestLogLevelDebug_String(t *testing.T) {
	got := LogLevelDebug.String()
	want := "Debug"
	if got != want {
		t.Fatalf("LogLevelDebug.String() = %q, want %q", got, want)
	}
}

func TestLogLevelVerbose_String(t *testing.T) {
	got := LogLevelVerbose.String()
	want := "Verbose"
	if got != want {
		t.Fatalf("LogLevelVerbose.String() = %q, want %q", got, want)
	}
}

func TestLogLevel_String_Unknown(t *testing.T) {
	unknown := LogLevel(999)
	got := unknown.String()
	want := "Unknown"
	if got != want {
		t.Fatalf("LogLevel(999).String() = %q, want %q", got, want)
	}
}

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
