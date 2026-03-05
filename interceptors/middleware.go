package interceptors

import (
	"time"

	"github.com/leandroluk/gonest/core"
)

// UseInterceptors creates a middleware that applies interceptors
func UseInterceptors(interceptors ...Interceptor) core.MiddlewareFunc {
	return func(next core.HandlerFunc) core.HandlerFunc {
		return func(ctx *core.Context) error {
			// Create execution context
			execCtx := &ExecutionContext{
				Context:   ctx,
				Handler:   next,
				Metadata:  make(map[string]any),
				StartTime: time.Now(),
			}

			// Build chain of interceptors
			handler := next

			// Apply interceptors in reverse order (like middleware)
			for i := len(interceptors) - 1; i >= 0; i-- {
				interceptor := interceptors[i]
				currentHandler := handler

				handler = func(ctx *core.Context) error {
					return interceptor.Intercept(execCtx, func() error {
						return currentHandler(ctx)
					})
				}
			}

			// Execute the chain
			return handler(ctx)
		}
	}
}

// ApplyInterceptors is a helper to apply interceptors to a handler
func ApplyInterceptors(handler core.HandlerFunc, interceptors ...Interceptor) core.HandlerFunc {
	middleware := UseInterceptors(interceptors...)
	return middleware(handler)
}
