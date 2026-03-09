package env

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

var (
	envMutex sync.RWMutex
	envStore = map[string]string{}
)

// Load reads the first valid .env file from the provided paths.
func Load(filePaths ...string) {
	for _, path := range filePaths {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		if err := loadEnvFile(path); err == nil {
			return
		}
	}
}

// Get retrieves an environment variable and converts it to T.
// Returns default value if not found or conversion fails.
func Get[T any](key string, optionalDefault ...T) T {
	var zero T
	val, found := lookupEnv(key)
	if !found || strings.TrimSpace(val) == "" {
		if len(optionalDefault) > 0 {
			return optionalDefault[0]
		}
		return zero
	}

	converted, err := convertStringToType[T](val)
	if err != nil {
		if len(optionalDefault) > 0 {
			return optionalDefault[0]
		}
		return zero
	}
	return converted
}

// Populate fills a struct with environment variables based on 'env' and 'default' tags.
func Populate(target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("env: target must be a pointer to a struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct && fieldType.Anonymous {
			if err := Populate(field.Addr().Interface()); err != nil {
				return err
			}
			continue
		}

		envKey := fieldType.Tag.Get("env")
		if envKey == "" {
			continue
		}

		val, found := lookupEnv(envKey)
		if !found || strings.TrimSpace(val) == "" {
			// Try default value
			val = fieldType.Tag.Get("default")
			if val == "" {
				continue
			}
		}

		converted, err := convertToStringKind(val, field.Type())
		if err != nil {
			return fmt.Errorf("env: error converting field %s: %w", fieldType.Name, err)
		}

		field.Set(converted)
	}

	return nil
}

func lookupEnv(key string) (string, bool) {
	envMutex.RLock()
	defer envMutex.RUnlock()
	if v, ok := envStore[key]; ok {
		return v, true
	}
	return os.LookupEnv(key)
}

func expandValue(raw string) string {
	return os.Expand(raw, func(k string) string {
		v, _ := lookupEnv(k)
		return v
	})
}


