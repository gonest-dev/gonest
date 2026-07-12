// Package route defines HTTP route declaration types for gonest controllers.
package route

// HttpMethod identifies the HTTP verb a route responds to.
type HttpMethod int

const (
	// HttpGet corresponds to the HTTP GET method.
	HttpGet HttpMethod = iota
	// HttpPost corresponds to the HTTP POST method.
	HttpPost
	// HttpPut corresponds to the HTTP PUT method.
	HttpPut
	// HttpDelete corresponds to the HTTP DELETE method.
	HttpDelete
	// HttpQuery corresponds to the HTTP QUERY method, used for
	// list/query endpoints (see INSIGHT.md).
	HttpQuery
)

// String implements fmt.Stringer for debug-friendly output.
func (m HttpMethod) String() string {
	switch m {
	case HttpGet:
		return "GET"
	case HttpPost:
		return "POST"
	case HttpPut:
		return "PUT"
	case HttpDelete:
		return "DELETE"
	case HttpQuery:
		return "QUERY"
	default:
		return "Unknown"
	}
}
