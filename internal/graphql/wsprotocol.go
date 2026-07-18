package graphql

import (
	"encoding/json"
	"time"

	gql "github.com/graphql-go/graphql"
)

// wsConnectionInitTimeout bounds how long WSProtocolHandler waits for the
// client's first ConnectionInit message before closing with 4408 --
// graphql-transport-ws protocol requirement. v1 does not expose this as
// config.
const wsConnectionInitTimeout = 3 * time.Second

// graphql-transport-ws message types (github.com/enisdenjo/graphql-ws's own
// PROTOCOL.md) -- only the ones this handshake-only task (T5) dispatches on;
// Subscribe/Next/Error/Complete are read/decoded by future tasks that
// extend this same loop.
const (
	wsMsgConnectionInit = "connection_init"
	wsMsgConnectionAck  = "connection_ack"
	wsMsgPing           = "ping"
	wsMsgPong           = "pong"
)

// graphql-transport-ws well-known WebSocket close codes.
const (
	wsCloseInvalidMessage        = 4400
	wsCloseUnauthorized          = 4401
	wsCloseConnectionInitTimeout = 4408
	wsCloseTooManyInitRequests   = 4429
)

// wsProtocolMessage is the wire shape shared by every graphql-transport-ws
// message -- only `type` is needed for this task's dispatch (Subscribe's
// `id`/`payload` fields are added by future tasks that extend this same
// loop).
type wsProtocolMessage struct {
	Type string `json:"type"`
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
