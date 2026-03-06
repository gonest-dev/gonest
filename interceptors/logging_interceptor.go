// gonest/interceptors/logging_interceptor.go
package interceptors

import (
	"log"
	"time"
)

// LoggingInterceptor logs request/response information
type LoggingInterceptor struct {
	logger      *log.Logger
	logRequest  bool
	logResponse bool
	logDuration bool
}

// LoggingInterceptorOptions configures the logging interceptor
type LoggingInterceptorOptions struct {
	Logger      *log.Logger
	LogRequest  bool
	LogResponse bool
	LogDuration bool
}

// NewLoggingInterceptor creates a new logging interceptor
func NewLoggingInterceptor(opts ...*LoggingInterceptorOptions) *LoggingInterceptor {
	options := &LoggingInterceptorOptions{
		Logger:      log.Default(),
		LogRequest:  true,
		LogResponse: false,
		LogDuration: true,
	}

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &LoggingInterceptor{
		logger:      options.Logger,
		logRequest:  options.LogRequest,
		logResponse: options.LogResponse,
		logDuration: options.LogDuration,
	}
}

// Intercept logs the request
func (i *LoggingInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	start := time.Now()

	// Log request
	if i.logRequest {
		method := ctx.Context.Get("method")
		path := ctx.Context.Get("path")
		i.logger.Printf("[REQUEST] %s %s", method, path)
	}

	// Execute handler
	err := next()

	// Log response
	duration := time.Since(start)

	if i.logDuration {
		method := ctx.Context.Get("method")
		path := ctx.Context.Get("path")

		if err != nil {
			i.logger.Printf("[RESPONSE] %s %s - ERROR: %v [%v]", method, path, err, duration)
		} else {
			i.logger.Printf("[RESPONSE] %s %s - OK [%v]", method, path, duration)
		}
	}

	return err
}

// SimpleLoggingInterceptor creates a basic logging interceptor
func SimpleLoggingInterceptor() *LoggingInterceptor {
	return NewLoggingInterceptor()
}
