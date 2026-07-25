package execution

import "bufio"

// Reply encapsulates the WRITE side of an HTTP request/response cycle for
// a single route Handler -- request-response-split feature (replaces the
// write-side half of the old single Context type). Reply holds a
// reference back to the Request that originated it (context.md's Decision:
// "não vejo problema response conhecer request internamente") -- useful for
// logging/decisions that need request data from a write-side callsite (e.g.
// gonest.NewLoggerMiddleware reading req.Method()/req.Path() after res
// already ran). Named Reply, not Response, to avoid colliding with
// route.Response (the OpenAPI per-status documentation builder) -- reached
// via HttpContext.Response() in every Handler/Guard/Middleware/Interceptor/
// Filter.Catch. Same (request, reply) naming Fastify uses for this exact
// role, a real precedent in the Node ecosystem this framework targets.
type Reply struct {
	req *Request
	res Responder
}

// New builds a Request and a Reply around the given Responder, the Reply
// holding a reference to the freshly built Request. Exported so other
// packages (namely internal/route, which needs to attach a *Route to a
// Request) can construct the pair in their own tests without a real
// Fiber-backed Responder existing yet.
func New(res Responder) (*Request, *Reply) {
	req := newRequest(res)
	return req, &Reply{req: req, res: res}
}

// Request returns the *Request that originated this Reply.
func (res *Reply) Request() *Request {
	return res.req
}

// Status sets the response status code and returns res for chaining.
func (res *Reply) Status(code int) *Reply {
	res.res.SetStatus(code)
	return res
}

// StatusCode returns the response status code currently set on this
// request -- the underlying HTTP framework's own default (200) if Status
// was never called. Used by a Logger middleware (gonest.NewLoggerMiddleware)
// to read back the status AFTER next(c) runs the rest of the chain, since
// Status/SetStatus are otherwise write-only.
func (res *Reply) StatusCode() int {
	return res.res.GetStatus()
}

// SetHeader sets the named response header to value.
func (res *Reply) SetHeader(name, value string) {
	res.res.SetHeaderValue(name, value)
}

// Json writes value as the JSON response body -- forces the underlying
// framework's default JSON Content-Type.
func (res *Reply) Json(value any) error {
	return res.res.JSON(value)
}

// Html writes s as a raw text/html response body, forcing
// Content-Type: text/html first -- request-response-split feature, renamed
// from HTML (all-caps) for consistency with Json/Text (see context.md's
// Decision: project already prefers "Json" over "JSON" elsewhere, e.g.
// BodyJsonSchema). Used by SetupSwagger (internal/openapi) to serve the
// Swagger UI's HTML page.
func (res *Reply) Html(s string) error {
	return res.res.HTML(s)
}

// Text writes s as a raw text/plain response body, forcing
// Content-Type: text/plain first -- request-response-split feature,
// replaces SendString. Unlike the old SendString (which wrote without
// setting any Content-Type), Text forces its content-type explicitly, same
// as Json/Html each force their own (context.md's Decision: "a ideia do
// Html/Text/Json é forçar os content types relacionados").
func (res *Reply) Text(s string) error {
	res.res.SetHeaderValue("Content-Type", "text/plain")
	return res.res.SendString(s)
}

// Stream registers fn as this response's own body-streaming writer
// (graphql-support feature, Milestone 17's SSE transport) -- fn runs on a
// live connection, writing chunks (and flushing) over time, instead of
// Json/Html/Text's "one full body, written once" contract. Used by
// internal/gqltransport's SSE handler to keep a GraphQL Subscription's
// connection open, writing one `data: ...\n\n` frame per emitted event.
func (res *Reply) Stream(fn func(w *bufio.Writer)) {
	res.res.WriteStream(fn)
}

// UpgradeWebSocket hands off the current connection to handler once the
// underlying HTTP engine completes the WebSocket handshake --
// graphql-realtime-protocols feature, Milestone 18's graphql-transport-ws/
// graphql-ws transport. subprotocols, when given, are offered to the
// underlying engine's handshake so it can negotiate and echo back a
// Sec-WebSocket-Protocol response header -- see Responder.Upgrade's own doc
// comment for why this matters. One-line delegation, same pattern as
// Stream/Json/Html/Text. Callers should check Request.IsWebSocketUpgrade()
// first.
func (res *Reply) UpgradeWebSocket(handler func(conn WSConn), subprotocols ...string) {
	res.res.Upgrade(handler, subprotocols...)
}
