package graphql

import (
	"encoding/json"
	"fmt"
	"time"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
)

// wsConnectionInitTimeout bounds how long WSProtocolHandler waits for the
// client's first ConnectionInit message before closing with 4408 --
// graphql-transport-ws protocol requirement. v1 does not expose this as
// config.
const wsConnectionInitTimeout = 3 * time.Second

// graphql-transport-ws message types (github.com/enisdenjo/graphql-ws's own
// PROTOCOL.md). connection_init/ping/pong were T5's handshake-only scope;
// subscribe/next/error/complete are added by T6 -- streaming Subscribe
// (multiple Next over time) is still out of scope, left to T7.
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
	wsCloseInvalidMessage        = 4400
	wsCloseUnauthorized          = 4401
	wsCloseConnectionInitTimeout = 4408
	wsCloseTooManyInitRequests   = 4429
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
// protocols feature, Milestone 18). This task (T5) covers only the
// handshake: ConnectionInit (with timeout) -> ConnectionAck, duplicate
// ConnectionInit -> 4429, unknown/invalid message -> 4400, and Ping ->
// Pong. Subscribe/Complete handling is added by future tasks that extend
// this same read loop -- not a rewrite.
func WSProtocolHandler(sch *gql.Schema, subs map[string]*Subscription) func(conn WSConn) {
	return func(conn WSConn) {
		defer func() { _ = recover() }()

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
				ack, _ := json.Marshal(map[string]string{"type": wsMsgConnectionAck})
				if err := conn.WriteMessage(wsTextMessage, ack); err != nil {
					return false
				}
				return true

			case wsMsgPing:
				pong, _ := json.Marshal(map[string]string{"type": wsMsgPong})
				if err := conn.WriteMessage(wsTextMessage, pong); err != nil {
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
				return handleSubscribe(conn, sch, subs, msg)

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

// handleSubscribe dispatches one Subscribe message. This task (T6) only
// covers the Query/Mutation path: the root field's name is resolved from
// the payload's query (via wsRootFieldName) and looked up in subs; when it
// is NOT a registered Subscription, the operation is request-response --
// dispatched through graphql.Execute exactly like GraphQL-over-HTTP, then
// answered with exactly one Next followed by Complete (no further Next
// ever follows for this id, unlike a real streaming Subscription).
//
// Returns false when the connection should be closed (write failure) --
// same "false stops the loop" contract handleMessage's other cases use.
//
// SPEC_DEVIATION: streaming dispatch for an id that DOES match a
// registered Subscription is out of scope for T6 (left to T7, which
// extends this same function/loop). To avoid leaving such a message
// silently unanswered in the meantime, it gets a minimal Error response
// instead of Next/Complete -- doesn't close the connection, doesn't hang,
// just reports "not implemented yet" for that one id.
func handleSubscribe(conn WSConn, sch *gql.Schema, subs map[string]*Subscription, msg wsProtocolMessage) bool {
	writeJSON := func(v any) bool {
		data, err := json.Marshal(v)
		if err != nil {
			return true
		}
		return conn.WriteMessage(wsTextMessage, data) == nil
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

	if _, isSubscription := subs[rootField]; isSubscription {
		// Streaming Subscription dispatch: not this task's scope (T7).
		return writeJSON(map[string]any{
			"id":      msg.Id,
			"type":    wsMsgError,
			"payload": []map[string]any{{"message": "gonest: streaming Subscribe not implemented yet"}},
		})
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
