// gonest/guards/middleware.go
package guards

import (
	"github.com/gonest-dev/gonest/core/common"
)

// UseGuards creates a middleware that applies guards
func UseGuards(guards ...Guard) common.MiddlewareFunc {
	return func(next common.HandlerFunc) common.HandlerFunc {
		return func(ctx *common.Context) error {
			// Create execution context
			execCtx := &ExecutionContext{
				Context:  ctx,
				Handler:  next,
				Metadata: make(map[string]any),
			}

			// Execute all guards
			for _, guard := range guards {
				canActivate, err := guard.CanActivate(execCtx)

				if err != nil {
					// Check if it's a GuardError
					if guardErr, ok := err.(*GuardError); ok {
						return ctx.JSON(guardErr.StatusCode, guardErr.ToJSON())
					}
					// Generic error
					return ctx.JSON(500, map[string]any{
						"statusCode": 500,
						"message":    "Internal server error",
						"error":      err.Error(),
					})
				}

				if !canActivate {
					// Guard rejected the request
					return ctx.JSON(403, map[string]any{
						"statusCode": 403,
						"message":    "Forbidden",
					})
				}
			}

			// All guards passed, proceed to handler
			return next(ctx)
		}
	}
}

// SetMetadata sets metadata in the execution context
func SetMetadata(key string, value any) func(*ExecutionContext) {
	return func(ctx *ExecutionContext) {
		ctx.Metadata[key] = value
	}
}

// GetMetadata retrieves metadata from execution context
func GetMetadata(ctx *ExecutionContext, key string) (any, bool) {
	value, exists := ctx.Metadata[key]
	return value, exists
}

// ApplyGuards is a helper to apply guards to a handler
func ApplyGuards(handler common.HandlerFunc, guards ...Guard) common.HandlerFunc {
	middleware := UseGuards(guards...)
	return middleware(handler)
}


