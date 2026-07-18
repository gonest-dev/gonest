package gqltransport_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/gqltransport"
	"gonest.dev/gonest/internal/schema"
)

// fakeWSConn is a minimal gqltransport.WSConn for unit tests -- written
// captures every WriteMessage payload, readErr is a channel a test sends
// to (or closes) to make the next ReadMessage call return that error,
// simulating a client disconnect (WSHandler's own read loop is what
// detects it).
type fakeWSConn struct {
	param     string
	queries   map[string]string
	written   chan []byte
	readErr   chan error
	closed    chan struct{}
	writeFail bool
}

func newFakeWSConn(param string, queries map[string]string) *fakeWSConn {
	return &fakeWSConn{
		param:   param,
		queries: queries,
		written: make(chan []byte, 16),
		readErr: make(chan error, 1),
		closed:  make(chan struct{}, 1),
	}
}

func (c *fakeWSConn) Params(name string) string { return c.param }
func (c *fakeWSConn) Query(name string) string  { return c.queries[name] }

func (c *fakeWSConn) WriteMessage(messageType int, data []byte) error {
	if c.writeFail {
		return errors.New("write on closed connection")
	}
	c.written <- data
	return nil
}

func (c *fakeWSConn) ReadMessage() (int, []byte, error) {
	err := <-c.readErr
	return 0, nil, err
}

func (c *fakeWSConn) Close() error {
	select {
	case c.closed <- struct{}{}:
	default:
	}
	return nil
}

func TestWSHandler_UnknownSubscription_WritesErrorAndCloses(t *testing.T) {
	conn := newFakeWSConn("missing", nil)
	conn.readErr <- errors.New("unused") // let the (never-reached) read loop unblock if it somehow got there

	done := make(chan struct{})
	go func() {
		defer close(done)
		gqltransport.WSHandler(map[string]*gqlresolver.Subscription{})(conn)
	}()

	select {
	case msg := <-conn.written:
		var payload map[string]any
		if err := json.Unmarshal(msg, &payload); err != nil {
			t.Fatalf("failed to decode error message: %v", err)
		}
		if payload["message"] == nil {
			t.Fatalf("expected an error message, got %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the error message")
	}

	select {
	case <-conn.closed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Close() to be called")
	}

	<-done
}

func TestWSHandler_EmitsEventAsJSONMessage_AndDetectsDisconnectViaReadLoop(t *testing.T) {
	var gotDone <-chan struct{}
	res := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Subscription("onCreated", func(s *gqlresolver.Subscription) {
			s.Handler(func(ctx *gqlresolver.GraphqlContext, emit func(any)) {
				gotDone = ctx.Done()
				emit(map[string]any{"value": "hello"})
				<-ctx.Done()
			})
		})
	})
	res.Declare()
	subs := map[string]*gqlresolver.Subscription{"onCreated": res.OwnSubscriptions()[0]}

	conn := newFakeWSConn("onCreated", nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		gqltransport.WSHandler(subs)(conn)
	}()

	select {
	case msg := <-conn.written:
		var payload map[string]any
		if err := json.Unmarshal(msg, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		if payload["value"] != "hello" {
			t.Fatalf("payload = %+v, want value=hello", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the emitted event")
	}

	// Simulate the client disconnecting -- WSHandler's blocking
	// ReadMessage loop is what detects this, no heartbeat needed (unlike
	// SSE, WebSocket is bidirectional).
	conn.readErr <- errors.New("connection closed")

	select {
	case <-gotDone:
	case <-time.After(time.Second):
		t.Fatal("ctx.Done() did not close after simulated read error")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WSHandler did not return after disconnect")
	}
}

type wsFilterArgs struct {
	UserId int64 `json:"userId"`
}

func TestWSHandler_ArgsFromQueryString_ParsedIntoContext(t *testing.T) {
	argsZero := &wsFilterArgs{}
	argsTyp := reflect.TypeOf(*argsZero)
	t.Cleanup(func() { schema.Deregister(argsTyp) })
	argsSchema := schema.New(argsTyp, uintptr(unsafe.Pointer(argsZero)))
	argsSchema.Property(&argsZero.UserId).Integer().Required()

	var receivedUserID int64
	resolver := gqlresolver.New(func(r *gqlresolver.Resolver) {
		r.Subscription("onCreated", func(s *gqlresolver.Subscription) {
			s.Args(argsSchema)
			s.Handler(func(ctx *gqlresolver.GraphqlContext, emit func(any)) {
				var a wsFilterArgs
				if err := ctx.Args().ParseInto(&a, argsSchema); err != nil {
					panic(err)
				}
				receivedUserID = a.UserId
				emit(map[string]any{"ok": true})
				<-ctx.Done()
			})
		})
	})
	resolver.Declare()
	subs := map[string]*gqlresolver.Subscription{"onCreated": resolver.OwnSubscriptions()[0]}

	conn := newFakeWSConn("onCreated", map[string]string{"args": `{"userId": 42}`})

	done := make(chan struct{})
	go func() {
		defer close(done)
		gqltransport.WSHandler(subs)(conn)
	}()

	select {
	case <-conn.written:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for emit")
	}

	conn.readErr <- errors.New("connection closed")
	<-done

	if receivedUserID != 42 {
		t.Fatalf("receivedUserID = %d, want 42", receivedUserID)
	}
}
