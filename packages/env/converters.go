package env

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// convertStringToType converts a string to the specified type.
func convertStringToType[T any](raw string) (T, error) {
	var zero T
	targetType := reflect.TypeFor[T]()
	val, err := convertToStringKind(raw, targetType)
	if err != nil {
		return zero, err
	}
	return val.Interface().(T), nil
}

// convertToStringKind converts a string to a reflect.Value of targetType.
func convertToStringKind(raw string, targetType reflect.Type) (reflect.Value, error) {
	zero := reflect.Zero(targetType)

	// Special Types
	switch targetType {
	case reflect.TypeFor[json.RawMessage]():
		if !json.Valid([]byte(raw)) {
			return zero, fmt.Errorf("env: invalid JSON")
		}
		return reflect.ValueOf(json.RawMessage(raw)), nil
	case reflect.TypeFor[time.Time]():
		t, err := parseTime(raw)
		return reflect.ValueOf(t), err
	case reflect.TypeFor[time.Duration]():
		d, err := time.ParseDuration(raw)
		return reflect.ValueOf(d), err
	}

	// Basic Kinds
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(raw), nil
	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		return reflect.ValueOf(v), err
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(raw, 10, targetType.Bits())
		if err != nil {
			return zero, err
		}
		return reflect.ValueOf(v).Convert(targetType), nil
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(raw, targetType.Bits())
		if err != nil {
			return zero, err
		}
		return reflect.ValueOf(v).Convert(targetType), nil
	}

	return zero, fmt.Errorf("env: unsupported type %v", targetType)
}

// parseTime parses a time string.
func parseTime(raw string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("env: invalid time format %q", raw)
}


