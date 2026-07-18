package gqltransport_test

import (
	"bufio"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/gqltransport"
	"gonest.dev/gonest/internal/schema"
)

// fakeSSEResponder is a minimal execution.Responder whose WriteStream runs
// fn synchronously against a bufio.Writer wrapping an io.Pipe -- the pipe's
// read side is what the test consumes as "the client", and closing it
// (fakeSSEResponder.disconnect) simulates a client going away: any further
// write into the pipe's write side returns io.ErrClosedPipe, exactly the
// write-failure SSEHandler's own emit/heartbeat use to detect disconnect.
type fakeSSEResponder struct {
	param   string
	queries map[string]string
	pr      *io.PipeReader
	pw      *io.PipeWriter
	status  int
}

func newFakeSSEResponder(param string, queries map[string]string) *fakeSSEResponder {
	pr, pw := io.Pipe()
	return &fakeSSEResponder{param: param, queries: queries, pr: pr, pw: pw}
}

func (f *fakeSSEResponder) JSON(v any) error { return nil }

func (f *fakeSSEResponder) SetStatus(code int) { f.status = code }
func (f *fakeSSEResponder) GetStatus() int {
	if f.status == 0 {
		return 200
	}
	return f.status
}
func (f *fakeSSEResponder) GetMethod() string                 { return "GET" }
func (f *fakeSSEResponder) GetPath() string                   { return "" }
func (f *fakeSSEResponder) GetHeader(name string) string      { return "" }
func (f *fakeSSEResponder) SetHeaderValue(name, value string) {}
func (f *fakeSSEResponder) GetParam(name string) string       { return f.param }
func (f *fakeSSEResponder) RawBody() []byte                   { return nil }
func (f *fakeSSEResponder) Queries() map[string]string        { return f.queries }
func (f *fakeSSEResponder) HTML(s string) error                { return nil }
func (f *fakeSSEResponder) SendString(s string) error          { return nil }
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

func TestSSEHandler_UnknownSubscription_Returns404(t *testing.T) {
	responder := newFakeSSEResponder("missing", nil)
	req, res := execution.New(responder)

	gqltransport.SSEHandler(map[string]*gqlresolver.Subscription{})(req, res)

	if responder.GetStatus() != 404 {
		t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
	}
}

func TestSSEHandler_EmitsEventAsSSEFrame(t *testing.T) {
	var gotDone <-chan struct{}
	sub := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Subscription("onCreated", func(s *gqlresolver.Subscription) {
			s.Handler(func(ctx *gqlresolver.GraphqlContext, emit func(any)) {
				gotDone = ctx.Done()
				// Emits repeatedly (not just once) so that AFTER the test
				// simulates a client disconnect, the NEXT emit's write
				// attempt is what actually detects the failure and closes
				// ctx.Done() -- a write failure can only ever be noticed
				// on an actual write, per SSEHandler's own doc comment on
				// why it also sends periodic heartbeats.
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						emit(map[string]any{"value": "hello"})
					}
				}
			})
		})
	})
	sub.Declare()
	subs := map[string]*gqlresolver.Subscription{"onCreated": sub.OwnSubscriptions()[0]}

	responder := newFakeSSEResponder("onCreated", nil)
	req, res := execution.New(responder)

	gqltransport.SSEHandler(subs)(req, res)

	r := bufio.NewReader(responder.pr)
	line := readLine(t, r, time.Second)
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("first line = %q, want a 'data: ' SSE frame", line)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(line), "data: ")), &payload); err != nil {
		t.Fatalf("failed to decode SSE frame payload: %v", err)
	}
	if payload["value"] != "hello" {
		t.Fatalf("payload = %+v, want value=hello", payload)
	}

	responder.disconnect()

	select {
	case <-gotDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ctx.Done() did not close after simulated client disconnect")
	}
}

type filterArgs struct {
	UserId int64 `json:"userId"`
}

func TestSSEHandler_ArgsFromQueryString_ParsedIntoContext(t *testing.T) {
	argsZero := &filterArgs{}
	argsTyp := reflect.TypeOf(*argsZero)
	t.Cleanup(func() { schema.Deregister(argsTyp) })
	argsSchema := schema.New(argsTyp, uintptr(unsafe.Pointer(argsZero)))
	argsSchema.Property(&argsZero.UserId).Integer().Required()

	var receivedUserID int64
	sub := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Subscription("onCreated", func(s *gqlresolver.Subscription) {
			s.Args(argsSchema)
			s.Handler(func(ctx *gqlresolver.GraphqlContext, emit func(any)) {
				var a filterArgs
				if err := ctx.Args().ParseInto(&a, argsSchema); err != nil {
					panic(err)
				}
				receivedUserID = a.UserId
				emit(map[string]any{"ok": true})
			})
		})
	})
	sub.Declare()
	subs := map[string]*gqlresolver.Subscription{"onCreated": sub.OwnSubscriptions()[0]}

	responder := newFakeSSEResponder("onCreated", map[string]string{"args": `{"userId": 42}`})
	req, res := execution.New(responder)

	gqltransport.SSEHandler(subs)(req, res)

	r := bufio.NewReader(responder.pr)
	readLine(t, r, time.Second)

	if receivedUserID != 42 {
		t.Fatalf("receivedUserID = %d, want 42", receivedUserID)
	}
}
