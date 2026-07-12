package gonest

import "testing"

func TestScopeSingleton_String(t *testing.T) {
	got := ScopeSingleton.String()
	want := "Singleton"
	if got != want {
		t.Fatalf("ScopeSingleton.String() = %q, want %q", got, want)
	}
}

func TestScopeTransient_String(t *testing.T) {
	got := ScopeTransient.String()
	want := "Transient"
	if got != want {
		t.Fatalf("ScopeTransient.String() = %q, want %q", got, want)
	}
}

func TestScopeRequest_String(t *testing.T) {
	got := ScopeRequest.String()
	want := "Request"
	if got != want {
		t.Fatalf("ScopeRequest.String() = %q, want %q", got, want)
	}
}

func TestScope_String_Unknown(t *testing.T) {
	unknown := Scope(999)
	got := unknown.String()
	want := "Unknown"
	if got != want {
		t.Fatalf("Scope(999).String() = %q, want %q", got, want)
	}
}
