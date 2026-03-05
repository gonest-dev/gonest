package validator

import (
	"context"
	"reflect"
	"strings"
)

// Schema creates a validation schema using a cleaner callback API
func Schema[T any](callback func(*T, *SchemaBuilder[T])) *SchemaType[T] {
	var instance T
	builder := NewSchema(&instance)
	callback(&instance, builder)
	return builder.Build()
}

// SchemaType defines validation rules for a type
type SchemaType[T any] struct {
	fields          []fieldValidator[T]
	crossValidators []func(*T) *FieldError
	async           bool
}

// fieldValidator is an interface that can validate a field of type T
type fieldValidator[T any] interface {
	validate(*T) *FieldError
	validateAsync(context.Context, *T) *FieldError
	hasAsync() bool
}

// SchemaBuilder builds validation schemas in a type-safe way
type SchemaBuilder[T any] struct {
	schema     *SchemaType[T]
	structType reflect.Type
	instance   *T
}

// NewSchema creates a new schema builder for a pointer to struct
func NewSchema[T any](instance *T) *SchemaBuilder[T] {
	// Get the actual struct type (not pointer)
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		panic("NewSchema requires a pointer to struct")
	}

	return &SchemaBuilder[T]{
		schema: &SchemaType[T]{
			fields:          make([]fieldValidator[T], 0),
			crossValidators: make([]func(*T) *FieldError, 0),
		},
		structType: t,
		instance:   instance,
	}
}

// fieldValidatorImpl implements fieldValidator for a specific field type
type fieldValidatorImpl[T any, F any] struct {
	fieldName      string
	displayName    string
	fieldIndex     int
	validators     []Validator[F]
	asyncValidator ContextValidator[F]
}

func (fv *fieldValidatorImpl[T, F]) validate(value *T) *FieldError {
	v := reflect.ValueOf(value).Elem()
	fieldValue := v.Field(fv.fieldIndex).Interface().(F)

	for _, validator := range fv.validators {
		if err := validator(fieldValue); err != nil {
			if err.field == "" {
				err.field = fv.displayName
			}
			return err
		}
	}

	return nil
}

func (fv *fieldValidatorImpl[T, F]) validateAsync(ctx context.Context, value *T) *FieldError {
	if err := fv.validate(value); err != nil {
		return err
	}

	if fv.asyncValidator != nil {
		v := reflect.ValueOf(value).Elem()
		fieldValue := v.Field(fv.fieldIndex).Interface().(F)

		if err := fv.asyncValidator(ctx, fieldValue); err != nil {
			if err.field == "" {
				err.field = fv.displayName
			}
			return err
		}
	}

	return nil
}

func (fv *fieldValidatorImpl[T, F]) hasAsync() bool {
	return fv.asyncValidator != nil
}

// getFieldInfoFromPointer extracts field info from a field pointer
func (sb *SchemaBuilder[T]) getFieldInfoFromPointer(fieldPtr any) (int, string, reflect.Type) {
	// Get the unsafe pointer address of the field
	fieldPtrValue := reflect.ValueOf(fieldPtr)
	if fieldPtrValue.Kind() != reflect.Ptr {
		panic("fieldPtr must be a pointer")
	}

	fieldAddr := fieldPtrValue.Pointer()

	// Get the base struct value
	instanceValue := reflect.ValueOf(sb.instance).Elem()

	// Iterate through fields to find matching address
	for i := 0; i < sb.structType.NumField(); i++ {
		field := sb.structType.Field(i)
		fieldValue := instanceValue.Field(i)

		// Get the address of this field
		if fieldValue.CanAddr() {
			fieldValueAddr := fieldValue.Addr().UnsafePointer()

			if uintptr(fieldValueAddr) == fieldAddr {
				// Found the field!
				displayName := field.Name

				// Check for json tag
				if jsonTag := field.Tag.Get("json"); jsonTag != "" {
					parts := strings.Split(jsonTag, ",")
					if parts[0] != "" && parts[0] != "-" {
						displayName = parts[0]
					}
				}

				return i, displayName, field.Type
			}
		}
	}

	panic("field not found in struct")
}

// Field adds a field validator using the field pointer directly
func (sb *SchemaBuilder[T]) Field(fieldPtr any, validators ...any) *SchemaBuilder[T] {
	fieldIndex, displayName, fieldType := sb.getFieldInfoFromPointer(fieldPtr)

	// Create the appropriate validator based on field type
	switch fieldType.Kind() {
	case reflect.String:
		sb.addStringField(fieldIndex, displayName, validators)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sb.addIntField(fieldIndex, displayName, validators)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		sb.addUintField(fieldIndex, displayName, validators)
	case reflect.Float32, reflect.Float64:
		sb.addFloatField(fieldIndex, displayName, validators)
	case reflect.Bool:
		sb.addBoolField(fieldIndex, displayName, validators)
	case reflect.Slice:
		sb.addSliceField(fieldIndex, displayName, fieldType, validators)
	default:
		panic("unsupported field type: " + fieldType.String())
	}

	return sb
}

