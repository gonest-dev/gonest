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
	return NewParamsSource(ctx).Parse[T](s)
}

func parseParams[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	return NewParamsSource(ctx).SafeParse[T](s)
}

func mustParseQuery[T any](ctx *execution.Request, s *schema.Schema) T {
	return NewQuerySource(ctx).Parse[T](s)
}

func parseQuery[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	return NewQuerySource(ctx).SafeParse[T](s)
}

func mustParseHeaders[T any](ctx *execution.Request, s *schema.Schema) T {
	return NewHeadersSource(ctx).Parse[T](s)
}

func parseHeaders[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	return NewHeadersSource(ctx).SafeParse[T](s)
}

func mustParseJSON[T any](ctx *execution.Request, s *schema.Schema) T {
	return NewJSONBodySource(ctx).Parse[T](s)
}

func parseJSON[T any](ctx *execution.Request, s *schema.Schema) (T, error) {
	return NewJSONBodySource(ctx).SafeParse[T](s)
}

func mustParseForm[T any](ctx *execution.Request, s *schema.Schema, onFile func(*execution.FormFile) error) T {
	return NewFormBodySource(ctx, onFile).Parse[T](s)
}

func parseForm[T any](ctx *execution.Request, s *schema.Schema, onFile func(*execution.FormFile) error) (T, error) {
	return NewFormBodySource(ctx, onFile).SafeParse[T](s)
}
