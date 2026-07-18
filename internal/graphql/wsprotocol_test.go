package graphql_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	gql "github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/graphql"
)

// closeCall records a single CloseWithCode invocation for assertions below.
type closeCall struct {
	code   int
	reason string
}

// protoFakeWSConn is a minimal graphql.WSConn for wsprotocol_test.go --
// same shape as ws_test.go's fakeWSConn, but CloseWithCode records the
// actual code/reason it was called with (ws_test.go's double just
// delegates to Close(), which loses that information -- this task's tests
// need to assert on the exact close code).
type protoFakeWSConn struct {
	written   chan []byte
	readErr   chan error
	readMsg   chan []byte
	closes    chan closeCall
	writeFail bool
}

func newProtoFakeWSConn() *protoFakeWSConn {
	return &protoFakeWSConn{
		written: make(chan []byte, 16),
		readErr: make(chan error, 4),
		readMsg: make(chan []byte, 16),
		closes:  make(chan closeCall, 4),
	}
}

func (c *protoFakeWSConn) Params(name string) string { return "" }
func (c *protoFakeWSConn) Query(name string) string  { return "" }

func (c *protoFakeWSConn) WriteMessage(messageType int, data []byte) error {
	if c.writeFail {
		return errors.New("write on closed connection")
	}
	c.written <- data
	return nil
}

// ReadMessage blocks until either a message is queued via readMsg, or an
// error is queued via readErr (simulating a dead connection) -- whichever
// arrives first.
func (c *protoFakeWSConn) ReadMessage() (int, []byte, error) {
	select {
	case data := <-c.readMsg:
		return 1, data, nil
	case err := <-c.readErr:
		return 0, nil, err
	}
}

func (c *protoFakeWSConn) Close() error { return nil }

func (c *protoFakeWSConn) CloseWithCode(code int, reason string) error {
	c.closes <- closeCall{code: code, reason: reason}
	// Unblock any pending ReadMessage the way a real closed socket would.
	select {
	case c.readErr <- errors.New("closed"):
	default:
	}
	return nil
}

func (c *protoFakeWSConn) sendJSON(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal test message: %v", err)
	}
	c.readMsg <- data
}

