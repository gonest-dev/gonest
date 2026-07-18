package graphql_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gonest.dev/gonest/internal/emitter"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
)

type orderCreatedEvent struct {
	OrderId int64 `json:"orderId"`
}

// TestWSProtocolAndSSEDistinct_SameSubscription_BothReceiveSameEmittedEvent
// is spec.md's own Independent Test for P2, updated for the REAL transports
// (graphql-realtime-protocols feature, Milestone 18, T16): an event
// published via Emitter.Emit reaches a client connected via the real
// graphql-transport-ws protocol (WSProtocolHandler) AND another connected
// via the real graphql-sse Distinct connections mode (SSEDistinctHandler),
// both subscribing to the SAME Subscription; disconnecting one does not
// affect the other.
//
// Replaces the pre-T15 version of this test, which drove the old ad-hoc
// sse.go/ws.go transports (Milestone 17) -- those had no handshake/protocol
// framing of their own; deleted alongside them in this same task since
// registerGraphql no longer wires them into any route.
func TestWSProtocolAndSSEDistinct_SameSubscription_BothReceiveSameEmittedEvent(t *testing.T) {
	em := emitter.New()

	res := graphql.New(func(r *graphql.Resolver) {
		r.Subscription("onOrderCreated", func(s *graphql.Subscription) {
			s.Handler(func(ctx *graphql.GraphqlContext, emit func(any)) {
				ch := emitter.Subscribe[orderCreatedEvent](em, ctx.Done())
				for ev := range ch {
					emit(map[string]any{"orderId": ev.OrderId})
				}
			})
		})
	})
	res.Declare()
	subs := map[string]*graphql.Subscription{"onOrderCreated": res.OwnSubscriptions()[0]}

	// --- WebSocket client: real graphql-transport-ws handshake + subscribe. ---
	wsConn := newProtoFakeWSConn()
	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		graphql.WSProtocolHandler(nil, subs)(wsConn)
	}()
	ackConn(t, wsConn)
	wsConn.sendJSON(t, map[string]any{
		"id":   "1",
		"type": "subscribe",
		"payload": map[string]any{
			"query": `subscription { onOrderCreated }`,
		},
	})

	// --- SSE Distinct client: real graphql-sse Distinct connections mode. ---
	sseResponder := newFakeSSEResponder("", map[string]string{"query": `subscription { onOrderCreated }`})
	sseReq, sseRes := execution.New(sseResponder)
	graphql.SSEDistinctHandler(nil, subs)(sseReq, sseRes)

	// A single dedicated goroutine reads lines off the SSE pipe --
	// bufio.Reader is not safe for concurrent reads, so the polling loop
	// below must never spawn more than one reader itself.
	sseLines := make(chan string, 16)
	go func() {
		r := bufio.NewReader(sseResponder.pr)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			sseLines <- line
		}
	}()

	// wsNextOrderId decodes a graphql-transport-ws "next" message's own
	// {id, type, payload:{data:{onOrderCreated:{orderId}}}} shape, returning
	// (orderId, true) only for a well-formed Next carrying this field.
	wsNextOrderId := func(msg []byte) (int64, bool) {
		var payload map[string]any
		if json.Unmarshal(msg, &payload) != nil || payload["type"] != "next" {
			return 0, false
		}
		p, ok := payload["payload"].(map[string]any)
		if !ok {
			return 0, false
		}
		data, ok := p["data"].(map[string]any)
		if !ok {
			return 0, false
		}
		inner, ok := data["onOrderCreated"].(map[string]any)
		if !ok {
			return 0, false
		}
		v, ok := inner["orderId"].(float64)
		if !ok {
			return 0, false
		}
		return int64(v), true
	}

	// sseNextOrderId decodes a graphql-sse "data: " frame's own
	// {data:{onOrderCreated:{orderId}}} shape, mirroring wsNextOrderId
	// above for the SSE wire format.
	sseNextOrderId := func(line string) (int64, bool) {
		if !strings.HasPrefix(line, "data: ") {
			return 0, false
		}
		var payload struct {
			Data map[string]any `json:"data"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(line), "data: ")), &payload) != nil {
			return 0, false
		}
		inner, ok := payload.Data["onOrderCreated"].(map[string]any)
		if !ok {
			return 0, false
		}
		v, ok := inner["orderId"].(float64)
		if !ok {
			return 0, false
		}
		return int64(v), true
	}

	// Give both Subscription goroutines a moment to reach
	// emitter.Subscribe (best-effort; Emit itself is safe to call before a
	// subscriber registers, it would just miss the event -- the retry
	// below tolerates a slow subscriber without a fixed sleep).
	deadline := time.Now().Add(2 * time.Second)
	var sseGot, wsGot bool
	for time.Now().Before(deadline) && !(sseGot && wsGot) {
		em.Emit(orderCreatedEvent{OrderId: 7})

		if !wsGot {
			select {
			case msg := <-wsConn.written:
				if id, ok := wsNextOrderId(msg); ok && id == 7 {
					wsGot = true
				}
			case <-time.After(200 * time.Millisecond):
			}
		}
		if !sseGot {
			select {
			case line := <-sseLines:
				if id, ok := sseNextOrderId(line); ok && id == 7 {
					sseGot = true
				}
			case <-time.After(200 * time.Millisecond):
			}
		}
	}

	if !wsGot {
		t.Fatal("WebSocket client never received the emitted event")
	}
	if !sseGot {
		t.Fatal("SSE client never received the emitted event")
	}

	// Disconnect the WebSocket client only -- the SSE client must be
	// unaffected.
	wsConn.readErr <- errors.New("simulated disconnect")
	select {
	case <-wsDone:
	case <-time.After(time.Second):
		t.Fatal("WSProtocolHandler did not return after simulated disconnect")
	}

	em.Emit(orderCreatedEvent{OrderId: 8})
	deadline2 := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline2) {
		select {
		case line := <-sseLines:
			if id, ok := sseNextOrderId(line); ok && id == 8 {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("SSE client stopped receiving events after the WebSocket client disconnected")
		}
	}
	t.Fatal("SSE client stopped receiving events after the WebSocket client disconnected")
}