func (sb *SchemaBuilder[T]) addStringField(fieldIndex int, displayName string, validators []any) {
	typedValidators := make([]Validator[string], len(validators))
	for i, v := range validators {
		typedValidators[i] = v.(Validator[string])
	}

	fv := &fieldValidatorImpl[T, string]{
		fieldIndex:  fieldIndex,
		displayName: displayName,
		validators:  typedValidators,
	}
	sb.schema.fields = append(sb.schema.fields, fv)
}

func (sb *SchemaBuilder[T]) addIntField(fieldIndex int, displayName string, validators []any) {
	typedValidators := make([]Validator[int], len(validators))
	for i, v := range validators {
		typedValidators[i] = v.(Validator[int])
	}

	fv := &fieldValidatorImpl[T, int]{
		fieldIndex:  fieldIndex,
		displayName: displayName,
		validators:  typedValidators,
	}
	sb.schema.fields = append(sb.schema.fields, fv)
}

func (sb *SchemaBuilder[T]) addUintField(fieldIndex int, displayName string, validators []any) {
	typedValidators := make([]Validator[uint], len(validators))
	for i, v := range validators {
		typedValidators[i] = v.(Validator[uint])
	}

	fv := &fieldValidatorImpl[T, uint]{
		fieldIndex:  fieldIndex,
		displayName: displayName,
		validators:  typedValidators,
	}
	sb.schema.fields = append(sb.schema.fields, fv)
}

func (sb *SchemaBuilder[T]) addFloatField(fieldIndex int, displayName string, validators []any) {
	typedValidators := make([]Validator[float64], len(validators))
	for i, v := range validators {
		typedValidators[i] = v.(Validator[float64])
	}

	fv := &fieldValidatorImpl[T, float64]{
		fieldIndex:  fieldIndex,
		displayName: displayName,
		validators:  typedValidators,
	}
	sb.schema.fields = append(sb.schema.fields, fv)
}

func (sb *SchemaBuilder[T]) addBoolField(fieldIndex int, displayName string, validators []any) {
	typedValidators := make([]Validator[bool], len(validators))
	for i, v := range validators {
		typedValidators[i] = v.(Validator[bool])
	}

	fv := &fieldValidatorImpl[T, bool]{
		fieldIndex:  fieldIndex,
		displayName: displayName,
		validators:  typedValidators,
	}
	sb.schema.fields = append(sb.schema.fields, fv)
}

func (sb *SchemaBuilder[T]) addSliceField(fieldIndex int, displayName string, fieldType reflect.Type, validators []any) {
	elemType := fieldType.Elem()

	switch elemType.Kind() {
	case reflect.String:
		typedValidators := make([]Validator[[]string], len(validators))
		for i, v := range validators {
			typedValidators[i] = v.(Validator[[]string])
		}
		fv := &fieldValidatorImpl[T, []string]{
			fieldIndex:  fieldIndex,
			displayName: displayName,
			validators:  typedValidators,
		}
		sb.schema.fields = append(sb.schema.fields, fv)
	case reflect.Int:
		typedValidators := make([]Validator[[]int], len(validators))
		for i, v := range validators {
			typedValidators[i] = v.(Validator[[]int])
		}
		fv := &fieldValidatorImpl[T, []int]{
			fieldIndex:  fieldIndex,
			displayName: displayName,
			validators:  typedValidators,
		}
		sb.schema.fields = append(sb.schema.fields, fv)
	default:
		panic("unsupported slice element type: " + elemType.String())
	}
}

// CrossField adds a cross-field validator
func (sb *SchemaBuilder[T]) CrossField(validator func(*T) *FieldError) *SchemaBuilder[T] {
	sb.schema.crossValidators = append(sb.schema.crossValidators, validator)
	return sb
}

// Build creates the final schema
func (sb *SchemaBuilder[T]) Build() *SchemaType[T] {
	return sb.schema
}

// Validate validates a value against the schema
func (s *SchemaType[T]) Validate(value *T) *ValidationResult {
	result := NewValidationResult()

	for _, field := range s.fields {
		if err := field.validate(value); err != nil {
			result.AddError(err)
		}
	}

	for _, validator := range s.crossValidators {
		if err := validator(value); err != nil {
			result.AddError(err)
		}
	}

	return result
}

// ValidateAsync validates a value asynchronously
func (s *SchemaType[T]) ValidateAsync(ctx context.Context, value *T) *ValidationResult {
	result := NewValidationResult()

	for _, field := range s.fields {
		if err := field.validateAsync(ctx, value); err != nil {
			result.AddError(err)
		}
	}

	for _, validator := range s.crossValidators {
		if err := validator(value); err != nil {
			result.AddError(err)
		}
	}

	return result
}

// HasAsync returns whether the schema has async validators
func (s *SchemaType[T]) HasAsync() bool {
	return s.async
}
