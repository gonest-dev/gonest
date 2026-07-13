package route

import (
	"fmt"
	"reflect"
	"strconv"
)

// defaultCoerce converts raw (a route param's raw string value, e.g. from
// httpctx.Context.Param) into the requested type T via reflect+strconv.
//
// Supported T: string, int, int64, bool, float64. Any other T, or a raw
// value that fails to parse as T, returns a non-nil error describing the
// failure — this function never panics itself. It is deliberately isolated
// from *httpctx.Context/*Route (see T5): the future public MustParam[T]
// wrapper is expected to convert this error into a panic, after first
// checking for a custom Pipe and for param existence via ctx.Param.
//
// Note on absence: httpctx.Context.Param returns "" for a param name that
// doesn't exist on the current route (Fiber's c.Params semantics — no
// separate "not found" signal). defaultCoerce has no notion of "this
// route's params" at all; it only ever sees the raw string already handed
// to it. For T=string, an empty raw converts successfully to "" (empty
// string is a valid string). For every other supported T, an empty raw
// fails to parse and returns an error. MustParam[T] (T5) is expected to
// check for absence itself before calling defaultCoerce, since defaultCoerce
// alone cannot distinguish "absent" from "present but empty" when T=string.
func defaultCoerce[T any](raw string) (T, error) {
	var zero T

	switch any(zero).(type) {
	case string:
		return any(raw).(T), nil
	case int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return zero, fmt.Errorf("gonest: value %q could not be converted to int: %w", raw, err)
		}
		return any(n).(T), nil
	case int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return zero, fmt.Errorf("gonest: value %q could not be converted to int64: %w", raw, err)
		}
		return any(n).(T), nil
	case bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return zero, fmt.Errorf("gonest: value %q could not be converted to bool: %w", raw, err)
		}
		return any(b).(T), nil
	case float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return zero, fmt.Errorf("gonest: value %q could not be converted to float64: %w", raw, err)
		}
		return any(f).(T), nil
	default:
		return zero, fmt.Errorf("gonest: unsupported param type %s for value %q", reflect.TypeOf(zero), raw)
	}
}
