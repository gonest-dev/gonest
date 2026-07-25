package execution

// HttpContext bundles the read side (Request) and write side (Reply) of one
// HTTP request/response cycle behind a single value -- the one parameter
// every Handler/Guard/Middleware/Interceptor/Filter.Catch receives. Exactly
// 2 methods, Request()/Response() -- every other read/write operation is
// reached through one of those two, never promoted directly onto
// HttpContext itself (deliberate: one way to reach any given piece of
// data, see this feature's design.md Tech Decisions). Replaces the
// separate (req, res) 2-parameter shape request-response-split (AD-030)
// introduced; unlike that split, still keeps Request/Reply as real,
// separate, independently testable types underneath -- HttpContext is a
// thin wrapper, not a re-merge of their fields.
type HttpContext struct {
	req *Request
	res *Reply
}

// NewHttpContext wraps req/res (already built via New(Responder)) into a
// single HttpContext. Exported so internal/app (Stage 2.5 dispatch wiring)
// can construct one per request without a same-package helper.
func NewHttpContext(req *Request, res *Reply) *HttpContext {
	return &HttpContext{req: req, res: res}
}

// Request returns this context's read side.
func (c *HttpContext) Request() *Request {
	return c.req
}

// Response returns this context's write side (type *Reply -- named
// Response to read naturally at the call site, e.g.
// c.Response().Status(200).Json(...), matching Route.Response's own
// method-name-vs-type-name precedent: the METHOD describes the role
// ("give me the response half"), the TYPE (Reply) is disambiguated from
// the unrelated OpenAPI documentation builder of the same conceptual
// area, route.Response).
func (c *HttpContext) Response() *Reply {
	return c.res
}
