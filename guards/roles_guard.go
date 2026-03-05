package guards

import (
	"fmt"
)

// RolesGuard validates user roles
type RolesGuard struct {
	requiredRoles []string
	userExtractor func(*ExecutionContext) ([]string, error)
}

// RolesGuardOptions configures the roles guard
type RolesGuardOptions struct {
	RequiredRoles []string
	UserExtractor func(*ExecutionContext) ([]string, error)
}

// NewRolesGuard creates a new roles guard
func NewRolesGuard(opts *RolesGuardOptions) *RolesGuard {
	if opts.UserExtractor == nil {
		// Default: extract roles from context
		opts.UserExtractor = func(ctx *ExecutionContext) ([]string, error) {
			roles, exists := ctx.Context.Get("user:roles").([]string)
			if !exists {
				return nil, fmt.Errorf("user roles not found in context")
			}
			return roles, nil
		}
	}

	return &RolesGuard{
		requiredRoles: opts.RequiredRoles,
		userExtractor: opts.UserExtractor,
	}
}

// CanActivate checks if user has required roles
func (g *RolesGuard) CanActivate(ctx *ExecutionContext) (bool, error) {
	// Extract user roles
	userRoles, err := g.userExtractor(ctx)
	if err != nil {
		return false, NewGuardError("Could not extract user roles", 403).
			WithDetail("error", err.Error())
	}

	// Check if user has at least one required role
	if !g.hasRequiredRole(userRoles) {
		return false, NewGuardError("Insufficient permissions", 403).
			WithDetail("required", g.requiredRoles).
			WithDetail("user", userRoles)
	}

	return true, nil
}

// hasRequiredRole checks if user has any of the required roles
func (g *RolesGuard) hasRequiredRole(userRoles []string) bool {
	for _, required := range g.requiredRoles {
		for _, userRole := range userRoles {
			if required == userRole {
				return true
			}
		}
	}
	return false
}

// RequireRoles creates a roles guard with simple role checking
func RequireRoles(roles ...string) *RolesGuard {
	return NewRolesGuard(&RolesGuardOptions{
		RequiredRoles: roles,
	})
}

// RequireAllRoles creates a guard that requires ALL specified roles
func RequireAllRoles(roles ...string) *RolesGuard {
	return &RolesGuard{
		requiredRoles: roles,
		userExtractor: func(ctx *ExecutionContext) ([]string, error) {
			userRoles, exists := ctx.Context.Get("user:roles").([]string)
			if !exists {
				return nil, fmt.Errorf("user roles not found in context")
			}
			return userRoles, nil
		},
	}
}

// CanActivate for RequireAllRoles checks if user has ALL required roles
func (g *RolesGuard) CanActivateAll(ctx *ExecutionContext) (bool, error) {
	userRoles, err := g.userExtractor(ctx)
	if err != nil {
		return false, NewGuardError("Could not extract user roles", 403).
			WithDetail("error", err.Error())
	}

	// Check if user has ALL required roles
	for _, required := range g.requiredRoles {
		found := false
		for _, userRole := range userRoles {
			if required == userRole {
				found = true
				break
			}
		}
		if !found {
			return false, NewGuardError("Missing required role", 403).
				WithDetail("required", required)
		}
	}

	return true, nil
}
