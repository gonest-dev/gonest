package graphql

import (
	"time"

	"gonest.dev/gonest/internal/execution"
)

// wsTextMessage is the WebSocket text-frame opcode (RFC 6455 -- 0x1),
// mirrored by every Go WebSocket library (gorilla/websocket,
// fasthttp/websocket) as a named constant with this same value; hardcoded
// here rather than importing fasthttp/websocket just for the constant,
// since WSConn (below) already keeps this package adapter-agnostic. Shared
// by every real-transport handler in this package (wsprotocol.go).
const wsTextMessage = 1

// WSConn is an alias of execution.WSConn -- moved there
// (graphql-realtime-protocols feature, Milestone 18, T2) so
// internal/execution can also depend on it (namely for the
// graphql-transport-ws protocol's own close-code handling) without an
// import cycle back into internal/graphql. Kept here under its original
// name so wsprotocol.go and its tests need no other change.
type WSConn = execution.WSConn

// sseHeartbeatInterval is how often the SSE transports (sse_distinct.go,
// sse_single.go) write a comment-only frame (`: ping\n\n`, ignored by every
// SSE client per the spec) purely to detect a disconnected client -- a
// write failure only surfaces on an actual write attempt, so a
// Subscription whose Handler never itself emits (or emits rarely) would
// otherwise never notice its client is gone until something else tries to
// write.
const sseHeartbeatInterval = 15 * time.Second
