package shared

import (
	"fmt"
	"time"

	"gonest.dev/gonest"
)

// TimingInterceptor logs how long each request took, wrapping the route
// Handler's own execution -- dogfoods the (ctx, next) continuation shape,
// attached per-controller (Interceptor has no Module-level global
// registration, unlike Middleware/Filter).
var TimingInterceptor = gonest.NewInterceptor(func(interceptor *gonest.Interceptor) {
	interceptor.Handler(func(req *gonest.Request, res *gonest.Response, next gonest.InterceptorNext) {
		start := time.Now()
		next(req, res)
		fmt.Printf("[timing] request took %s\n", time.Since(start))
	})
})
