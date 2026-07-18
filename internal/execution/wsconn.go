package execution

// WSConn is the minimal contract internal/adapter/fiber's WebSocket
// connection wrapper satisfies -- deliberately NOT the real
// *websocket.Conn type (github.com/gofiber/contrib/v3/websocket), so
// packages built on top of it (internal/graphql's WSHandler among them)
// stay adapter-agnostic the same way Responder does for HTTP.
// ReadMessage/WriteMessage mirror that library's own real method
// signatures (confirmed via Context7: gofiber/contrib's own README
// example calls c.ReadMessage()/c.WriteMessage(mt, msg) on a
// *websocket.Conn) -- Params/Query read the upgrade request's own path
// param/query string, captured at connection time. CloseWithCode closes
// with a specific WebSocket close code/reason, needed by the
// graphql-transport-ws protocol's own well-known close codes (4400,
// 4401, 4408, 4409, 4429).
type WSConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
	CloseWithCode(code int, reason string) error
	Params(name string) string
	Query(name string) string
}
