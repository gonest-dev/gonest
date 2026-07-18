package graphql

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/validate"
)

// wsConnectionInitTimeout bounds how long WSProtocolHandler waits for the
// client's first ConnectionInit message before closing with 4408 --
// graphql-transport-ws protocol requirement. v1 does not expose this as
// config.
const wsConnectionInitTimeout = 3 * time.Second

// graphql-transport-ws message types (github.com/enisdenjo/graphql-ws's own
// PROTOCOL.md). connection_init/ping/pong were T5's handshake-only scope;
// subscribe/next/error/complete are added by T6 (Query/Mutation only);
// streaming Subscribe (multiple Next over time, and Complete cancelling one
// id) is T7's own scope.
const (
	wsMsgConnectionInit = "connection_init"
	wsMsgConnectionAck  = "connection_ack"
	wsMsgPing           = "ping"
	wsMsgPong           = "pong"
	wsMsgSubscribe      = "subscribe"
	wsMsgNext           = "next"
	wsMsgError          = "error"
	wsMsgComplete       = "complete"
)

// graphql-transport-ws well-known WebSocket close codes.
const (
	wsCloseInvalidMessage          = 4400
	wsCloseUnauthorized            = 4401
	wsCloseConnectionInitTimeout   = 4408
	wsCloseTooManyInitRequests     = 4429
	wsCloseSubscriberAlreadyExists = 4409
)

// wsProtocolMessage is the wire shape shared by every graphql-transport-ws
// message. `id` and `payload` are only meaningful for operation messages
// (Subscribe/Next/Error/Complete) -- connection_init/ping/pong ignore them.
type wsProtocolMessage struct {
	Id      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// wsSubscribePayload is the `payload` shape of a Subscribe message -- the
// exact same {query, variables, operationName} triple graphql-over-HTTP
// (and Execute) already accepts, just arriving over the WS frame instead
// of an HTTP body.
type wsSubscribePayload struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
}

// wsRootFieldName parses query and returns the name of its single root
// selection field (e.g. "ping" for `{ ping }` or `query { ping }`) --
// enough to look it up in subs and decide Query/Mutation (Execute) vs
// Subscription dispatch, without needing a second, hand-rolled parser:
// github.com/graphql-go/graphql/language/parser is the exact AST parser
// gql.Do (Execute) already uses internally to run the query, so this
// reuses the same parsing rules rather than guessing via string sniffing.
//
// When operationName is empty, the query's own single/first operation
// definition is used (matching GraphQL-over-HTTP's own rule: operationName
// is only required to disambiguate a document with multiple operations).
func wsRootFieldName(query, operationName string) (string, error) {
	doc, err := parser.Parse(parser.ParseParams{Source: query})
	if err != nil {
		return "", err
	}

	var op *ast.OperationDefinition
	for _, def := range doc.Definitions {
		od, ok := def.(*ast.OperationDefinition)
		if !ok {
			continue
		}
		if operationName == "" {
			op = od
			break
		}
		if od.Name != nil && od.Name.Value == operationName {
			op = od
			break
		}
	}

	if op == nil || op.SelectionSet == nil || len(op.SelectionSet.Selections) == 0 {
		return "", fmt.Errorf("gonest: no root selection field found in query")
	}

	field, ok := op.SelectionSet.Selections[0].(*ast.Field)
	if !ok || field.Name == nil {
		return "", fmt.Errorf("gonest: root selection is not a field")
	}

	return field.Name.Value, nil
}

