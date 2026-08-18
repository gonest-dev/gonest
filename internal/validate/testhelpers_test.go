package validate

import (
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// This package's own tests exercise paramsSource/querySource/jsonBodySource/
// formBodySource/headersSource directly (white-box), rather than through
// gonest.Parse[T]/MustParse[T] (gonest.go, the root package -- importing it
// from here would be a cycle: gonest.go imports internal/validate). These
// helpers are the exact same thin `var zero T; src.ParseInto(&zero, s);
// return zero` shape as gonest.Parse/MustParse, scoped to this package's own
// tests, so every existing test keeps calling a one-line generic helper
// instead of hand-rolling `var zero Foo; NewXSource(ctx).ParseInto(&zero, s)`
// at every call site.

func mustParseParams[T any](ctx *execution.Request, s *schema.Schema) T {
	v, err := parseParams[T](ctx, s)
	if err != nil {
		panic(err)
	}
	return v
}

func parseParams[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	var zero T
	err := NewParamsSource(ctx).ParseInto(&zero, s)
	return zero, err
}

func mustParseQuery[T any](ctx *execution.Request, s *schema.Schema) T {
	v, err := parseQuery[T](ctx, s)
	if err != nil {
		panic(err)
	}
	return v
}

func parseQuery[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	var zero T
	err := NewQuerySource(ctx).ParseInto(&zero, s)
	return zero, err
}

func mustParseHeaders[T any](ctx *execution.Request, s *schema.Schema) T {
	v, err := parseHeaders[T](ctx, s)
	if err != nil {
		panic(err)
	}
	return v
}

func parseHeaders[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	var zero T
	err := NewHeadersSource(ctx).ParseInto(&zero, s)
	return zero, err
}

func mustParseJSON[T any](ctx *execution.Request, s *schema.Schema) T {
	v, err := parseJSON[T](ctx, s)
	if err != nil {
		panic(err)
	}
	return v
}

func parseJSON[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	var zero T
	err := NewJSONBodySource(ctx).ParseInto(&zero, s)
	return zero, err
}

func mustParseForm[T any](ctx *execution.Request, s *schema.Schema, onFile func(*execution.FormFile) error) T {
	v, err := parseForm[T](ctx, s, onFile)
	if err != nil {
		panic(err)
	}
	return v
}

func parseForm[T any](ctx *execution.Request, s *schema.Schema, onFile func(*execution.FormFile) error) (T, error) {
	var zero T
	err := NewFormBodySource(ctx, onFile).ParseInto(&zero, s)
	return zero, err
}