func TestWSProtocolHandler_NoConnectionInitWithinTimeout_Closes4408(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	select {
	case call := <-conn.closes:
		if call.code != 4408 {
			t.Fatalf("close code = %d, want 4408", call.code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for CloseWithCode(4408, ...)")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WSProtocolHandler did not return after timeout close")
	}
}

func TestWSProtocolHandler_ConnectionInit_RespondsAck(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	conn.sendJSON(t, map[string]string{"type": "connection_init"})

	select {
	case msg := <-conn.written:
		var payload map[string]any
		if err := json.Unmarshal(msg, &payload); err != nil {
			t.Fatalf("failed to decode ack message: %v", err)
		}
		if payload["type"] != "connection_ack" {
			t.Fatalf("payload = %+v, want type=connection_ack", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection_ack")
	}

	conn.readErr <- errors.New("connection closed")
	<-done
}

func TestWSProtocolHandler_DuplicateConnectionInit_Closes4429(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	conn.sendJSON(t, map[string]string{"type": "connection_init"})

	select {
	case <-conn.written:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection_ack")
	}

	conn.sendJSON(t, map[string]string{"type": "connection_init"})

	select {
	case call := <-conn.closes:
		if call.code != 4429 {
			t.Fatalf("close code = %d, want 4429", call.code)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for CloseWithCode(4429, ...)")
	}

	<-done
}

func TestWSProtocolHandler_UnknownMessageType_Closes4400(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	conn.sendJSON(t, map[string]string{"type": "not_a_real_type"})

	select {
	case call := <-conn.closes:
		if call.code != 4400 {
			t.Fatalf("close code = %d, want 4400", call.code)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for CloseWithCode(4400, ...)")
	}

	<-done
}

// newProtoTestSchema builds a minimal *gql.Schema exposing one Query field
// ("ping" -> "pong") and one Mutation field ("bump" -> "bumped") -- same
// pattern execute_test.go's newExecTestSchema already uses, just with a
// Mutation added so both dispatch paths are covered.
func newProtoTestSchema(t *testing.T) *gql.Schema {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
		r.Mutation("bump", func(m *graphql.Mutation) {
			m.Handler(func(ctx *graphql.GraphqlContext) any { return "bumped" })
		})
	})
	res.Declare()
	sch, err := graphql.Build(res.OwnQueries(), res.OwnMutations(), nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	return sch
}

// ackConn brings a protoFakeWSConn up through ConnectionInit ->
// ConnectionAck so subscribe tests can start from a ready state, discarding
// the ack message itself.
func ackConn(t *testing.T, conn *protoFakeWSConn) {
	t.Helper()
	conn.sendJSON(t, map[string]string{"type": "connection_init"})
	select {
	case <-conn.written:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection_ack")
	}
}

func TestWSProtocolHandler_SubscribeQuery_RespondsNextThenComplete(t *testing.T) {
	sch := newProtoTestSchema(t)
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(sch, nil)(conn)
	}()

	ackConn(t, conn)

	conn.sendJSON(t, map[string]any{
		"id":   "1",
		"type": "subscribe",
		"payload": map[string]any{
			"query": `{ ping }`,
		},
	})

	var next, complete map[string]any
	for i := 0; i < 2; i++ {
		select {
		case msg := <-conn.written:
			var payload map[string]any
			if err := json.Unmarshal(msg, &payload); err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}
			switch payload["type"] {
			case "next":
				next = payload
			case "complete":
				complete = payload
			default:
				t.Fatalf("unexpected message type %v", payload["type"])
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for next/complete")
		}
	}

	if next == nil {
		t.Fatal("did not receive a Next message")
	}
	if next["id"] != "1" {
		t.Fatalf("next[id] = %v, want %q", next["id"], "1")
	}
	payload, ok := next["payload"].(map[string]any)
	if !ok {
		t.Fatalf("next[payload] = %#v, want map", next["payload"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["ping"] != "pong" {
		t.Fatalf("next payload data = %#v, want {ping: pong}", payload["data"])
	}

	if complete == nil {
		t.Fatal("did not receive a Complete message")
	}
	if complete["id"] != "1" {
		t.Fatalf("complete[id] = %v, want %q", complete["id"], "1")
	}

	// No further message should follow a single-result operation's own
	// Complete (streaming would keep emitting Next; this must not).
	select {
	case msg := <-conn.written:
		t.Fatalf("unexpected extra message after Complete: %s", msg)
	case <-time.After(200 * time.Millisecond):
	}

	conn.readErr <- errors.New("connection closed")
	<-done
}

func TestWSProtocolHandler_SubscribeMutation_RespondsNextThenComplete(t *testing.T) {
	sch := newProtoTestSchema(t)
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(sch, nil)(conn)
	}()

	ackConn(t, conn)

	conn.sendJSON(t, map[string]any{
		"id":   "1",
		"type": "subscribe",
		"payload": map[string]any{
			"query": `mutation { bump }`,
		},
	})

	var next, complete map[string]any
	for i := 0; i < 2; i++ {
		select {
		case msg := <-conn.written:
			var payload map[string]any
			if err := json.Unmarshal(msg, &payload); err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}
			switch payload["type"] {
			case "next":
				next = payload
			case "complete":
				complete = payload
			default:
				t.Fatalf("unexpected message type %v", payload["type"])
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for next/complete")
		}
	}

	if next == nil {
		t.Fatal("did not receive a Next message")
	}
	payload, ok := next["payload"].(map[string]any)
	if !ok {
		t.Fatalf("next[payload] = %#v, want map", next["payload"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["bump"] != "bumped" {
		t.Fatalf("next payload data = %#v, want {bump: bumped}", payload["data"])
	}

	if complete == nil {
		t.Fatal("did not receive a Complete message")
	}

	conn.readErr <- errors.New("connection closed")
	<-done
}

func TestWSProtocolHandler_OperationBeforeAck_Closes4401(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	conn.sendJSON(t, map[string]any{
		"id":   "1",
		"type": "subscribe",
		"payload": map[string]any{
			"query": `{ ping }`,
		},
	})

	select {
	case call := <-conn.closes:
		if call.code != 4401 {
			t.Fatalf("close code = %d, want 4401", call.code)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for CloseWithCode(4401, ...)")
	}

	<-done
}

func TestWSProtocolHandler_Ping_RespondsPong(t *testing.T) {
	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, nil)(conn)
	}()

	conn.sendJSON(t, map[string]string{"type": "connection_init"})
	select {
	case <-conn.written:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection_ack")
	}

	conn.sendJSON(t, map[string]string{"type": "ping"})

	select {
	case msg := <-conn.written:
		var payload map[string]any
		if err := json.Unmarshal(msg, &payload); err != nil {
			t.Fatalf("failed to decode pong message: %v", err)
		}
		if payload["type"] != "pong" {
			t.Fatalf("payload = %+v, want type=pong", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pong")
	}

	conn.readErr <- errors.New("connection closed")
	<-done
}
