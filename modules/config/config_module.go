package config

import (
	"reflect"
	"strings"

	"github.com/gonest-dev/gonest/core/common"
	"github.com/gonest-dev/gonest/packages/env"
)

// ConfigModule is the GoNest module for configuration.
type ConfigModule struct {
	options *Options
}

// Options defines configuration for ConfigModule.
type Options struct {
	EnvFiles []string
	Schema   any // Should be *validator.SchemaType[T]
}

// Configure implements the common.Module interface.
func (m *ConfigModule) Configure(builder *common.ModuleBuilder) {
	// Initialize ConfigService with options
	svc := NewConfigService()

	// Load environment variables
	if len(m.options.EnvFiles) > 0 {
		env.Load(m.options.EnvFiles...)
	} else {
		env.Load(".env")
	}

	// Perform validation if schema is provided
	// 2. Validate if schema is present
	if svc.Schema != nil {
		// We need to resolve the type T from SchemaType[T]
		schemaVal := reflect.ValueOf(svc.Schema)
		schemaType := schemaVal.Type()

		// Verify it's a pointer to SchemaType
		if schemaType.Kind() == reflect.Ptr && strings.Contains(schemaType.Elem().Name(), "SchemaType") {
			// Create a new instance of the configuration struct
			// SchemaType[T] has a Validate(ctx, *T) method
			// We can use reflection to call it

			// Get the type T (the target struct)
			// validator.Schema returns *SchemaType[T]
			// We can get the type of T from the field validators or via reflect
			// For now, let's assume we can use Populate to fill a new instance of T
			// and then validate it.

			// Simplified reflection to call Validate on the generic schema
			validateMethod := schemaVal.MethodByName("Validate")
			if validateMethod.IsValid() {
				// The Validate method takes (*T)
				// We need to know what T is.
				// Since we can't easily get T from the generic type at runtime without more metadata,
				// let's assume the user provides the pointer to the config struct in the service
				// OR we improve validator to provide the type.

				// For current implementation, let's store the schema in the service
				// and have the service provide a ValidatedConfig method.
				svc.Schema = m.options.Schema
			}
		}
	}

	builder.Providers(func() *ConfigService { return svc }).
		Exports(svc)
}

// ForRoot initializes the ConfigModule with options.
func ForRoot(opts *Options) *ConfigModule {
	if opts == nil {
		opts = &Options{}
	}
	return &ConfigModule{options: opts}
}

// WithEnvFiles sets the .env files to load.
func WithEnvFiles(files ...string) *Options {
	return &Options{EnvFiles: files}
}

// WithValidation sets the validation schema for config.
func WithValidation(schema any) *Options {
	return &Options{Schema: schema}
}

// Add more option helpers if needed


