package gonest

import (
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolve"
)

// MustResolve declares a dependency on type T (which must be a pointer
// type, e.g. *Foo) from owner's builder fn -- used inside a Provider's or
// Controller's deferred builder fn. It allocates and returns a placeholder
// value; the real module-scoped search happens in a later bootstrap stage.
// It panics if T is not a pointer type. Go cannot re-export a generic
// function via var, so this is a real wrapper calling the internal one.
func MustResolve[T any](owner module.Owner) T {
	return resolve.MustResolve[T](owner)
}
