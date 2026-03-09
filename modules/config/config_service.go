package config

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gonest-dev/gonest/packages/env"
)

// ConfigService provides access to environment variables and configuration.
type ConfigService struct {
	Schema any
}

// NewConfigService creates a new instance of ConfigService.
func NewConfigService() *ConfigService {
	return &ConfigService{}
}

// Get retrieves a configuration value by key and converts it to string.
func (s *ConfigService) Get(key string, defaultValue ...string) string {
	return env.Get(key, defaultValue...)
}

// GetTyped retrieves a configuration value and converts it to T.
func (s *ConfigService) GetTyped(key string, defaultValue ...any) any {
	// This is a bit tricky with generics in methods.
	// We'll keep it simple for now or use the standalone GetTyped.
	return env.Get[string](key)
}

// Populate fills a struct with environment variables and validates it if a schema exists.
func (s *ConfigService) Populate(target any) error {
	// 1. Populate from env
	if err := env.Populate(target); err != nil {
		return err
	}

	// 2. Validate if schema is present
	if s.Schema != nil {
		schemaVal := reflect.ValueOf(s.Schema)
		validateMethod := schemaVal.MethodByName("Validate")
		if !validateMethod.IsValid() {
			return nil
		}

		// Call Validate(context.Background(), target)
		results := validateMethod.Call([]reflect.Value{
			reflect.ValueOf(context.Background()),
			reflect.ValueOf(target),
		})

		if len(results) > 0 {
			res := results[0].Interface()
			// res is *validator.ValidationResult
			// We check for Invalid()
			invalidMethod := reflect.ValueOf(res).MethodByName("Invalid")
			if invalidMethod.IsValid() {
				isInvalid := invalidMethod.Call(nil)[0].Bool()
				if isInvalid {
					return fmt.Errorf("config: validation failed: %v", res)
				}
			}
		}
	}

	return nil
}

// GetConfig retrieves a configuration value and converts it to T.
func GetConfig[T any](s *ConfigService, key string, defaultValue ...T) T {
	return env.Get(key, defaultValue...)
}


