package shared

import (
	"fmt"
	"sync/atomic"

	"gonest.dev/gonest"
)

var requestCounter atomic.Int64

// RequestIDMiddleware demonstrates a GLOBAL middleware (registered via
// Module.Use on the root AppModule, not per-controller) -- attaches a
// synthetic, monotonically increasing X-Request-Id response header before
// the rest of the chain (Guard/Interceptor/Handler) runs.
var RequestIDMiddleware = func() *gonest.Middleware {
	return gonest.NewMiddleware(func(middleware *gonest.Middleware) {
		middleware.Handler(func(ctx *gonest.RestContext, next gonest.Next) {
			id := requestCounter.Add(1)
			ctx.SetHeader("X-Request-Id", fmt.Sprintf("req-%d", id))
			next(ctx)
		})
	})
}