// WSProtocolHandler builds the WebSocket connection handler implementing
// the graphql-transport-ws protocol's state machine (graphql-realtime-
// protocols feature, Milestone 18). T5 covered the handshake (ConnectionInit
// with timeout -> ConnectionAck, duplicate ConnectionInit -> 4429, unknown/
// invalid message -> 4400, Ping -> Pong); T6 added single-result Subscribe
// dispatch for Query/Mutation. T7 (this) adds streaming dispatch: a
// Subscribe whose root field matches a registered Subscription runs
// sub.HandlerFunc() in its own goroutine, emitting one Next per emit(value)
// call until either the client sends Complete for that id or the
// connection itself drops -- multiple such streams, and the handshake's
// own Ack/Pong replies, can all be writing to the same conn concurrently,
// so every write on conn goes through the single writeMu below.
func WSProtocolHandler(sch *gql.Schema, subs map[string]*Subscription) func(conn WSConn) {
	return func(conn WSConn) {
		defer func() { _ = recover() }()

		// writeMu serializes every write onto conn: the handshake replies
		// (Ack/Pong) below, the single-result Next/Complete pair T6 already
		// sends for Query/Mutation, and -- new in T7 -- however many
		// concurrent Subscription streams are emitting Next messages for
		// their own ids at the same time.
		var writeMu sync.Mutex
		writeJSON := func(v any) bool {
			data, err := json.Marshal(v)
			if err != nil {
				return true
			}
			writeMu.Lock()
			err = conn.WriteMessage(wsTextMessage, data)
			writeMu.Unlock()
			return err == nil
		}

		// activeOps tracks, per Subscribe id, the done channel of whichever
		// streaming Subscription is currently running for that id --
		// per-id (not a single connection-wide channel) so a Complete for
		// one id never disturbs another id's still-active stream. stopOp
		// removes and closes one id's done channel (idempotent: a second
		// call for an id already removed is a no-op); stopAllOps does the
		// same for every id still active, used when the connection itself
		// goes away.
		var opsMu sync.Mutex
		activeOps := make(map[string]chan struct{})

		registerOp := func(id string, done chan struct{}) {
			opsMu.Lock()
			activeOps[id] = done
			opsMu.Unlock()
		}
		// isOpActive reports whether id is currently registered in
		// activeOps -- T8's duplicate-id guard: handleSubscribe checks this
		// before dispatching a Subscribe so a second Subscribe for an id
		// whose earlier streaming operation hasn't completed yet (no
		// Complete sent, and the handler hasn't returned) is rejected with
		// 4409 rather than racing/overwriting the first one's registration.
		// Only Subscription (streaming) ids are ever actually present in
		// activeOps -- see handleSubscribe's own comment for why the
		// Query/Mutation single-result path deliberately does NOT register
		// here too.
		isOpActive := func(id string) bool {
			opsMu.Lock()
			_, ok := activeOps[id]
			opsMu.Unlock()
			return ok
		}
		stopOp := func(id string) {
			opsMu.Lock()
			done, ok := activeOps[id]
			if ok {
				delete(activeOps, id)
			}
			opsMu.Unlock()
			if ok {
				close(done)
			}
		}
		stopAllOps := func() {
			opsMu.Lock()
			toClose := make([]chan struct{}, 0, len(activeOps))
			for id, done := range activeOps {
				toClose = append(toClose, done)
				delete(activeOps, id)
			}
			opsMu.Unlock()
			for _, done := range toClose {
				close(done)
			}
		}
		// Every return path out of this handler -- handshake timeout,
		// 4400/4401/4429 closes, and the normal ReadMessage-error exit at
		// the bottom of the loop -- must stop every Subscription still
		// streaming on this connection; deferring it here covers all of
		// them in one place rather than duplicating the call at each exit.
		defer stopAllOps()

		type readResult struct {
			messageType int
			data        []byte
			err         error
		}

		firstRead := make(chan readResult, 1)
		go func() {
			messageType, data, err := conn.ReadMessage()
			firstRead <- readResult{messageType, data, err}
		}()

		var pending *readResult
		select {
		case res := <-firstRead:
			pending = &res
		case <-time.After(wsConnectionInitTimeout):
			_ = conn.CloseWithCode(wsCloseConnectionInitTimeout, "Connection initialisation timeout")
			return
		}

		acked := false

		// handleMessage decodes and dispatches a single raw frame. Returns
		// false when the connection should be closed (already closed by
		// this function) and the loop must stop.
		handleMessage := func(data []byte) bool {
			var msg wsProtocolMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = conn.CloseWithCode(wsCloseInvalidMessage, "Invalid message")
				return false
			}

			switch msg.Type {
			case wsMsgConnectionInit:
				if acked {
					_ = conn.CloseWithCode(wsCloseTooManyInitRequests, "Too many initialisation requests")
					return false
				}
				acked = true
				if !writeJSON(map[string]string{"type": wsMsgConnectionAck}) {
					return false
				}
				return true

			case wsMsgPing:
				if !writeJSON(map[string]string{"type": wsMsgPong}) {
					return false
				}
				return true

			case wsMsgSubscribe:
				// graphql-transport-ws: any operation (Subscribe included)
				// received before ConnectionAck was sent is Unauthorized --
				// distinct from an unrecognised message TYPE (default case
				// below, still 4400 regardless of ack state).
				if !acked {
					_ = conn.CloseWithCode(wsCloseUnauthorized, "Unauthorized")
					return false
				}
				return handleSubscribe(conn, sch, subs, msg, writeJSON, registerOp, stopOp, isOpActive)

			case wsMsgComplete:
				// Cancels only this id's own active stream (if any) --
				// stopOp is a no-op for an id that isn't (or is no longer)
				// streaming, e.g. Complete for a Query/Mutation's id that
				// already finished on its own in T6's single-result path.
				stopOp(msg.Id)
				return true

			default:
				_ = conn.CloseWithCode(wsCloseInvalidMessage, "Invalid message")
				return false
			}
		}

		if pending.err != nil {
			return
		}
		if !handleMessage(pending.data) {
			return
		}

		for {
			messageType, data, err := conn.ReadMessage()
			_ = messageType
			if err != nil {
				return
			}
			if !handleMessage(data) {
				return
			}
		}
	}
}

