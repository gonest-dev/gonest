package swagger

import (
	"reflect"
	"strings"
)

// Descriptor creates a schema using a cleaner callback API
func Descriptor[T any](callback func(*T, *DescriptorBuilder[T])) *Schema {
	var instance T
	builder := NewDescriptor(&instance)
	callback(&instance, builder)
	return builder.Build()
}

// DescriptorBuilder builds OpenAPI schemas using field pointers
type DescriptorBuilder[T any] struct {
	instance   *T
	structType reflect.Type
	fields     map[string]*FieldDescriptor
}

// FieldDescriptor holds metadata for a single field
type FieldDescriptor struct {
	name         string
	jsonName     string
	fieldType    reflect.Type
	description  string
	required     bool
	format       string
	example      any
	minimum      *float64
	maximum      *float64
	minLength    *int
	maxLength    *int
	pattern      string
	enum         []any
	writeOnly    bool
	readOnly     bool
	deprecated   bool
	defaultValue any
}

// FieldDescriptorBuilder provides fluent API for field configuration
type FieldDescriptorBuilder struct {
	descriptor *FieldDescriptor
	builder    *DescriptorBuilder[any]
}

// NewDescriptor creates a new descriptor builder
func NewDescriptor[T any](instance *T) *DescriptorBuilder[T] {
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		panic("NewDescriptor requires a pointer to struct")
	}

	return &DescriptorBuilder[T]{
		instance:   instance,
		structType: t,
		fields:     make(map[string]*FieldDescriptor),
	}
}

// Field starts describing a field using its pointer
func (b *DescriptorBuilder[T]) Field(fieldPtr any) *FieldDescriptorBuilder {
	fieldName := b.getFieldNameFromPointer(fieldPtr)
	fieldIndex, jsonName, fieldType := b.getFieldInfo(fieldName)

	// Create or get existing descriptor
	descriptor, exists := b.fields[fieldName]
	if !exists {
		descriptor = &FieldDescriptor{
			name:      fieldName,
			jsonName:  jsonName,
			fieldType: fieldType,
		}
		b.fields[fieldName] = descriptor
	}

	_ = fieldIndex // Used for validation

	return &FieldDescriptorBuilder{
		descriptor: descriptor,
	}
}

// getFieldNameFromPointer extracts field name from pointer
func (b *DescriptorBuilder[T]) getFieldNameFromPointer(fieldPtr any) string {
	fieldPtrValue := reflect.ValueOf(fieldPtr)
	if fieldPtrValue.Kind() != reflect.Ptr {
		panic("fieldPtr must be a pointer")
	}

	fieldAddr := fieldPtrValue.Pointer()
	instanceValue := reflect.ValueOf(b.instance).Elem()

	for i := 0; i < b.structType.NumField(); i++ {
		field := b.structType.Field(i)
		fieldValue := instanceValue.Field(i)

		if fieldValue.CanAddr() {
			fieldValueAddr := fieldValue.Addr().UnsafePointer()

			if uintptr(fieldValueAddr) == fieldAddr {
				return field.Name
			}
		}
	}

	panic("field not found in struct")
}

// getFieldInfo gets field information by name
func (b *DescriptorBuilder[T]) getFieldInfo(fieldName string) (int, string, reflect.Type) {
	for i := 0; i < b.structType.NumField(); i++ {
		field := b.structType.Field(i)

		if field.Name == fieldName {
			jsonName := field.Name

			// Get json tag
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" && parts[0] != "-" {
					jsonName = parts[0]
				}
			}

			return i, jsonName, field.Type
		}
	}

	panic("field '" + fieldName + "' not found in struct")
}

// Build converts descriptors to OpenAPI Schema
func (b *DescriptorBuilder[T]) Build() *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
		Required:   []string{},
	}

	for _, fieldDesc := range b.fields {
		fieldSchema := fieldDesc.toSchema()
		schema.Properties[fieldDesc.jsonName] = fieldSchema

		if fieldDesc.required {
			schema.Required = append(schema.Required, fieldDesc.jsonName)
		}
	}

	return schema
}

