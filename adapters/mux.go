package adapters

import (
	"encoding/json"
	"net/http"

	"github.com/gonest-dev/gonest/core"
)

// MuxAdapter adapts GoNest to standard net/http
type MuxAdapter struct {
	config *AdapterConfig
}

// NewMuxAdapter creates a standard net/http adapter
func NewMuxAdapter(config ...*AdapterConfig) *MuxAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &MuxAdapter{
		config: cfg,
	}
}

// Name returns adapter name
func (a *MuxAdapter) Name() string {
	return "standard"
}

// WrapHandler wraps GoNest handler for net/http
func (a *MuxAdapter) WrapHandler(handler core.HandlerFunc) any {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := core.NewContext(w, r)
		ctx.Set("adapter", "standard")

		if err := handler(ctx); err != nil {
			a.handleError(w, err)
		}
	}
}

// WrapMiddleware wraps GoNest middleware for net/http
func (a *MuxAdapter) WrapMiddleware(middleware core.MiddlewareFunc) any {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := core.NewContext(w, r)
			ctx.Set("adapter", "standard")

			wrapped := middleware(func(c *core.Context) error {
				next.ServeHTTP(w, r)
				return nil
			})

			if err := wrapped(ctx); err != nil {
				a.handleError(w, err)
			}
		})
	}
}

// ExtractContext extracts context from http.Request
func (a *MuxAdapter) ExtractContext(platformCtx any) *core.Context {
	r, ok := platformCtx.(*http.Request)
	if !ok {
		// Return empty context if cast fails
		return &core.Context{}
	}

	// Create with nil ResponseWriter - will be set later
	return core.NewContext(nil, r)
}

// CreateContext creates GoNest context from http.Request
func (a *MuxAdapter) CreateContext(r *http.Request) *core.Context {
	// Create context with nil ResponseWriter - will be set in WrapHandler
	ctx := core.NewContext(nil, r)

	// Additional metadata
	ctx.Set("adapter", "standard")
	ctx.Set("remote_addr", r.RemoteAddr)

	return ctx
}

// handleError handles errors
func (a *MuxAdapter) handleError(w http.ResponseWriter, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, w)
		return
	}

	// Default error handling
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]any{
		"error": err.Error(),
	})
}

// ToMuxHandler converts GoNest handler to http.Handler
func ToMuxHandler(handler core.HandlerFunc) http.Handler {
	adapter := NewMuxAdapter()
	wrappedHandler := adapter.WrapHandler(handler)
	return http.HandlerFunc(wrappedHandler.(func(http.ResponseWriter, *http.Request)))
}

// ToMuxHandlerFunc converts GoNest handler to http.HandlerFunc
func ToMuxHandlerFunc(handler core.HandlerFunc) http.HandlerFunc {
	adapter := NewMuxAdapter()
	wrappedHandler := adapter.WrapHandler(handler)
	return wrappedHandler.(func(http.ResponseWriter, *http.Request))
}
