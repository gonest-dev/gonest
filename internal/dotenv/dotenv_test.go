package dotenv

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
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

// TestLoad_MultiplePaths_FirstPathWinsSameKey proves T7's precedence rule:
// when two paths both define the same key with different values, Load
// applies pathA's envPair first (calling os.Setenv), so pathB's LookupEnv
// check for the same key finds it already present and skips it -- the
// process ends up with pathA's value, not pathB's.
//
// NOT parallel (t.Parallel omitted deliberately) -- os.Setenv/os.Unsetenv is
// global process state, and this test shares the key namespace with the
// other Load precedence tests below.
func TestLoad_MultiplePaths_FirstPathWinsSameKey(t *testing.T) {
	const key = "GONEST_DOTENV_T7_FIRST_WINS"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.env")
	pathB := filepath.Join(dir, "b.env")
	if err := os.WriteFile(pathA, []byte(key+"=from-a\n"), 0o644); err != nil {
		t.Fatalf("failed to write a.env: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(key+"=from-b\n"), 0o644); err != nil {
		t.Fatalf("failed to write b.env: %v", err)
	}

	if err := Get().Load(pathA, pathB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv(key); got != "from-a" {
		t.Fatalf("expected %q (first path wins), got %q", "from-a", got)
	}
}

// TestLoad_PreExistingProcessEnv_WinsOverFile proves T7's second half: a key
// already set in the process environment BEFORE Load ever runs is never
// overwritten by a file value, since LookupEnv reports it present on the
// very first check.
func TestLoad_PreExistingProcessEnv_WinsOverFile(t *testing.T) {
	const key = "GONEST_DOTENV_T7_PREEXISTING_WINS"
	t.Setenv(key, "pre-existing")

	dir := t.TempDir()
	path := filepath.Join(dir, "a.env")
	if err := os.WriteFile(path, []byte(key+"=from-file\n"), 0o644); err != nil {
		t.Fatalf("failed to write a.env: %v", err)
	}

	if err := Get().Load(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv(key); got != "pre-existing" {
		t.Fatalf("expected pre-existing process env %q to win, got %q", "pre-existing", got)
	}
}

// TestLoad_KeyOnlyInSecondPath_IsSet proves a key present ONLY in the second
// path (not in the first path, not pre-existing) is set normally -- the
// "first path wins on conflict" rule must not accidentally suppress keys
// that never conflicted in the first place.
func TestLoad_KeyOnlyInSecondPath_IsSet(t *testing.T) {
	const key = "GONEST_DOTENV_T7_SECOND_PATH_ONLY"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.env")
	pathB := filepath.Join(dir, "b.env")
	if err := os.WriteFile(pathA, []byte("GONEST_DOTENV_T7_UNRELATED=x\n"), 0o644); err != nil {
		t.Fatalf("failed to write a.env: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(key+"=from-b\n"), 0o644); err != nil {
		t.Fatalf("failed to write b.env: %v", err)
	}

	if err := Get().Load(pathA, pathB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv(key); got != "from-b" {
		t.Fatalf("expected %q, got %q", "from-b", got)
	}
}

// TestLoad_FourInlineCommentLines_MatchesSpecExamples_ViaRealLoad is the
// pending assertion T5's own test left as a documented SPEC_DEVIATION: back
// then, Load parsed each path via parseFile but did not yet call os.Setenv,
// so os.Getenv couldn't observe anything. Now that T7 wires Setenv into
// Load, this test loads a real .env file (the same 4 literal lines from
// spec.md's "P1: Comentários inline") via the real Load and confirms all 4
// resolved values through os.Getenv -- the layer parseFile-level test above
// (TestLoad_FourInlineCommentLines_MatchesSpecExamples) stays as-is,
// covering the parsing layer; this one covers the full Load path.
func TestLoad_FourInlineCommentLines_MatchesSpecExamples_ViaRealLoad(t *testing.T) {
	const (
		key1 = "GONEST_DOTENV_T7_INLINE_VAR1"
		key2 = "GONEST_DOTENV_T7_INLINE_VAR2"
		key3 = "GONEST_DOTENV_T7_INLINE_VAR3"
		key4 = "GONEST_DOTENV_T7_INLINE_VAR4"
	)
	for _, k := range []string{key1, key2, key3, key4} {
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range []string{key1, key2, key3, key4} {
			os.Unsetenv(k)
		}
	})

	content := key1 + "=VAL # comment\n" +
		key2 + "=VAL# not a comment\n" +
		key3 + `="VAL # not a comment"` + "\n" +
		key4 + `="VAL" # comment` + "\n"

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	if err := Get().Load(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := map[string]string{
		key1: "VAL",
		key2: "VAL# not a comment",
		key3: "VAL # not a comment",
		key4: "VAL",
	}
	for key, want := range cases {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// --- T11 fixtures/tests -----------------------------------------------

// TestDotenv_SatisfiesParseable is a compile-time-checked-at-runtime guard
// mirroring the package-level `var _ execution.Parseable = (*Dotenv)(nil)`
// declared in dotenv.go -- proves *Dotenv is usable anywhere an
// execution.Parseable is expected (e.g. gonest.MustParse[T](gonest.Dotenv(),
// schema)).
func TestDotenv_SatisfiesParseable(t *testing.T) {
	var p execution.Parseable = Get()
	if p == nil {
		t.Fatal("expected Get() to satisfy execution.Parseable as a non-nil value")
	}
}

// envParseIntoFixture is a minimal env-tagged struct used to prove
// ParseInto delegates to validate.ParseEnvInto.
type envParseIntoFixture struct {
	Host string `env:"GONEST_DOTENV_T11_HOST"`
}

var envParseIntoSchema = func() *schema.Schema {
	f := &envParseIntoFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Host).String().Required()
	return m
}()

// TestDotenv_ParseInto_DelegatesToParseEnvInto proves (*Dotenv).ParseInto
// reads the real process environment (via validate.ParseEnvInto) and
// populates dst -- a one-line delegation, but the delegation itself must
// actually run.
func TestDotenv_ParseInto_DelegatesToParseEnvInto(t *testing.T) {
	t.Setenv("GONEST_DOTENV_T11_HOST", "db.internal")

	var dst envParseIntoFixture
	if err := Get().ParseInto(&dst, envParseIntoSchema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Host != "db.internal" {
		t.Fatalf("dst.Host = %q, want %q", dst.Host, "db.internal")
	}
}