// handleSubscribe dispatches one Subscribe message. T6 covers the Query/
// Mutation path: the root field's name is resolved from the payload's
// query (via wsRootFieldName) and looked up in subs; when it is NOT a
// registered Subscription, the operation is request-response -- dispatched
// through graphql.Execute exactly like GraphQL-over-HTTP, then answered
// with exactly one Next followed by Complete (no further Next ever follows
// for this id, unlike a real streaming Subscription).
//
// T7 adds the other branch: when the root field IS a registered
// Subscription, sub.HandlerFunc() runs in its own goroutine (own done
// channel, registered under msg.Id via registerOp so a later Complete for
// this specific id -- or the connection dropping -- can cancel it without
// touching any other id's own stream), emitting one Next per emit(value)
// call for as long as the handler keeps running. Unlike the Query/Mutation
// branch, this never sends an immediate Complete -- Complete only follows
// a client-initiated cancel (wsMsgComplete's own handleMessage case) or the
// handler itself returning (self-cleanup below).
//
// writeJSON and registerOp/stopOp/isOpActive are threaded in from
// WSProtocolHandler so every write on conn and every id->done
// registration/lookup goes through that single connection-wide
// writeMu/opsMu -- this function holds no lock of its own.
//
// T8 adds the duplicate-id guard up front: isOpActive(msg.Id) is checked
// before either dispatch branch runs, and a hit closes the whole connection
// with 4409 (graphql-transport-ws's "Subscriber already exists"). Only the
// streaming Subscription branch below ever calls registerOp -- the Query/
// Mutation single-result branch deliberately does NOT also register its own
// id in activeOps, even briefly. That is a correctness-neutral choice, not
// an oversight: every Subscribe is decoded and dispatched from inside
// handleMessage, which itself is only ever invoked synchronously, one
// message at a time, from WSProtocolHandler's single read loop (the loop
// never calls conn.ReadMessage() again until the current handleMessage call
// returns). The Query/Mutation branch runs graphql.Execute and writes its
// Next+Complete synchronously, in-line, before returning -- so by
// construction no second Subscribe for the same id can ever reach this
// function while an earlier single-result operation for that id is still
// "in flight": there is no such window on this connection. Registering it
// in activeOps would add a lock/unlock pair (and an immediate stopOp right
// after) that could never actually catch a real duplicate, only add cost.
// The streaming branch is different because it returns immediately after
// registerOp, while sub.HandlerFunc() keeps running in its own goroutine
// for an arbitrary, unbounded time afterwards -- that is the only case
// where a later Subscribe for the same id can legitimately race with a
// still-active earlier one, which is exactly what isOpActive here guards
// against.
//
// Returns false when the connection should be closed (the new duplicate-id
// 4409 close, or a write failure on the synchronous Query/Mutation/Error
// paths) -- same "false stops the loop" contract handleMessage's other
// cases use. The streaming branch never returns false: a write failure
// inside emit() only stops that one stream (via stopOp), it does not by
// itself close the connection -- the read loop's own ReadMessage error is
// what detects a dead connection.
func handleSubscribe(conn WSConn, sch *gql.Schema, subs map[string]*Subscription, msg wsProtocolMessage, writeJSON func(any) bool, registerOp func(string, chan struct{}), stopOp func(string), isOpActive func(string) bool) bool {
	if isOpActive(msg.Id) {
		_ = conn.CloseWithCode(wsCloseSubscriberAlreadyExists, "Subscriber already exists")
		return false
	}

	var payload wsSubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return writeJSON(map[string]any{
			"id":      msg.Id,
			"type":    wsMsgError,
			"payload": []map[string]any{{"message": "gonest: invalid Subscribe payload: " + err.Error()}},
		})
	}

	rootField, err := wsRootFieldName(payload.Query, payload.OperationName)
	if err != nil {
		return writeJSON(map[string]any{
			"id":      msg.Id,
			"type":    wsMsgError,
			"payload": []map[string]any{{"message": "gonest: " + err.Error()}},
		})
	}

	if sub, isSubscription := subs[rootField]; isSubscription {
		done := make(chan struct{})
		registerOp(msg.Id, done)

		var args execution.Parseable
		if sub.ArgsSchema() != nil {
			args = validate.NewGraphqlArgsSource(payload.Variables)
		}
		ctx := NewGraphqlContext(args, done)

		emit := func(v any) {
			if !writeJSON(map[string]any{
				"id":   msg.Id,
				"type": wsMsgNext,
				"payload": map[string]any{
					"data": map[string]any{rootField: v},
				},
			}) {
				// conn is gone -- stop this id's own stream; the read
				// loop's ReadMessage error will separately tear down any
				// other stream still active on this connection.
				stopOp(msg.Id)
			}
		}

		go func() {
			// Self-cleanup for a handler that returns on its own (stream
			// ended without an explicit client Complete or a connection
			// drop): stopOp is a no-op if this id was already removed by
			// either of those, so this never double-closes done.
			defer stopOp(msg.Id)
			sub.HandlerFunc()(ctx, emit)
		}()

		return true
	}

	data, errs := Execute(sch, payload.Query, payload.Variables, payload.OperationName)

	if !writeJSON(map[string]any{
		"id":   msg.Id,
		"type": wsMsgNext,
		"payload": map[string]any{
			"data":   data,
			"errors": errs,
		},
	}) {
		return false
	}

	return writeJSON(map[string]any{
		"id":   msg.Id,
		"type": wsMsgComplete,
	})
}
