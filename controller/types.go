package controller

import (
	"github.com/gonest-dev/gonest/core"
)

// HTTPMethod represents HTTP methods
type HTTPMethod string

const (
	HTTPMethodGET     HTTPMethod = "GET"
	HTTPMethodPOST    HTTPMethod = "POST"
	HTTPMethodPUT     HTTPMethod = "PUT"
	HTTPMethodPATCH   HTTPMethod = "PATCH"
	HTTPMethodDELETE  HTTPMethod = "DELETE"
	HTTPMethodOPTIONS HTTPMethod = "OPTIONS"
	HTTPMethodHEAD    HTTPMethod = "HEAD"
)

// Controller interface that all controllers must implement
type Controller interface {
	// GetRoutes returns all routes defined in this controller
	GetRoutes() []*Route
}

// HandlerFunc is the function signature for route handlers
type HandlerFunc func(*core.Context) error

// Route represents a single route definition
type Route struct {
	Method      HTTPMethod
	Path        string
	Handler     HandlerFunc
	Params      []*ParamConfig
	Middlewares []core.MiddlewareFunc
	// Metadata for validation, guards, etc
	Metadata map[string]any
}

// ParamType represents the type of parameter
type ParamType string

const (
	ParamTypeBODY    ParamType = "body"
	ParamTypeQUERY   ParamType = "query"
	ParamTypePARAM   ParamType = "param"
	ParamTypeHEADER  ParamType = "header"
	ParamTypeREQ     ParamType = "req"
	ParamTypeRES     ParamType = "res"
	ParamTypeSESSION ParamType = "session"
	ParamTypeUSER    ParamType = "user"
)

// ParamConfig represents parameter configuration
type ParamConfig struct {
	Type      ParamType
	Name      string
	Required  bool
	Transform func(any) (any, error)
	Validate  func(any) error
}

// ControllerOptions represents controller configuration
type ControllerOptions struct {
	Prefix      string
	Middlewares []core.MiddlewareFunc
	// Metadata for guards, interceptors, etc
	Metadata map[string]any
}
