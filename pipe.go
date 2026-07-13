package gonest

import "github.com/gonest-dev/gonest/internal/pipe"

// Pipe represents a param transform: a reflect-validated function that
// coerces a raw route param string into a typed value, attached to a
// specific route param via Route.Param (e.g. INSIGHT.md's
// `route.Param("user_id", ParseIntPipe)`). Unlike Middleware/Guard/
// Interceptor, Pipe.New(fn) defers fn until Route.Param declares it -- see
// internal/pipe.Pipe's doc comment for the full contract.
type Pipe = pipe.Pipe

// NewPipe creates a Pipe that defers fn until it is attached to a route via
// Route.Param, which declares it at that point (see internal/route.Route's
// Param doc comment for why: nothing in the DI bootstrap walks
// Route-registered Pipes, since Pipes aren't tracked by any Module -- Route
// itself is responsible for declaring any Pipe it's given). Like
// NewMiddleware/NewGuard/NewInterceptor/NewHttpException, New here is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed (root package is the only public door since
// Go blocks external import of internal/*, per AD-004 in STATE.md). See
// internal/pipe.New's doc comment for the full contract.
var NewPipe = pipe.New
