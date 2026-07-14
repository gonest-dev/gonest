package route

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/pipe"
)

// defaultCoerce converts raw (a route param's raw string value, e.g. from
// execution.Context.Param) into the requested type T via reflect+strconv.
//
// Supported T: string, int, int64, bool, float64. Any other T, or a raw
// value that fails to parse as T, returns a non-nil error describing the
// failure — this function never panics itself. It is deliberately isolated
// from *execution.Context/*Route (see T5): the future public MustParam[T]
// wrapper is expected to convert this error into a panic, after first
// checking for a custom Pipe and for param existence via ctx.Param.
//
// Note on absence: execution.Context.Param returns "" for a param name that
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

// MustParam is the real implementation behind the public root
// gonest.MustParam[T] wrapper (Go cannot re-export a generic function via
// var, see AD-004 -- same reasoning as inject.MustInject/gonest.MustInject).
//
// It resolves ctx's currently-attached *Route (via ctx.Route(), an `any`
// that MustParam type-asserts back to *Route -- see execution.Context's
// WithRoute/Route doc comment for why the link is untyped at the execution
// layer) and:
//
//  1. If a Route is attached and its declared path does NOT have a ":name"
//     segment for name (Route.HasParam), panics with the
//     "no param named" message -- this is the existence check that resolves
//     T4's documented ambiguity (ctx.Param returning "" cannot alone
//     distinguish "absent" from "present but empty" for T=string).
//  2. If a Route is attached and has a custom Pipe registered for name
//     (Route.PipeFor), calls that Pipe's Handler with (ctx, raw) and
//     returns its result. Any panic from the Pipe's Handler itself
//     propagates unchanged (expected pass-through, not caught here).
//  3. Otherwise (no Route attached, or no custom Pipe for name), falls back
//     to defaultCoerce[T]. A conversion failure panics with the
//     "could not be converted" message.
func MustParam[T any](ctx *execution.Context, name string) T {
	raw := ctx.Param(name)

	r, hasRoute := ctx.Route().(*Route)
	if hasRoute && !r.HasParam(name) {
		panic(fmt.Sprintf("gonest: no param named %q on this route", name))
	}

	if hasRoute {
		if p, ok := r.PipeFor(name); ok {
			return callPipeHandler[T](p, ctx, raw, name)
		}
	}

	v, err := defaultCoerce[T](raw)
	if err != nil {
		var zero T
		panic(fmt.Sprintf("gonest: param %q could not be converted to %s: %v", name, reflect.TypeOf(zero), err))
	}
	return v
}

// callPipeHandler invokes p's stored Handler (func(ctx *execution.Context, raw
// string) T) via reflect and returns its typed result. Panics from the
// Handler itself propagate unchanged -- reflect.Value.Call does not recover
// panics, so this is naturally pass-through.
func callPipeHandler[T any](p *pipe.Pipe, ctx *execution.Context, raw, name string) T {
	fn := p.HandlerFunc()
	out := fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(raw)})
	result, ok := out[0].Interface().(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf("gonest: param %q could not be converted to %s: custom Pipe returned %s", name, reflect.TypeOf(zero), out[0].Type()))
	}
	return result
}
