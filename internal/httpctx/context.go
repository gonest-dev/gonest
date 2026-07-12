// Package httpctx provides Context, the single per-request access point
// exposed to route Handlers. Context delegates to a minimal responder
// interface so the real Fiber-backed implementation can be wired in later
// (see design.md's "FiberApp" component) without changing Context's public
// API or these tests.
package httpctx

// responder is the minimal surface Context needs to serve its request/response
// methods. A fake implementation is used in tests; a Fiber-backed
// implementation is added in a later task.
type responder interface {
	JSON(v any) error
	SetStatus(code int)
	GetHeader(name string) string
	SetHeaderValue(name, value string)
	GetParam(name string) string
}

// Context encapsulates the HTTP request/response for a single route Handler.
type Context struct {
	res responder
}

// newContext builds a Context around the given responder.
func newContext(res responder) *Context {
	return &Context{res: res}
}

// Json writes value as the JSON response body.
func (ctx *Context) Json(value any) error {
	return ctx.res.JSON(value)
}

// Status sets the response status code and returns ctx for chaining.
func (ctx *Context) Status(code int) *Context {
	ctx.res.SetStatus(code)
	return ctx
}

// Header returns the value of the named request/response header.
func (ctx *Context) Header(name string) string {
	return ctx.res.GetHeader(name)
}

// SetHeader sets the named response header to value.
func (ctx *Context) SetHeader(name, value string) {
	ctx.res.SetHeaderValue(name, value)
}

// Param returns the raw string value of the named route param.
func (ctx *Context) Param(name string) string {
	return ctx.res.GetParam(name)
}
