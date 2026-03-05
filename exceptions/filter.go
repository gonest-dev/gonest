package exceptions

import (
	"log"

	"github.com/leandroluk/gonest/core"
)

// GlobalExceptionFilter handles all unhandled exceptions
type GlobalExceptionFilter struct {
	logger       *log.Logger
	includeStack bool
	showDetails  bool
}

// GlobalExceptionFilterOptions configures the global exception filter
type GlobalExceptionFilterOptions struct {
	Logger       *log.Logger
	IncludeStack bool
	ShowDetails  bool
}

// NewGlobalExceptionFilter creates a new global exception filter
func NewGlobalExceptionFilter(opts ...*GlobalExceptionFilterOptions) *GlobalExceptionFilter {
	options := &GlobalExceptionFilterOptions{
		Logger:       log.Default(),
		IncludeStack: false,
		ShowDetails:  true,
	}

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &GlobalExceptionFilter{
		logger:       options.Logger,
		includeStack: options.IncludeStack,
		showDetails:  options.ShowDetails,
	}
}

// Catch handles the exception
func (f *GlobalExceptionFilter) Catch(err error, ctx *core.Context) error {
	// Check if it's an HttpException
	if httpErr, ok := err.(*HttpException); ok {
		// Log the error
		f.logger.Printf("[ERROR] %s %s: %s",
			ctx.Get("method"), ctx.Get("path"), httpErr.Message)

		// Return JSON response
		response := httpErr.ToJSON()
		if !f.showDetails {
			delete(response, "details")
			delete(response, "cause")
		}

		return ctx.JSON(httpErr.StatusCode, response)
	}

	// Unknown error - return 500
	f.logger.Printf("[ERROR] %s %s: %v",
		ctx.Get("method"), ctx.Get("path"), err)

	return ctx.JSON(500, map[string]any{
		"statusCode": 500,
		"message":    "Internal Server Error",
		"error":      err.Error(),
	})
}

// UseExceptionFilter creates a middleware that applies exception filter
func UseExceptionFilter(filter ExceptionFilter) core.MiddlewareFunc {
	return func(next core.HandlerFunc) core.HandlerFunc {
		return func(ctx *core.Context) error {
			// Execute handler
			err := next(ctx)

			// If error, apply filter
			if err != nil {
				return filter.Catch(err, ctx)
			}

			return nil
		}
	}
}
