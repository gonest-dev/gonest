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
