// ssesingle.go implements graphql-sse's (github.com/enisdenjo/graphql-sse)
// "Single connection mode" PUT (reservation) and GET (the one SSE connection
// a reservation token gets attached to) handlers -- graphql-realtime-
// protocols feature, Milestone 18's T12. reservation.go (T11) already owns
// the token <-> connection bookkeeping this file drives; T13/T14 add the
// POST (start operation) / DELETE (stop operation) handlers that route
// through the same *ReservationRegistry via Route.
package graphql

import (
	"bufio"
	"fmt"
	"sync"
	"time"

	"gonest.dev/gonest/internal/execution"
)

// sseSingleTokenHeader is the header name graphql-sse's own PROTOCOL.md
// documents a client MAY use to transmit its reservation token (the other
// allowed transport is the "token" query parameter, see
// SSESingleConnectHandler): "Token SHOULD be transmitted by the client
// through either: a header value X-GraphQL-Event-Stream-Token, or a search
// parameter token."
const sseSingleTokenHeader = "X-GraphQL-Event-Stream-Token"

// SSESingleReserveHandler builds the PUT /graphql handler implementing
// graphql-sse's Single connection mode reservation handshake: "The client
// requests a reservation for an incoming SSE connection through a PUT HTTP
// request... The server accepts the reservation request by responding with
// 201 (Created) and a reservation token in the body of the response."
//
// PROTOCOL.md does not pin down the exact JSON shape of that body (only
// "token... in the body of the response") -- {"token": "<token>"} is used
// here, the same bare single-field JSON convention this package's own
// error responses already use (e.g. sse.go's {"message": "..."}).
func SSESingleReserveHandler(reg *ReservationRegistry) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		token := reg.Reserve()
		res.Status(201)
		_ = res.Json(map[string]any{"token": token})
	}
}

// SSESingleConnectHandler builds the GET /graphql handler implementing
// graphql-sse's Single connection mode's one SSE connection: the client
// hands back the token a prior PUT reserved (via either the
// X-GraphQL-Event-Stream-Token header or a ?token= query parameter --
// PROTOCOL.md allows both, and this handler accepts whichever the client
// used, checking the header first), and this handler opens (via res.Stream,
// the same bufio.Writer-based mechanics sse.go's own SSEHandler and
// ssedistinct.go's own streamSSEDistinctSubscription use) the single SSE
// connection that will carry every subscription event for that token, via
// reg.Attach.
//
// A missing or never-reserved token (reg.Attach reports ok=false because
// reg.Reserve was never called with it, or its reservation was already
// released) responds with a bare HTTP 404, NOT a GraphQL-shaped error event.
// This is a deliberate difference from ssedistinct.go's own error handling
// (which must always respond as an `event: next` frame, since a native
// EventSource client can never read a non-2xx response's body): unlike
// ssedistinct.go's errors, which are GraphQL EXECUTION failures (a bad
// query/variables) that a client legitimately reached via a real SSE
// connection, this is a TRANSPORT failure -- the connection never found its
// reservation, so there is no SSE stream to carry a `next` frame over in the
// first place, and this handler never even calls res.Stream in that case.
// Plain HTTP status is the only channel left, and is the correct one here.
func SSESingleConnectHandler(reg *ReservationRegistry) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		token := req.Header(sseSingleTokenHeader)
		if token == "" {
			token = req.Queries()["token"]
		}
		if token == "" {
			res.Status(404)
			_ = res.Json(map[string]any{"message": "gonest: missing reservation token (send it via the " + sseSingleTokenHeader + " header or a ?token= query parameter)"})
			return
		}

		// write is registered with reg.Attach BEFORE res.Stream is ever
		// called, so an unknown/unreserved token can still be rejected with
		// a plain 404 below instead of hijacking the connection first: w is
		// nil until res.Stream's own fn actually runs (fasthttp/Fiber only
		// hand the live *bufio.Writer to that fn), so write is a closure
		// over a not-yet-set pointer, guarded by mu the same way
		// ssedistinct.go's own write closure guards its shared
		// *bufio.Writer against concurrent access -- here the concurrent
		// callers are this handler's own heartbeat loop AND whatever
		// POST/DELETE handler later resolves this token via reg.Route
		// (T13/T14).
		var mu sync.Mutex
		var w *bufio.Writer

		// done closes the moment a write over this connection fails --
		// detected either by this handler's own heartbeat below or by a
		// POST-triggered write via reg.Route (T13) -- so the connection can
		// be torn down (and the reservation released) as soon as the
		// disconnect is noticed, without waiting on the next heartbeat
		// tick. Same done/closeOnce idiom sse.go's own SSEHandler and
		// ssedistinct.go's own streamSSEDistinctSubscription already use.
		done := make(chan struct{})
		var closeOnce sync.Once
		closeDone := func() { closeOnce.Do(func() { close(done) }) }

		write := func(frame string) error {
			mu.Lock()
			defer mu.Unlock()
			if w == nil {
				return fmt.Errorf("gonest: SSE single connection for token %q is not open yet", token)
			}
			if _, err := w.WriteString(frame); err != nil {
				closeDone()
				return err
			}
			if err := w.Flush(); err != nil {
				closeDone()
				return err
			}
			return nil
		}

		if ok := reg.Attach(token, write); !ok {
			res.Status(404)
			_ = res.Json(map[string]any{"message": fmt.Sprintf("gonest: unknown or unreserved token %q", token)})
			return
		}

		res.SetHeader("Content-Type", "text/event-stream")
		res.SetHeader("Cache-Control", "no-cache")
		res.SetHeader("Connection", "keep-alive")

		res.Stream(func(bw *bufio.Writer) {
			// A panic here must not crash the process -- same recover
			// contract sse.go's own SSEHandler and ssedistinct.go document
			// for the same reason.
			defer func() { _ = recover() }()

			// The client disconnecting (naturally, or a write failure
			// noticed elsewhere) always ends with the reservation removed
			// from the registry -- a token that outlives its one SSE
			// connection can never be attached to again (Attach only ever
			// succeeds once per Reserve), so leaving it registered would
			// just be a permanent, unusable entry.
			defer reg.Release(token)

			mu.Lock()
			w = bw
			mu.Unlock()

			ticker := time.NewTicker(sseHeartbeatInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if err := write(": ping\n\n"); err != nil {
						return
					}
				}
			}
		})
	}
}
