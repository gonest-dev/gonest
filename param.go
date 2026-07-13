package gonest

import (
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
)

// MustParam converts the route param named name (from ctx.Param) into the
// requested type T, using a custom Pipe if the current route registered one
// for name (via Route.Param), or the default reflect+strconv coercion
// otherwise. Panics if name isn't declared on the current route's path, or
// if the value fails to convert. Go cannot re-export a generic function via
// var, so this is a real wrapper calling the internal one (same pattern as
// MustInject, see AD-004 in STATE.md).
func MustParam[T any](ctx *httpctx.Context, name string) T {
	return route.MustParam[T](ctx, name)
}
