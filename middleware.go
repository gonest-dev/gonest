package gonest

import "github.com/gonest-dev/gonest/internal/middleware"

// Middleware represents a single reusable request-observation/mutation
// unit with a continuation-passing (ctx, next) shape, mirroring
// Express/Nest middleware. It is attached via Controller.Use/Module.Use
// (e.g. INSIGHT.md's `controller.Use(RequestIdMiddleware)`). See
// internal/middleware.Middleware's doc comment for the full contract.
type Middleware = middleware.Middleware

// Next represents the continuation of the middleware chain: calling it
// runs whatever comes after the current middleware (the next middleware,
// or eventually the route's own Handler). See internal/middleware.Next's
// doc comment for why its underlying type is identical in shape to a route
// Handler.
type Next = middleware.Next

// NewMiddleware creates a Middleware and runs fn on it immediately (unlike
// Provider/Module/Controller/Pipe, which defer fn until bootstrap). Like
// NewHttpException, New here is not generic, so Go allows aliasing the
// plain func directly via var -- no wrapper function is needed (root
// package is the only public door since Go blocks external import of
// internal/*, per AD-004 in STATE.md). See internal/middleware.New's doc
// comment for the full contract.
var NewMiddleware = middleware.New
