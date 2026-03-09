package config_test

import (
	"os"
	"testing"

	"github.com/gonest-dev/gonest/modules/config"
)

func TestConfigModule(t *testing.T) {
	// Setup temporary .env file
	content := "APP_NAME=GoNestApp\nPORT=3000"
	err := os.WriteFile(".env.config.test", []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(".env.config.test")

	// Initialize module
	_ = config.ForRoot(&config.Options{EnvFiles: []string{".env.config.test"}})

	service := config.NewConfigService()

	// Test Get (string by default)
	if v := service.Get("APP_NAME"); v != "GoNestApp" {
		t.Errorf("expected GoNestApp, got %v", v)
	}

	// Test GetConfig
	if v := config.GetConfig[int](service, "PORT", 0); v != 3000 {
		t.Errorf("expected 3000, got %d", v)
	}
}


