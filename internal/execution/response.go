package execution

// Response encapsulates the WRITE side of an HTTP request/response cycle for
// a single route Handler -- request-response-split feature (replaces the
// write-side half of the old single Context type). Response holds a
// reference back to the Request that originated it (context.md's Decision:
// "não vejo problema response conhecer request internamente") -- useful for
// logging/decisions that need request data from a write-side callsite (e.g.
// gonest.NewLoggerMiddleware reading req.Method()/req.Path() after res
// already ran).
type Response struct {
	req *Request
	res Responder
}

// New builds a Request and a Response around the given Responder, the
// Response holding a reference to the freshly built Request. Exported so
// other packages (namely internal/route, which needs to attach a *Route to
// a Request) can construct the pair in their own tests without a real
// Fiber-backed Responder existing yet.
func New(res Responder) (*Request, *Response) {
	req := newRequest(res)
	return req, &Response{req: req, res: res}
}

// Request returns the *Request that originated this Response.
func (res *Response) Request() *Request {
	return res.req
}

// Status sets the response status code and returns res for chaining.
func (res *Response) Status(code int) *Response {
	res.res.SetStatus(code)
	return res
}

// StatusCode returns the response status code currently set on this
// request -- the underlying HTTP framework's own default (200) if Status
// was never called. Used by a Logger middleware (gonest.NewLoggerMiddleware)
// to read back the status AFTER next(req, res) runs the rest of the chain,
// since Status/SetStatus are otherwise write-only.
func (res *Response) StatusCode() int {
	return res.res.GetStatus()
}

// SetHeader sets the named response header to value.
func (res *Response) SetHeader(name, value string) {
	res.res.SetHeaderValue(name, value)
}

// Json writes value as the JSON response body -- forces the underlying
// framework's default JSON Content-Type.
func (res *Response) Json(value any) error {
	return res.res.JSON(value)
}

// Html writes s as a raw text/html response body, forcing
// Content-Type: text/html first -- request-response-split feature, renamed
// from HTML (all-caps) for consistency with Json/Text (see context.md's
// Decision: project already prefers "Json" over "JSON" elsewhere, e.g.
// BodyJsonSchema). Used by SetupSwagger (internal/openapi) to serve the
// Swagger UI's HTML page.
func (res *Response) Html(s string) error {
	return res.res.HTML(s)
}

// Text writes s as a raw text/plain response body, forcing
// Content-Type: text/plain first -- request-response-split feature,
// replaces SendString. Unlike the old SendString (which wrote without
// setting any Content-Type), Text forces its content-type explicitly, same
// as Json/Html each force their own (context.md's Decision: "a ideia do
// Html/Text/Json é forçar os content types relacionados").
func (res *Response) Text(s string) error {
	res.res.SetHeaderValue("Content-Type", "text/plain")
	return res.res.SendString(s)
}
