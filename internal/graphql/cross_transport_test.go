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

var errWsDisconnect = errors.New("simulated disconnect")

type orderCreatedEvent struct {
	OrderId int64 `json:"orderId"`
}

// TestSSEAndWebSocket_SameSubscription_BothReceiveSameEmittedEvent is
// spec.md's own Independent Test for P2: an event published via
// Emitter.Emit reaches a client connected via WebSocket AND another
// connected via SSE, both subscribing to the SAME Subscription;
// disconnecting one does not affect the other.
func TestSSEAndWebSocket_SameSubscription_BothReceiveSameEmittedEvent(t *testing.T) {
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

	// --- SSE client ---
	sseResponder := newFakeSSEResponder("onOrderCreated", nil)
	sseReq, sseRes := execution.New(sseResponder)
	graphql.SSEHandler(subs)(sseReq, sseRes)

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

	// --- WebSocket client ---
	wsConn := newFakeWSConn("onOrderCreated", nil)
	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		graphql.WSHandler(subs)(wsConn)
	}()

	// Give both Subscription goroutines a moment to reach
	// emitter.Subscribe (best-effort; Emit itself is safe to call before a
	// subscriber registers, it would just miss the event -- the retry
	// below tolerates a slow subscriber without a fixed sleep).
	deadline := time.Now().Add(2 * time.Second)
	var sseGot, wsGot bool
	for time.Now().Before(deadline) && !(sseGot && wsGot) {
		em.Emit(orderCreatedEvent{OrderId: 7})

		if !sseGot {
			select {
			case line := <-sseLines:
				if strings.HasPrefix(line, "data: ") {
					var payload map[string]any
					if json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(line), "data: ")), &payload) == nil {
						if v, _ := payload["orderId"].(float64); int64(v) == 7 {
							sseGot = true
						}
					}
				}
			case <-time.After(200 * time.Millisecond):
			}
		}
		if !wsGot {
			select {
			case msg := <-wsConn.written:
				var payload map[string]any
				if json.Unmarshal(msg, &payload) == nil {
					if v, _ := payload["orderId"].(float64); int64(v) == 7 {
						wsGot = true
					}
				}
			case <-time.After(200 * time.Millisecond):
			}
		}
	}

	if !sseGot {
		t.Fatal("SSE client never received the emitted event")
	}
	if !wsGot {
		t.Fatal("WebSocket client never received the emitted event")
	}

	// Disconnect the WebSocket client only -- the SSE client must be
	// unaffected.
	wsConn.readErr <- errWsDisconnect
	select {
	case <-wsDone:
	case <-time.After(time.Second):
		t.Fatal("WSHandler did not return after simulated disconnect")
	}

	em.Emit(orderCreatedEvent{OrderId: 8})
	deadline2 := time.Now().Add(time.Second)
	for time.Now().Before(deadline2) {
		select {
		case line := <-sseLines:
			if strings.HasPrefix(line, "data: ") {
				return
			}
			// blank line trailing the previous "data: ...\n\n" frame --
			// keep waiting for the actual next data line.
		case <-time.After(time.Second):
			t.Fatal("SSE client stopped receiving events after the WebSocket client disconnected")
		}
	}
	t.Fatal("SSE client stopped receiving events after the WebSocket client disconnected")
}
