// ssesingle.go implements graphql-sse's (github.com/enisdenjo/graphql-sse)
// "Single connection mode" PUT (reservation), GET (the one SSE connection a
// reservation token gets attached to) and POST (start operation) handlers --
// graphql-realtime-protocols feature, Milestone 18's T12/T13. reservation.go
// (T11) already owns the token <-> connection bookkeeping this file drives;
// T14 adds the remaining DELETE (stop operation) handler, routing through
// the same *ReservationRegistry via Route/StopOperation.
package graphql

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	gql "github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/validate"
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

// sseSingleOperationBody is the POST /graphql body shape graphql-sse's
// Single connection mode uses to start an operation over an already-open
// reservation: the standard GraphQL-over-HTTP triple (query/variables/
// operationName -- identical to graphqlRequestBody, internal/app/graphql.go,
// and wsSubscribePayload, wsprotocol.go) plus `extensions.operationId`,
// which PROTOCOL.md documents as the transport this operation's Next/
// Complete frames must be routed under: "The unique operation ID SHOULD be
// sent through the extensions parameter of the GraphQL request inside the
// operationId field."
type sseSingleOperationBody struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
	Extensions    struct {
		OperationId string `json:"operationId"`
	} `json:"extensions"`
}

// sseSingleFrame is the wire shape of every event graphql-sse's Single
// connection mode routes over the one SSE connection: `{"id": "<operation
// id>", "payload": <ExecutionResult>}` for `next`, and bare `{"id": "..."}`
// (Payload omitted via omitempty) for `complete` -- PROTOCOL.md's own
// TypeScript shape, confirmed by direct reading, not fabricated: a
// `complete` message carries no `payload` field at all.
type sseSingleFrame struct {
	Id      string          `json:"id"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// writeSSESingleFrame marshals event/frame and writes it as one SSE frame
// (`event: <event>\ndata: <json>\n\n`) via write -- the single helper both
// the Query/Mutation and Subscription branches of
// SSESingleOperationHandler funnel every next/complete write through, so
// the two branches can never drift on wire shape.
func writeSSESingleFrame(write func(string) error, event string, frame sseSingleFrame) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return write(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
}

// SSESingleCancelHandler builds the DELETE /graphql handler implementing
// graphql-sse's Single connection mode operation termination: "This is done
// by sending a DELETE HTTP request encoding the unique operation ID inside
// the URL's search parameters behind the operationId key... The HTTP
// request MUST contain the matching reservation token." The token is read
// the same way every other Single connection mode handler reads it
// (X-GraphQL-Event-Stream-Token header, falling back to a ?token= query
// parameter), and operationId is read from the ?operationId= query
// parameter per that same PROTOCOL.md excerpt.
//
// reg.StopOperation(token, operationId) (already implemented, reservation.go
// T11/T13) is the exact primitive this delegates to -- it removes and
// closes only that operationId's done channel on token's reservation,
// leaving every other operationId active on the same token (or on any
// other token) untouched, so cancelling one Subscription can never
// interrupt another one multiplexed over the same SSE connection.
//
// ok=false (StopOperation found nothing to stop -- either operationId was
// never started, already completed on its own, or was already cancelled by
// a prior DELETE, OR token itself is unknown) responds with a plain HTTP
// 404: same "transport-level, not a GraphQL execution error" reasoning
// SSESingleConnectHandler's own doc comment gives for its 404s -- there is
// no GraphQL result to report here at all, just "nothing found to cancel".
// This makes a DELETE against an already-finished operationId NOT
// idempotent in its HTTP response (a second DELETE 404s), but it IS
// idempotent in effect (never double-closes a channel, never panics) --
// StopOperation's own contract already guarantees that. A successful
// cancellation responds 200 with an empty JSON body; PROTOCOL.md does not
// specify a response shape for DELETE, so the convention is picked to match
// this file's own PUT/POST bare-JSON responses.
func SSESingleCancelHandler(reg *ReservationRegistry) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		token := req.Header(sseSingleTokenHeader)
		if token == "" {
			token = req.Queries()["token"]
		}
		operationId := req.Queries()["operationId"]

		if ok := reg.StopOperation(token, operationId); !ok {
			res.Status(404)
			_ = res.Json(map[string]any{"message": fmt.Sprintf("gonest: no active operation %q for reservation token %q", operationId, token)})
			return
		}

		res.Status(200)
		_ = res.Json(map[string]any{})
	}
}

// SSESingleOperationHandler builds the POST /graphql handler implementing
// graphql-sse's Single connection mode operation start: "separate HTTP
// requests solicit GraphQL operations... successful responses (execution
// results) get accepted with a 202 (Accepted) and are then streamed through
// the single SSE connection." This function assumes it has already been
// routed to correctly (a future task -- distinguishing this POST from the
// plain GraphQL-over-HTTP POST /graphql body is registerGraphql's own T15
// concern, not this handler's).
//
// The reservation token is read the same way SSESingleConnectHandler reads
// it (X-GraphQL-Event-Stream-Token header, falling back to a ?token= query
// parameter), and reg.Route resolves it -- together with the body's
// extensions.operationId -- to the write func of that token's single SSE
// connection. ok=false (no GET has attached that token's connection yet)
// responds 409 (Conflict): the reservation exists in principle but there is
// currently nothing to deliver the result to, distinct from the PUT/GET
// handlers' own 404 (unknown token entirely).
//
// Query/Mutation vs Subscription dispatch reuses wsRootFieldName, the exact
// AST-parsing helper wsprotocol.go's WS transport and ssedistinct.go's SSE
// Distinct transport already use -- never re-implemented here. A
// Query/Mutation runs once through Execute and is answered with exactly one
// `next` frame (payload {data, errors}) followed by one `complete` frame (no
// payload), matching PROTOCOL.md's shape verbatim. A Subscription instead
// starts sub.HandlerFunc() in its own goroutine -- registered under
// (token, operationId) via reg.StartOperation so a future DELETE handler
// (T14) can cancel this specific stream -- and every emit(value) becomes its
// own `next` frame (payload {data: {<fieldName>: value}}, no `errors` key,
// same shape ssedistinct.go's own streamSSEDistinctSubscription already
// uses) routed through the same write; no `complete` frame is ever sent by
// this handler for a Subscription -- graphql-sse's own protocol leaves that
// to an explicit DELETE (T14) or the reservation's own connection dropping
// (reg.Release, which now also closes every still-registered operation's
// done channel -- see reservation.go).
//
// This handler always responds 202 BEFORE the operation's real result is
// known -- PROTOCOL.md's own contract ("get accepted with a 202... and are
// then streamed through the single SSE connection"), never through this
// POST response's own body.
func SSESingleOperationHandler(sch *gql.Schema, subs map[string]*Subscription, reg *ReservationRegistry) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		// Same BodySource wiring graphqlHandler (internal/app/graphql.go)
		// does for the plain GraphQL-over-HTTP POST /graphql -- this route
		// is (or, once T15 lands, will be) dispatched directly rather than
		// through registerRoutes' own withRoute closure, so req.Body().Raw()
		// needs it wired here explicitly.
		req.WithSources(
			validate.NewParamsSource(req),
			validate.NewQuerySource(req),
			validate.NewHeadersSource(req),
			execution.NewBodySource(
				req,
				func() execution.Parseable { return validate.NewJSONBodySource(req) },
				func(onFile func(*execution.FormFile) error) execution.Parseable {
					return validate.NewFormBodySource(req, onFile)
				},
			),
		)

		token := req.Header(sseSingleTokenHeader)
		if token == "" {
			token = req.Queries()["token"]
		}

		var body sseSingleOperationBody
		if err := json.Unmarshal(req.Body().Raw(), &body); err != nil {
			res.Status(400)
			_ = res.Json(map[string]any{"message": "gonest: invalid GraphQL request body: " + err.Error()})
			return
		}

		operationId := body.Extensions.OperationId

		write, ok := reg.Route(token, operationId)
		if !ok {
			res.Status(409)
			_ = res.Json(map[string]any{"message": fmt.Sprintf("gonest: no open SSE connection for reservation token %q -- PUT+GET a reservation before POSTing an operation", token)})
			return
		}

		rootField, _ := wsRootFieldName(body.Query, body.OperationName)

		if sub, isSubscription := subs[rootField]; isSubscription {
			res.Status(202)
			_ = res.Json(map[string]any{})

			done := make(chan struct{})
			reg.StartOperation(token, operationId, done)

			var args execution.Parseable
			if sub.ArgsSchema() != nil {
				args = validate.NewGraphqlArgsSource(body.Variables)
			}
			ctx := NewGraphqlContext(args, done)

			emit := func(v any) {
				payload, err := json.Marshal(map[string]any{"data": map[string]any{rootField: v}})
				if err != nil {
					return
				}
				if err := writeSSESingleFrame(write, "next", sseSingleFrame{Id: operationId, Payload: payload}); err != nil {
					// The single SSE connection is gone -- stop just this
					// operation's own stream; any other operation active on
					// the same token is unaffected, and the GET handler's
					// own heartbeat/reg.Release will separately notice the
					// connection is dead.
					reg.StopOperation(token, operationId)
				}
			}

			go func() {
				// Self-cleanup for a Handler that returns on its own
				// (stream ended without an explicit DELETE or a connection
				// drop): StopOperation is a no-op if this operationId was
				// already removed by either of those, so this never
				// double-closes done.
				defer reg.StopOperation(token, operationId)
				sub.HandlerFunc()(ctx, emit)
			}()

			return
		}

		data, errs := Execute(sch, body.Query, body.Variables, body.OperationName)

		res.Status(202)
		_ = res.Json(map[string]any{})

		payload, err := json.Marshal(map[string]any{"data": data, "errors": errs})
		if err != nil {
			payload = []byte(`{"data":null,"errors":[{"message":"gonest: failed to marshal result"}]}`)
		}
		if err := writeSSESingleFrame(write, "next", sseSingleFrame{Id: operationId, Payload: payload}); err != nil {
			return
		}
		_ = writeSSESingleFrame(write, "complete", sseSingleFrame{Id: operationId})
	}
}
