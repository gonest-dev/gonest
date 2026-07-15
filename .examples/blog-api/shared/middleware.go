package shared

import (
	"fmt"
	"sync/atomic"

	"github.com/gonest-dev/gonest"
)

var requestCounter int64

// RequestIDMiddleware demonstrates a GLOBAL middleware (registered via
// Module.Use on the root AppModule, not per-controller) -- attaches a
// synthetic, monotonically increasing X-Request-Id response header before
// the rest of the chain (Guard/Interceptor/Handler) runs.
var RequestIDMiddleware = gonest.NewMiddleware(func(middleware *gonest.Middleware) {
	middleware.Handler(func(ctx *gonest.Context, next gonest.Next) {
		id := atomic.AddInt64(&requestCounter, 1)
		ctx.SetHeader("X-Request-Id", fmt.Sprintf("req-%d", id))
		next(ctx)
	})
})
