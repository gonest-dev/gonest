package graphql

import (
	"encoding/json"
	"fmt"
	"sync"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/validate"
)

// wsTextMessage is the WebSocket text-frame opcode (RFC 6455 -- 0x1),
// mirrored by every Go WebSocket library (gorilla/websocket,
// fasthttp/websocket) as a named constant with this same value; hardcoded
// here rather than importing fasthttp/websocket just for the constant,
// since WSConn (below) already keeps this package adapter-agnostic.
const wsTextMessage = 1

// WSConn is an alias of execution.WSConn -- moved there
// (graphql-realtime-protocols feature, Milestone 18, T2) so
// internal/execution can also depend on it (namely for the
// graphql-transport-ws protocol's own close-code handling) without an
// import cycle back into internal/graphql. Kept here under its original
// name so ws.go's WSHandler and ws_test.go's fakeWSConn need no other
// change.
type WSConn = execution.WSConn

// WSHandler builds the WebSocket connection handler for whichever
// Subscription subs[name] resolves to (graphql-support feature, Milestone
// 17, T10). Same Args format as SSEHandler (a single `?args=<JSON
// object>` query parameter) and the same dispatch shape (calls
// Subscription.HandlerFunc() directly, bypassing graphql-go's own
// execution engine) -- see sse.go's own doc comments for why both.
//
// Unlike SSE (send-only, needs a heartbeat to detect a dead connection),
// WebSocket is bidirectional -- the blocking ReadMessage loop below IS the
// disconnect detector: it returns an error the moment the connection
// closes from either side, no heartbeat needed.
func WSHandler(subs map[string]*Subscription) func(conn WSConn) {
	return func(conn WSConn) {
		defer func() { _ = recover() }()

		name := conn.Params("name")
		sub, ok := subs[name]
		if !ok {
			_ = conn.WriteMessage(wsTextMessage, []byte(fmt.Sprintf(`{"message":%q}`, fmt.Sprintf("gonest: no subscription named %q", name))))
			_ = conn.Close()
			return
		}

		var argsParseable execution.Parseable
		if sub.ArgsSchema() != nil {
			var parsed map[string]any
			if raw := conn.Query("args"); raw != "" {
				if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
					_ = conn.WriteMessage(wsTextMessage, []byte(fmt.Sprintf(`{"message":%q}`, "gonest: invalid ?args= JSON: "+err.Error())))
					_ = conn.Close()
					return
				}
			}
			argsParseable = validate.NewGraphqlArgsSource(parsed)
		}

		done := make(chan struct{})
		var closeOnce sync.Once
		closeDone := func() { closeOnce.Do(func() { close(done) }) }
		ctx := NewGraphqlContext(argsParseable, done)

		var writeMu sync.Mutex
		emit := func(v any) {
			payload, err := json.Marshal(v)
			if err != nil {
				return
			}
			writeMu.Lock()
			writeErr := conn.WriteMessage(wsTextMessage, payload)
			writeMu.Unlock()
			if writeErr != nil {
				closeDone()
			}
		}

		handlerDone := make(chan struct{})
		go func() {
			defer close(handlerDone)
			sub.HandlerFunc()(ctx, emit)
		}()

		// Blocking read loop -- its only purpose is disconnect detection;
		// gonest's Subscription protocol has no client-to-server message
		// of its own (Args are already fixed at connection time via the
		// query string), so any received message is simply discarded.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				closeDone()
				break
			}
		}

		<-handlerDone
		_ = conn.Close()
	}
}
