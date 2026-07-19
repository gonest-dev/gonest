package dotenv

import (
	"errors"
	"strings"
	"testing"
)

func TestParseFile_BlankLine_Skipped(t *testing.T) {
	pairs, err := parseFile([]byte("FOO=bar\n\nBAZ=qux\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d: %+v", len(pairs), pairs)
	}
	if pairs[0] != (envPair{Key: "FOO", Value: "bar"}) {
		t.Errorf("pairs[0] = %+v, want FOO=bar", pairs[0])
	}
	if pairs[1] != (envPair{Key: "BAZ", Value: "qux"}) {
		t.Errorf("pairs[1] = %+v, want BAZ=qux", pairs[1])
	}
}

func TestParseFile_WholeLineComment_Skipped(t *testing.T) {
	pairs, err := parseFile([]byte("# this is a comment\nFOO=bar\n  # indented comment\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d: %+v", len(pairs), pairs)
	}
	if pairs[0] != (envPair{Key: "FOO", Value: "bar"}) {
		t.Errorf("pairs[0] = %+v, want FOO=bar", pairs[0])
	}
}

func TestParseFile_MissingEquals_ReturnsError(t *testing.T) {
	_, err := parseFile([]byte("FOO=bar\nNOEQUALSHERE\n"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not identify line 2", err.Error())
	}
	if !strings.Contains(err.Error(), "NOEQUALSHERE") {
		t.Errorf("error %q does not contain offending line content", err.Error())
	}
}

func TestParseFile_CrLf_Tolerated(t *testing.T) {
	pairs, err := parseFile([]byte("FOO=bar\r\nBAZ=qux\r\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d: %+v", len(pairs), pairs)
	}
	if pairs[1].Value != "qux" {
		t.Errorf("pairs[1].Value = %q, want %q (no stray \\r)", pairs[1].Value, "qux")
	}
}

func TestParseValue_Bare_ExtractsRaw(t *testing.T) {
	value, err := parseValue("bar", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_SingleQuoted_StripsQuotes(t *testing.T) {
	value, err := parseValue("'bar'", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_DoubleQuoted_StripsQuotes(t *testing.T) {
	value, err := parseValue(`"bar"`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_Backtick_StripsBackticks(t *testing.T) {
	value, err := parseValue("`bar`", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_UnterminatedQuote_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"single", "'bar"},
		{"double", `"bar`},
		{"backtick", "`bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseValue(tt.raw, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, errUnterminatedQuote) {
				t.Errorf("error %v does not wrap errUnterminatedQuote", err)
			}
		})
	}
}

func TestParseFile_UnterminatedQuote_IdentifiesLine(t *testing.T) {
	_, err := parseFile([]byte("FOO=bar\nBAZ=\"unterminated\n"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not identify line 2", err.Error())
	}
}

func TestResolveInterpolation_BareDollarVar_ExpandsFromResolvedMap(t *testing.T) {
	resolved := map[string]string{"A": "hello"}
	got := resolveInterpolation("$A world", resolved)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}

	got = resolveInterpolation("${A} world", resolved)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestResolveInterpolation_DoubleQuoted_Expands(t *testing.T) {
	pairs, err := parseFile([]byte("A=hello\nB=\"${A} world\"\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d: %+v", len(pairs), pairs)
	}
	if pairs[1].Value != "hello world" {
		t.Errorf("pairs[1].Value = %q, want %q", pairs[1].Value, "hello world")
	}
}

func TestResolveInterpolation_ColonDashDefault_UnsetOrEmpty(t *testing.T) {
	// VAR entirely unset (not in resolved, not in real env).
	got := resolveInterpolation("${VAR:-default}", nil)
	if got != "default" {
		t.Errorf("unset: got %q, want %q", got, "default")
	}

	// VAR set but empty.
	resolved := map[string]string{"VAR": ""}
	got = resolveInterpolation("${VAR:-default}", resolved)
	if got != "default" {
		t.Errorf("empty: got %q, want %q", got, "default")
	}

	// VAR set and non-empty -- default NOT used.
	resolved = map[string]string{"VAR": "value"}
	got = resolveInterpolation("${VAR:-default}", resolved)
	if got != "value" {
		t.Errorf("set: got %q, want %q", got, "value")
	}
}

func TestResolveInterpolation_DashDefault_OnlyUnsetNotEmpty(t *testing.T) {
	// VAR unset -- default used.
	got := resolveInterpolation("${VAR-default}", nil)
	if got != "default" {
		t.Errorf("unset: got %q, want %q", got, "default")
	}

	// VAR set but empty -- default NOT used, resolves to real (empty) value.
	resolved := map[string]string{"VAR": ""}
	got = resolveInterpolation("${VAR-default}", resolved)
	if got != "" {
		t.Errorf("empty-but-set: got %q, want empty string", got)
	}
}

func TestResolveInterpolation_ColonPlusAlternate_SetAndNonEmpty(t *testing.T) {
	resolved := map[string]string{"VAR": "actual"}
	got := resolveInterpolation("${VAR:+alt}", resolved)
	if got != "alt" {
		t.Errorf("got %q, want %q (not VAR's value)", got, "alt")
	}

	// VAR set but empty -- ":+" requires non-empty, so alt NOT used.
	resolved = map[string]string{"VAR": ""}
	got = resolveInterpolation("${VAR:+alt}", resolved)
	if got != "" {
		t.Errorf("empty: got %q, want empty string", got)
	}

	// VAR unset -- alt NOT used.
	got = resolveInterpolation("${VAR:+alt}", nil)
	if got != "" {
		t.Errorf("unset: got %q, want empty string", got)
	}
}

func TestResolveInterpolation_PlusAlternate_SetEvenEmpty(t *testing.T) {
	// VAR set but empty -- "+" (no ":") only requires "set", so alt IS used.
	resolved := map[string]string{"VAR": ""}
	got := resolveInterpolation("${VAR+alt}", resolved)
	if got != "alt" {
		t.Errorf("empty-but-set: got %q, want %q", got, "alt")
	}

	// VAR unset -- alt NOT used.
	got = resolveInterpolation("${VAR+alt}", nil)
	if got != "" {
		t.Errorf("unset: got %q, want empty string", got)
	}
}

func TestResolveInterpolation_UndefinedNoOperator_ExpandsEmpty(t *testing.T) {
	got := resolveInterpolation("${NUNCA_DEFINIDA}", nil)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}

	got = resolveInterpolation("$NUNCA_DEFINIDA", nil)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestParseFile_LaterLineReferencesEarlierKey_ResolvesRealValue(t *testing.T) {
	pairs, err := parseFile([]byte("A=hello\nB=${A} world\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d: %+v", len(pairs), pairs)
	}
	if pairs[1].Value != "hello world" {
		t.Errorf("pairs[1].Value = %q, want %q", pairs[1].Value, "hello world")
	}
}
