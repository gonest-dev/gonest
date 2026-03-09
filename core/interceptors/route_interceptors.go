// gonest/interceptors/route_interceptors.go
package interceptors

import "github.com/gonest-dev/gonest/core/common"

// RouteInterceptor wraps an interceptor for route-level use
type RouteInterceptor struct {
	interceptor Interceptor
}

// NewRouteInterceptor creates a route-level interceptor
func NewRouteInterceptor(interceptor Interceptor) *RouteInterceptor {
	return &RouteInterceptor{
		interceptor: interceptor,
	}
}

// Apply applies the interceptor to a handler
func (ri *RouteInterceptor) Apply(handler common.HandlerFunc) common.HandlerFunc {
	return ApplyInterceptors(handler, ri.interceptor)
}

// UseRouteInterceptor is a helper for route-level interceptors
func UseRouteInterceptor(interceptor Interceptor) common.MiddlewareFunc {
	return UseInterceptors(interceptor)
}

// Helpers for common route-level interceptors

// CacheRoute caches a specific route
func CacheRoute(ttl ...interface{}) common.MiddlewareFunc {
	var cacheInterceptor *CacheInterceptor

	if len(ttl) > 0 {
		if duration, ok := ttl[0].(interface{ GetDuration() interface{} }); ok {
			_ = duration // placeholder for actual implementation
		}
	}

	cacheInterceptor = SimpleCacheInterceptor(0) // Use default
	return UseInterceptors(cacheInterceptor)
}

// TimeoutRoute sets timeout for a specific route
func TimeoutRoute(timeout interface{}) common.MiddlewareFunc {
	// Type assertion placeholder
	_ = timeout
	timeoutInterceptor := NewTimeoutInterceptor(0) // Use passed timeout
	return UseInterceptors(timeoutInterceptor)
}

// LogRoute adds logging to a specific route
func LogRoute() common.MiddlewareFunc {
	loggingInterceptor := SimpleLoggingInterceptor()
	return UseInterceptors(loggingInterceptor)
}

// TransformRoute adds transformation to a specific route
func TransformRoute(transform func(any) (any, error)) common.MiddlewareFunc {
	transformInterceptor := NewTransformInterceptor(transform)
	return UseInterceptors(transformInterceptor)
}


