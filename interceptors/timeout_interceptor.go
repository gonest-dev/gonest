package interceptors

import (
	"context"
	"fmt"
	"time"
)

// TimeoutInterceptor adds timeout to requests
type TimeoutInterceptor struct {
	timeout time.Duration
}

// NewTimeoutInterceptor creates a new timeout interceptor
func NewTimeoutInterceptor(timeout time.Duration) *TimeoutInterceptor {
	return &TimeoutInterceptor{
		timeout: timeout,
	}
}

// Intercept adds timeout to the request
func (i *TimeoutInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), i.timeout)
	defer cancel()

	// Channel to receive result
	done := make(chan error, 1)

	// Execute handler in goroutine
	go func() {
		done <- next()
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("request timeout after %v", i.timeout)
	}
}
