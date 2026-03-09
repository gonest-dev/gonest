// gonest/interceptors/types.go
package interceptors

import (
	"time"

	"github.com/gonest-dev/gonest/core/common"
)

// ExecutionContext provides context for interceptor execution
type ExecutionContext struct {
	Context   *common.Context
	Handler   common.HandlerFunc
	Metadata  map[string]any
	StartTime time.Time
}

// Interceptor interface that all interceptors must implement
type Interceptor interface {
	// Intercept handles the request
	Intercept(ctx *ExecutionContext, next func() error) error
}

// InterceptorFunc is a function type that implements Interceptor
type InterceptorFunc func(*ExecutionContext, func() error) error

// Intercept implements Interceptor interface for InterceptorFunc
func (f InterceptorFunc) Intercept(ctx *ExecutionContext, next func() error) error {
	return f(ctx, next)
}

// CallHandler wraps the handler execution
type CallHandler struct {
	handler common.HandlerFunc
	context *common.Context
}

// Handle executes the handler
func (ch *CallHandler) Handle() error {
	return ch.handler(ch.context)
}

// NewCallHandler creates a new call handler
func NewCallHandler(handler common.HandlerFunc, ctx *common.Context) *CallHandler {
	return &CallHandler{
		handler: handler,
		context: ctx,
	}
}


