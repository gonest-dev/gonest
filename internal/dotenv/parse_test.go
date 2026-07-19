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
	value, err := parseValue("bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_SingleQuoted_StripsQuotes(t *testing.T) {
	value, err := parseValue("'bar'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_DoubleQuoted_StripsQuotes(t *testing.T) {
	value, err := parseValue(`"bar"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "bar" {
		t.Errorf("value = %q, want %q", value, "bar")
	}
}

func TestParseValue_Backtick_StripsBackticks(t *testing.T) {
	value, err := parseValue("`bar`")
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
			_, err := parseValue(tt.raw)
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
