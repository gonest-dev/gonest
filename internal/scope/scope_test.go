package scope

import "testing"

func TestSingleton_String(t *testing.T) {
	got := Singleton.String()
	want := "Singleton"
	if got != want {
		t.Fatalf("Singleton.String() = %q, want %q", got, want)
	}
}

func TestTransient_String(t *testing.T) {
	got := Transient.String()
	want := "Transient"
	if got != want {
		t.Fatalf("Transient.String() = %q, want %q", got, want)
	}
}

func TestRequest_String(t *testing.T) {
	got := Request.String()
	want := "Request"
	if got != want {
		t.Fatalf("Request.String() = %q, want %q", got, want)
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
