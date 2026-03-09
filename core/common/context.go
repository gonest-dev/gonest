// gonest/common/context.go
package common

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

// Context represents the request context with helper methods
type Context struct {
	Request    *http.Request
	Response   http.ResponseWriter
	params     map[string]string
	query      map[string]string
	metadata   map[string]any
	mu         sync.RWMutex
	statusCode int
	ctx        context.Context
}

// NewContext creates a new request context
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request:    r,
		Response:   w,
		params:     make(map[string]string),
		query:      make(map[string]string),
		metadata:   make(map[string]any),
		statusCode: http.StatusOK,
		ctx:        r.Context(),
	}
}

// Context returns the underlying context.Context
func (c *Context) Context() context.Context {
	return c.ctx
}

// SetContext sets the underlying context.Context
func (c *Context) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// Param returns a path parameter by name
func (c *Context) Param(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.params[name]
}

// SetParam sets a path parameter
func (c *Context) SetParam(name, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.params[name] = value
}

// Query returns a query parameter by name
func (c *Context) Query(name string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.URL.Query().Get(name)
}

// QueryDefault returns a query parameter with a default value
func (c *Context) QueryDefault(name, defaultValue string) string {
	value := c.Query(name)
	if value == "" {
		return defaultValue
	}
	return value
}

// Header returns a request header by name
func (c *Context) Header(name string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.Header.Get(name)
}

// SetHeader sets a response header
func (c *Context) SetHeader(name, value string) {
	if c.Response != nil {
		c.Response.Header().Set(name, value)
	}
}

// Set stores metadata in the context
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// Get retrieves metadata from the context
func (c *Context) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metadata[key]
}

// GetString retrieves a string metadata from the context
func (c *Context) GetString(key string) string {
	if value := c.Get(key); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt retrieves an int metadata from the context
func (c *Context) GetInt(key string) int {
	if value := c.Get(key); value != nil {
		if i, ok := value.(int); ok {
			return i
		}
	}
	return 0
}

// GetBool retrieves a bool metadata from the context
func (c *Context) GetBool(key string) bool {
	if value := c.Get(key); value != nil {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// Status sets the HTTP status code
func (c *Context) Status(code int) *Context {
	c.statusCode = code
	return c
}

// JSON sends a JSON response
func (c *Context) JSON(code int, obj any) error {
	c.statusCode = code
	c.SetHeader("Content-Type", "application/json")
	c.Response.WriteHeader(code)

	encoder := json.NewEncoder(c.Response)
	return encoder.Encode(obj)
}

// String sends a plain text response
func (c *Context) String(code int, format string, values ...any) error {
	c.statusCode = code
	c.SetHeader("Content-Type", "text/plain")
	c.Response.WriteHeader(code)

	if len(values) > 0 {
		_, err := c.Response.Write([]byte(format))
		return err
	}
	_, err := c.Response.Write([]byte(format))
	return err
}

// HTML sends an HTML response
func (c *Context) HTML(code int, html string) error {
	c.statusCode = code
	c.SetHeader("Content-Type", "text/html")
	c.Response.WriteHeader(code)

	_, err := c.Response.Write([]byte(html))
	return err
}

// Data sends raw bytes as response
func (c *Context) Data(code int, contentType string, data []byte) error {
	c.statusCode = code
	c.SetHeader("Content-Type", contentType)
	c.Response.WriteHeader(code)

	_, err := c.Response.Write(data)
	return err
}

// BindJSON binds the request body to a struct
func (c *Context) BindJSON(obj any) error {
	if c.Request == nil || c.Request.Body == nil {
		return json.Unmarshal([]byte("{}"), obj)
	}

	defer func() { _ = c.Request.Body.Close() }()
	decoder := json.NewDecoder(c.Request.Body)
	return decoder.Decode(obj)
}

// Body returns the raw request body
func (c *Context) Body() ([]byte, error) {
	if c.Request == nil || c.Request.Body == nil {
		return []byte{}, nil
	}

	defer func() { _ = c.Request.Body.Close() }()
	return io.ReadAll(c.Request.Body)
}

// Method returns the HTTP method
func (c *Context) Method() string {
	if c.Request == nil {
		return ""
	}
	return c.Request.Method
}

// Path returns the request path
func (c *Context) Path() string {
	if c.Request == nil {
		return ""
	}
	return c.Request.URL.Path
}

// StatusCode returns the response status code
func (c *Context) StatusCode() int {
	return c.statusCode
}

// Reset resets the context for reuse
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Request = r
	c.Response = w
	c.statusCode = http.StatusOK
	c.ctx = r.Context()

	c.mu.Lock()
	// Clear maps but keep the underlying storage
	for k := range c.params {
		delete(c.params, k)
	}
	for k := range c.metadata {
		delete(c.metadata, k)
	}
	c.mu.Unlock()
}


