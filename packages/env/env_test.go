package env

import (
	"os"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	// Setup temporary .env file
	content := `
PORT=8080
DB_HOST=localhost
DB_URL=postgres://${DB_HOST}:5432/mydb
DEBUG=true
TIMEOUT=10s
JSON_DATA={"id": 1, "name": "test"}
# Comment
  SPACED_KEY = value  
`
	err := os.WriteFile(".env.test", []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(".env.test")

	Load(".env.test")

	// Test string
	if v := Get[string]("DB_HOST"); v != "localhost" {
		t.Errorf("expected localhost, got %s", v)
	}

	// Test expansion
	if v := Get[string]("DB_URL"); v != "postgres://localhost:5432/mydb" {
		t.Errorf("expected postgres://localhost:5432/mydb, got %s", v)
	}

	// Test int
	if v := Get[int]("PORT"); v != 8080 {
		t.Errorf("expected 8080, got %d", v)
	}

	// Test bool
	if v := Get[bool]("DEBUG"); v != true {
		t.Errorf("expected true, got %v", v)
	}

	// Test duration
	if v := Get[time.Duration]("TIMEOUT"); v != 10*time.Second {
		t.Errorf("expected 10s, got %v", v)
	}

	// Test default
	if v := Get[int]("NON_EXISTENT", 123); v != 123 {
		t.Errorf("expected 123, got %d", v)
	}
}


