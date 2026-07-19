package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGet_ReturnsSameSingletonInstance(t *testing.T) {
	a := Get()
	b := Get()
	if a != b {
		t.Fatalf("expected Get() to return the same instance, got %p and %p", a, b)
	}
}

func TestLoad_NonexistentPath_ReturnsError(t *testing.T) {
	err := Get().Load("./path/does/not/exist.env")
	if err == nil {
		t.Fatal("expected Load with a nonexistent path to return a non-nil error")
	}
}

func TestMustLoad_NonexistentPath_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustLoad with a nonexistent path to panic")
		}
	}()

	Get().MustLoad("./path/does/not/exist.env")
}

// TestLoad_FourInlineCommentLines_MatchesSpecExamples mirrors, one per env
// var, the 4 literal lines from spec.md's "P1: Comentários inline" (each
// line uses a distinct key -- VAR1..VAR4 -- so the 4 examples can coexist in
// the same file; the VAL/comment text on each line is exactly as spec.md
// states it).
//
// SPEC_DEVIATION (T5): spec.md's Independent Test says to confirm the 4
// values via Load + os.Getenv. As of this task, Dotenv.Load (T1's skeleton)
// parses each path with parseFile but does NOT yet call os.Setenv -- that
// wiring is T7 ("Precedência entre paths + os.Environ() pré-existente"),
// which depends on T6, which depends on THIS task (T5). Asserting through
// os.Getenv here would therefore always read an empty string, regardless of
// whether comment-stripping is correct -- it would not test this task's
// behavior at all. This test instead calls parseFile directly (the actual
// entry point T5's new code lives behind) and asserts on the returned
// envPair values, preserving the spec's intent (4 literal lines, 1 file,
// all 4 rules proven together) without depending on T7's not-yet-written
// os.Setenv wiring. Once T7 lands, an equivalent Load+os.Getenv assertion
// can be added at that layer if desired.
func TestLoad_FourInlineCommentLines_MatchesSpecExamples(t *testing.T) {
	content := "VAR1=VAL # comment\n" +
		"VAR2=VAL# not a comment\n" +
		`VAR3="VAL # not a comment"` + "\n" +
		`VAR4="VAL" # comment` + "\n"

	pairs, err := parseFile([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []envPair{
		{Key: "VAR1", Value: "VAL"},
		{Key: "VAR2", Value: "VAL# not a comment"},
		{Key: "VAR3", Value: "VAL # not a comment"},
		{Key: "VAR4", Value: "VAL"},
	}
	if len(pairs) != len(want) {
		t.Fatalf("expected %d pairs, got %d: %+v", len(want), len(pairs), pairs)
	}
	for i, w := range want {
		if pairs[i] != w {
			t.Errorf("pairs[%d] = %+v, want %+v", i, pairs[i], w)
		}
	}
}

func TestLoad_ExistingEmptyPath_NoError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("failed to create empty test file: %v", err)
	}

	if err := Get().Load(path); err != nil {
		t.Fatalf("expected Load with an existing empty path to return no error, got: %v", err)
	}
}
