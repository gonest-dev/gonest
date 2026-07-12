package gonest

import "testing"

// Behavior of Scope itself is covered in internal/scope. This only smoke-tests
// that the root-level re-export (type alias + const aliases) actually points
// at the same values.
func TestScope(t *testing.T) {
	testCases := []struct {
		scope Scope
		want  string
	}{
		{scope: ScopeSingleton, want: "Singleton"},
		{scope: ScopeTransient, want: "Transient"},
		{scope: ScopeRequest, want: "Request"},
	}

	for _, tt := range testCases {
		if got := tt.scope.String(); got != tt.want {
			t.Fatalf("Scope(%v).String() = %q, want %q", tt.scope, got, tt.want)
		}
	}
}
