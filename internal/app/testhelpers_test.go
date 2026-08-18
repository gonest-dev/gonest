package app

import (
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// mustParse mirrors gonest.MustParse[T] (gonest.go, the root package --
// unimportable from here, since gonest imports internal/app and this would
// be a cycle) for this package's own tests, which dispatch real requests
// through registerRoutes (this package) and read ctx.Params()/etc. inside
// route Handlers exactly like a real caller would.
func mustParse[T any](src execution.Parseable, s *schema.Schema) T {
	var zero T
	if err := src.ParseInto(&zero, s); err != nil {
		panic(err)
	}
	return zero
}
