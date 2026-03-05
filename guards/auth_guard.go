package guards

import (
	"strings"
)

// AuthGuard validates authentication tokens
type AuthGuard struct {
	tokenValidator func(token string) (bool, error)
	headerName     string
	prefix         string
}

// AuthGuardOptions configures the auth guard
type AuthGuardOptions struct {
	TokenValidator func(token string) (bool, error)
	HeaderName     string
	Prefix         string
}

// NewAuthGuard creates a new authentication guard
func NewAuthGuard(opts *AuthGuardOptions) *AuthGuard {
	if opts.HeaderName == "" {
		opts.HeaderName = "Authorization"
	}

	if opts.Prefix == "" {
		opts.Prefix = "Bearer"
	}

	return &AuthGuard{
		tokenValidator: opts.TokenValidator,
		headerName:     opts.HeaderName,
		prefix:         opts.Prefix,
	}
}

// CanActivate checks if request has valid authentication
func (g *AuthGuard) CanActivate(ctx *ExecutionContext) (bool, error) {
	// Get authorization header
	authHeader := ctx.Context.Get(g.headerName)

	if authHeader == "" {
		return false, NewGuardError("Missing authentication token", 401)
	}

	// Extract token
	token := g.extractToken(authHeader.(string))
	if token == "" {
		return false, NewGuardError("Invalid token format", 401)
	}

	// Validate token
	if g.tokenValidator != nil {
		valid, err := g.tokenValidator(token)
		if err != nil {
			return false, NewGuardError("Token validation failed", 401).
				WithDetail("error", err.Error())
		}

		if !valid {
			return false, NewGuardError("Invalid token", 401)
		}
	}

	// Store token in context for later use
	ctx.Context.Set("auth:token", token)

	return true, nil
}

// extractToken extracts token from authorization header
func (g *AuthGuard) extractToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)

	if len(parts) != 2 {
		return ""
	}

	if parts[0] != g.prefix {
		return ""
	}

	return parts[1]
}

// SimpleAuthGuard creates a basic auth guard with token validation
func SimpleAuthGuard(validTokens ...string) *AuthGuard {
	tokenMap := make(map[string]bool)
	for _, token := range validTokens {
		tokenMap[token] = true
	}

	return NewAuthGuard(&AuthGuardOptions{
		TokenValidator: func(token string) (bool, error) {
			return tokenMap[token], nil
		},
	})
}