// toSchema converts FieldDescriptor to Schema
func (fd *FieldDescriptor) toSchema() *Schema {
	schema := &Schema{
		Type:        getSchemaType(fd.fieldType),
		Format:      fd.format,
		Description: fd.description,
		Example:     fd.example,
		Pattern:     fd.pattern,
	}

	if fd.minimum != nil {
		schema.AdditionalProperties = map[string]any{"minimum": *fd.minimum}
	}
	if fd.maximum != nil {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["maximum"] = *fd.maximum
	}
	if fd.minLength != nil {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["minLength"] = *fd.minLength
	}
	if fd.maxLength != nil {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["maxLength"] = *fd.maxLength
	}
	if len(fd.enum) > 0 {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["enum"] = fd.enum
	}
	if fd.writeOnly {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["writeOnly"] = true
	}
	if fd.readOnly {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["readOnly"] = true
	}
	if fd.deprecated {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["deprecated"] = true
	}
	if fd.defaultValue != nil {
		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = make(map[string]any)
		}
		schema.AdditionalProperties.(map[string]any)["default"] = fd.defaultValue
	}

	// Handle arrays
	if fd.fieldType.Kind() == reflect.Slice || fd.fieldType.Kind() == reflect.Array {
		schema.Items = &Schema{
			Type: getSchemaType(fd.fieldType.Elem()),
		}
	}

	return schema
}

// getSchemaType converts Go type to OpenAPI type
func getSchemaType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct:
		return "object"
	case reflect.Ptr:
		return getSchemaType(t.Elem())
	default:
		return "object"
	}
}

// FieldDescriptorBuilder methods

// Description sets the field description
func (f *FieldDescriptorBuilder) Description(desc string) *FieldDescriptorBuilder {
	f.descriptor.description = desc
	return f
}

// Required marks the field as required
func (f *FieldDescriptorBuilder) Required() *FieldDescriptorBuilder {
	f.descriptor.required = true
	return f
}

// Format sets the field format (email, date-time, password, etc)
func (f *FieldDescriptorBuilder) Format(format string) *FieldDescriptorBuilder {
	f.descriptor.format = format
	return f
}

// Example sets an example value
func (f *FieldDescriptorBuilder) Example(ex any) *FieldDescriptorBuilder {
	f.descriptor.example = ex
	return f
}

// Minimum sets minimum value for numbers
func (f *FieldDescriptorBuilder) Minimum(min float64) *FieldDescriptorBuilder {
	f.descriptor.minimum = &min
	return f
}

// Maximum sets maximum value for numbers
func (f *FieldDescriptorBuilder) Maximum(max float64) *FieldDescriptorBuilder {
	f.descriptor.maximum = &max
	return f
}

// MinLength sets minimum length for strings
func (f *FieldDescriptorBuilder) MinLength(min int) *FieldDescriptorBuilder {
	f.descriptor.minLength = &min
	return f
}

// MaxLength sets maximum length for strings
func (f *FieldDescriptorBuilder) MaxLength(max int) *FieldDescriptorBuilder {
	f.descriptor.maxLength = &max
	return f
}

// Pattern sets regex pattern for strings
func (f *FieldDescriptorBuilder) Pattern(pattern string) *FieldDescriptorBuilder {
	f.descriptor.pattern = pattern
	return f
}

// Enum sets allowed values
func (f *FieldDescriptorBuilder) Enum(values ...any) *FieldDescriptorBuilder {
	f.descriptor.enum = values
	return f
}

// WriteOnly marks field as write-only (not returned in responses)
func (f *FieldDescriptorBuilder) WriteOnly() *FieldDescriptorBuilder {
	f.descriptor.writeOnly = true
	return f
}

// ReadOnly marks field as read-only (not accepted in requests)
func (f *FieldDescriptorBuilder) ReadOnly() *FieldDescriptorBuilder {
	f.descriptor.readOnly = true
	return f
}

// Deprecated marks field as deprecated
func (f *FieldDescriptorBuilder) Deprecated() *FieldDescriptorBuilder {
	f.descriptor.deprecated = true
	return f
}

// Default sets default value
func (f *FieldDescriptorBuilder) Default(value any) *FieldDescriptorBuilder {
	f.descriptor.defaultValue = value
	return f
}
