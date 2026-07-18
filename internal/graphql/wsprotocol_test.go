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

// newProtoSubTestSchema builds a subs map holding one Subscription
// ("count") whose Handler is supplied by the caller, matching how
// registerGraphql wires a real Subscription's HandlerFunc into
// WSProtocolHandler's own subs parameter.
func newProtoSubTestSchema(t *testing.T, handler func(ctx *graphql.GraphqlContext, emit func(any))) map[string]*graphql.Subscription {
	t.Helper()
	return newProtoMultiSubTestSchema(t, map[string]func(ctx *graphql.GraphqlContext, emit func(any)){
		"count": handler,
	})
}

// newProtoMultiSubTestSchema is newProtoSubTestSchema generalised to
// register one Subscription per (fieldName -> handler) entry -- used by
// tests that need two independently-identifiable streams running at once
// (e.g. to prove Completing one id's stream never touches another id's),
// since driving that from a single shared field/handler would leave which
// goroutine ran for which Subscribe id up to the Go scheduler, not the
// test.
func newProtoMultiSubTestSchema(t *testing.T, handlers map[string]func(ctx *graphql.GraphqlContext, emit func(any))) map[string]*graphql.Subscription {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		for name, handler := range handlers {
			name, handler := name, handler
			r.Subscription(name, func(s *graphql.Subscription) {
				s.Handler(handler)
			})
		}
	})
	res.Declare()
	subs := make(map[string]*graphql.Subscription)
	for _, s := range res.OwnSubscriptions() {
		subs[s.Name()] = s
	}
	return subs
}

func TestWSProtocolHandler_SubscribeSubscription_EmitsNextPerEmittedValue(t *testing.T) {
	started := make(chan struct{})
	subs := newProtoSubTestSchema(t, func(ctx *graphql.GraphqlContext, emit func(any)) {
		close(started)
		emit(1)
		emit(2)
		<-ctx.Done()
	})

	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, subs)(conn)
	}()

	ackConn(t, conn)

	conn.sendJSON(t, map[string]any{
		"id":   "1",
		"type": "subscribe",
		"payload": map[string]any{
			"query": `subscription { count }`,
		},
	})

	<-started

	for _, want := range []float64{1, 2} {
		select {
		case msg := <-conn.written:
			var payload map[string]any
			if err := json.Unmarshal(msg, &payload); err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}
			if payload["type"] != "next" {
				t.Fatalf("payload[type] = %v, want next", payload["type"])
			}
			if payload["id"] != "1" {
				t.Fatalf("payload[id] = %v, want %q", payload["id"], "1")
			}
			data, ok := payload["payload"].(map[string]any)["data"].(map[string]any)
			if !ok || data["count"] != want {
				t.Fatalf("next payload data = %#v, want {count: %v}", payload["payload"], want)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for next")
		}
	}

	// No Complete should follow -- the Subscription handler is still
	// blocked on ctx.Done(), i.e. streaming, unlike T6's single-result path.
	select {
	case msg := <-conn.written:
		t.Fatalf("unexpected extra message while still streaming: %s", msg)
	case <-time.After(200 * time.Millisecond):
	}

	conn.readErr <- errors.New("connection closed")
	<-done
}

func TestWSProtocolHandler_CompleteOneId_DoesNotAffectOtherActiveId(t *testing.T) {
	doneCh1 := make(chan struct{})
	doneCh2 := make(chan struct{})

	// Two distinct Subscription fields (rather than reusing one field for
	// both ids) so which stream is which is fixed by the query text itself,
	// not by goroutine scheduling order.
	subs := newProtoMultiSubTestSchema(t, map[string]func(ctx *graphql.GraphqlContext, emit func(any)){
		"countA": func(ctx *graphql.GraphqlContext, emit func(any)) {
			<-ctx.Done()
			close(doneCh1)
		},
		"countB": func(ctx *graphql.GraphqlContext, emit func(any)) {
			<-ctx.Done()
			close(doneCh2)
		},
	})

	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, subs)(conn)
	}()

	ackConn(t, conn)

	conn.sendJSON(t, map[string]any{
		"id":      "1",
		"type":    "subscribe",
		"payload": map[string]any{"query": `subscription { countA }`},
	})
	conn.sendJSON(t, map[string]any{
		"id":      "2",
		"type":    "subscribe",
		"payload": map[string]any{"query": `subscription { countB }`},
	})

	// Wait until both goroutines are registered by giving the handler loop
	// a moment to process both Subscribe messages before sending Complete.
	time.Sleep(100 * time.Millisecond)

	conn.sendJSON(t, map[string]any{"id": "1", "type": "complete"})

	select {
	case <-doneCh1:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for id 1's stream to stop after its own Complete")
	}

	select {
	case <-doneCh2:
		t.Fatal("id 2's stream stopped, but only id 1 was completed")
	case <-time.After(200 * time.Millisecond):
	}

	conn.readErr <- errors.New("connection closed")

	select {
	case <-doneCh2:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for id 2's stream to stop after connection drop")
	}

	<-done
}

func TestWSProtocolHandler_ConnectionDrops_StopsAllActiveSubscriptions(t *testing.T) {
	stopped1 := make(chan struct{})
	stopped2 := make(chan struct{})
	subs := newProtoMultiSubTestSchema(t, map[string]func(ctx *graphql.GraphqlContext, emit func(any)){
		"countA": func(ctx *graphql.GraphqlContext, emit func(any)) {
			<-ctx.Done()
			close(stopped1)
		},
		"countB": func(ctx *graphql.GraphqlContext, emit func(any)) {
			<-ctx.Done()
			close(stopped2)
		},
	})

	conn := newProtoFakeWSConn()

	done := make(chan struct{})
	go func() {
		defer close(done)
		graphql.WSProtocolHandler(nil, subs)(conn)
	}()

	ackConn(t, conn)

	conn.sendJSON(t, map[string]any{
		"id":      "1",
		"type":    "subscribe",
		"payload": map[string]any{"query": `subscription { countA }`},
	})
	conn.sendJSON(t, map[string]any{
		"id":      "2",
		"type":    "subscribe",
		"payload": map[string]any{"query": `subscription { countB }`},
	})

	time.Sleep(100 * time.Millisecond)

	conn.readErr <- errors.New("connection closed")

	select {
	case <-stopped1:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for id 1's stream to stop after connection drop")
	}
	select {
	case <-stopped2:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for id 2's stream to stop after connection drop")
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
