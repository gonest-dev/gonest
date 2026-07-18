package graphql_test

import (
	"bufio"
	"io"
	"testing"
	"time"

	"gonest.dev/gonest/internal/execution"
)

// fakeSSEResponder is a minimal execution.Responder whose WriteStream runs
// fn synchronously against a bufio.Writer wrapping an io.Pipe -- the pipe's
// read side is what a test consumes as "the client", and closing it
// (fakeSSEResponder.disconnect) simulates a client going away: any further
// write into the pipe's write side returns io.ErrClosedPipe, exactly the
// write-failure the SSE handlers (ssedistinct.go, ssesingle.go) use to
// detect disconnect.
//
// Shared across this package's *_test.go files (ssedistinct_test.go,
// ssesingle_test.go, cross_transport_test.go) -- extracted here
// (graphql-realtime-protocols feature, Milestone 18, T16) when the old
// sse.go/sse_test.go (Milestone 17's ad-hoc SSE transport, since replaced
// by the real graphql-sse-protocol handlers) were deleted, since those
// files already depended on this fixture and must not break.
type fakeSSEResponder struct {
	param    string
	queries  map[string]string
	headers  map[string]string
	body     []byte
	pr       *io.PipeReader
	pw       *io.PipeWriter
	status   int
	jsonBody any
}

func newFakeSSEResponder(param string, queries map[string]string) *fakeSSEResponder {
	pr, pw := io.Pipe()
	return &fakeSSEResponder{param: param, queries: queries, pr: pr, pw: pw}
}

func (f *fakeSSEResponder) JSON(v any) error {
	f.jsonBody = v
	return nil
}

func (f *fakeSSEResponder) SetStatus(code int) { f.status = code }
func (f *fakeSSEResponder) GetStatus() int {
	if f.status == 0 {
		return 200
	}
	return f.status
}
func (f *fakeSSEResponder) GetMethod() string { return "GET" }
func (f *fakeSSEResponder) GetPath() string   { return "" }
func (f *fakeSSEResponder) GetHeader(name string) string {
	if f.headers == nil {
		return ""
	}
	return f.headers[name]
}
func (f *fakeSSEResponder) SetHeaderValue(name, value string)     {}
func (f *fakeSSEResponder) GetParam(name string) string           { return f.param }
func (f *fakeSSEResponder) RawBody() []byte                       { return f.body }
func (f *fakeSSEResponder) Queries() map[string]string            { return f.queries }
func (f *fakeSSEResponder) HTML(s string) error                   { return nil }
func (f *fakeSSEResponder) SendString(s string) error             { return nil }
func (f *fakeSSEResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }

func (f *fakeSSEResponder) WriteStream(fn func(w *bufio.Writer)) {
	go func() {
		w := bufio.NewWriter(f.pw)
		fn(w)
		f.pw.Close()
	}()
}

// disconnect simulates the client going away -- any subsequent write into
// f.pw returns io.ErrClosedPipe.
func (f *fakeSSEResponder) disconnect() {
	f.pr.Close()
}

func (f *fakeSSEResponder) IsUpgradeRequest() bool                                              { return false }
func (f *fakeSSEResponder) Upgrade(handler func(conn execution.WSConn), subprotocols ...string) {}

func readLine(t *testing.T, r *bufio.Reader, timeout time.Duration) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- result{line, err}
	}()
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("ReadString error: %v", res.err)
		}
		return res.line
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a line")
		return ""
	}
}
