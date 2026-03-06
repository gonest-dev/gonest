// gonest/platform/chi.go
package platform

import (
	"encoding/json"
	"net/http"

	"github.com/gonest-dev/gonest/core"
)

// ChiAdapter adapts GoNest to Chi router
type ChiAdapter struct {
	config *AdapterConfig
}

// NewChiAdapter creates a Chi adapter
func NewChiAdapter(config ...*AdapterConfig) *ChiAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &ChiAdapter{
		config: cfg,
	}
}

// Name returns adapter name
func (a *ChiAdapter) Name() string {
	return "chi"
}

// WrapHandler wraps GoNest handler for Chi
func (a *ChiAdapter) WrapHandler(handler core.HandlerFunc) any {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := core.NewContext(w, r)
		ctx.Set("adapter", "chi")

		if err := handler(ctx); err != nil {
			a.handleError(w, err)
		}
	}
}

// WrapMiddleware wraps GoNest middleware for Chi
func (a *ChiAdapter) WrapMiddleware(middleware core.MiddlewareFunc) any {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := core.NewContext(w, r)
			ctx.Set("adapter", "chi")

			wrapped := middleware(func(_ *core.Context) error {
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
func (a *ChiAdapter) ExtractContext(platformCtx any) *core.Context {
	r, ok := platformCtx.(*http.Request)
	if !ok {
		return &core.Context{}
	}

	return core.NewContext(nil, r)
}

// CreateContext creates GoNest context from http.Request
func (a *ChiAdapter) CreateContext(r *http.Request) *core.Context {
	ctx := core.NewContext(nil, r)
	ctx.Set("adapter", "chi")
	return ctx
}

// handleError handles errors
func (a *ChiAdapter) handleError(w http.ResponseWriter, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, w)
		return
	}

	// Default error handling
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": err.Error(),
	})
}

// ToChiHandler converts GoNest handler to Chi handler
func ToChiHandler(handler core.HandlerFunc) http.HandlerFunc {
	adapter := NewChiAdapter()
	wrappedHandler := adapter.WrapHandler(handler)
	return wrappedHandler.(func(http.ResponseWriter, *http.Request))
}

// ToChiMiddleware converts GoNest middleware to Chi middleware
func ToChiMiddleware(middleware core.MiddlewareFunc) func(http.Handler) http.Handler {
	adapter := NewChiAdapter()
	wrappedMiddleware := adapter.WrapMiddleware(middleware)
	return wrappedMiddleware.(func(http.Handler) http.Handler)
}
