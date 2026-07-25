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

// SSEDistinctHandler builds the GET /graphql handler implementing
// graphql-sse's (github.com/enisdenjo/graphql-sse) "Distinct connections
// mode": a normal GraphQL-over-HTTP GET request (query/variables/
// operationName carried as query-string params, per
// https://github.com/graphql/graphql-over-http) whose response IS the SSE
// stream, instead of a single JSON body -- graphql-realtime-protocols
// feature, Milestone 18's T9.
//
// T9's own scope is Query/Mutation only: the root field is resolved via
// wsRootFieldName (same AST-parsing helper wsprotocol.go's WS transport
// already uses, reused here rather than hand-rolled again) and dispatched
// through Execute exactly like every other transport (GraphQL-over-HTTP
// POST, WS's single-result Subscribe path) -- writing exactly one `event:
// next` frame (data: {"data":...,"errors":...}, same shape Execute already
// returns) followed by one `event: complete` frame, then the connection
// ends.
//
// T10 adds the other branch: when the root field IS a registered
// Subscription, the connection stays open -- sub.HandlerFunc() runs in its
// own goroutine (same pattern sse.go's own SSEHandler and wsprotocol.go's
// own handleSubscribe already use: emit writes an `event: next` frame per
// call, a periodic heartbeat comment-frame detects a dead connection via
// write failure since a quiet Handler would otherwise never notice, and
// ctx.Done() closes the moment either detects the client is gone so a
// cooperative Handler can release its own resources promptly) -- until the
// Handler itself returns (naturally, or because it observed <-ctx.Done()),
// at which point exactly one final `event: complete` frame is written and
// the connection ends. Unlike Query/Mutation's single next+complete pair,
// this branch may write an unbounded number of next frames over time.
//
// GraphQL-over-HTTP SSE's own constraint (PROTOCOL.md, and this task's own
// brief): a client using the native EventSource API can never read a non-2xx
// response's body, so ANY error -- a syntactically invalid query, an unknown
// field, a Subscription hit before T10 -- must still arrive as a well-formed
// `event: next` frame carrying the error in `data.errors`, never as a bare
// HTTP 4xx/5xx status. This function never calls res.Status at all,
// including for its own ?variables= JSON-decode failure.
func SSEDistinctHandler(sch *gql.Schema, subs map[string]*Subscription) func(c *execution.HttpContext) {
	return func(c *execution.HttpContext) {
		req, res := c.Request(), c.Response()
		queries := req.Queries()
		query := queries["query"]
		operationName := queries["operationName"]

		var variables map[string]any
		if raw := queries["variables"]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &variables); err != nil {
				writeSSEDistinctResult(res, nil, []map[string]any{
					{"message": "gonest: invalid ?variables= JSON: " + err.Error()},
				})
				return
			}
		}

		// A query that fails to parse (rootErr != nil) is deliberately NOT
		// branched into its own error response here: rootField simply stays
		// "" in that case, which can never match a real subs key, so control
		// falls through to Execute below -- which re-parses the exact same
		// query internally (gql.Do) and produces the identical GraphQL-shaped
		// parse error in its own errors slice. Reusing that path avoids
		// hand-rolling a second, possibly-divergent error shape for the same
		// failure.
		rootField, _ := wsRootFieldName(query, operationName)

		if sub, isSubscription := subs[rootField]; isSubscription {
			streamSSEDistinctSubscription(res, sub, rootField, variables)
			return
		}

		data, errs := Execute(sch, query, variables, operationName)
		writeSSEDistinctResult(res, data, errs)
	}
}

// streamSSEDistinctSubscription keeps the SSE Distinct connection open for
// a matched Subscription root field: sub.HandlerFunc() runs in its own
// goroutine, one `event: next` frame per emit(value) call, until the
// Handler returns (naturally, or because ctx.Done() fired -- either the
// heartbeat or an emit's own write failure closing it upon detecting the
// client is gone), at which point one final `event: complete` frame is
// written. Mechanically this is sse.go's own SSEHandler (heartbeat/mutex/
// disconnect-detection) copy+adapted, not reimplemented: same done-channel/
// closeOnce/mu-guarded write/heartbeat-ticker shape, only the wire frame
// format differs (`event: next\ndata: {"data":{<fieldName>:<value>}}\n\n`
// here vs SSEHandler's own bare `data: <value>\n\n`, plus the terminating
// `event: complete\ndata: \n\n` frame Distinct connections require that
// SSEHandler's own long-lived-per-Subscription endpoint never needed).
func streamSSEDistinctSubscription(res *execution.Reply, sub *Subscription, rootField string, variables map[string]any) {
	res.SetHeader("Content-Type", "text/event-stream")
	res.SetHeader("Cache-Control", "no-cache")
	res.SetHeader("Connection", "keep-alive")

	var argsParseable execution.Parseable
	if sub.ArgsSchema() != nil {
		argsParseable = validate.NewGraphqlArgsSource(variables)
	}

	// done closes the moment a write to the client fails (detected either
	// by the Handler's own emit calls or by the heartbeat below) --
	// ctx.Done() IS this channel, same contract sse.go's own SSEHandler
	// documents, so a cooperative Handler observing <-ctx.Done() can
	// release its own resources as soon as the disconnect is noticed, not
	// only once this whole function eventually returns.
	done := make(chan struct{})
	var closeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }
	ctx := NewGraphqlContext(argsParseable, done)

	res.Stream(func(w *bufio.Writer) {
		// A panic inside Handler (or this function) must not crash the
		// process -- same recover contract sse.go's own SSEHandler and
		// this file's own writeSSEDistinctResult document for the same
		// reason.
		defer func() { _ = recover() }()

		// w is shared between the Handler's own goroutine (via emit) and
		// this function's heartbeat loop -- bufio.Writer is not safe for
		// concurrent use, so every write goes through write, serialized by
		// mu.
		var mu sync.Mutex
		write := func(frame string) error {
			mu.Lock()
			defer mu.Unlock()
			if _, err := w.WriteString(frame); err != nil {
				return err
			}
			return w.Flush()
		}

		emit := func(v any) {
			payload, err := json.Marshal(map[string]any{"data": map[string]any{rootField: v}})
			if err != nil {
				return
			}
			if err := write(fmt.Sprintf("event: next\ndata: %s\n\n", payload)); err != nil {
				closeDone()
			}
		}

		handlerDone := make(chan struct{})
		go func() {
			defer close(handlerDone)
			sub.HandlerFunc()(ctx, emit)
		}()

		ticker := time.NewTicker(sseHeartbeatInterval)
		defer ticker.Stop()
	loop:
		for {
			select {
			case <-handlerDone:
				closeDone()
				break loop
			case <-ticker.C:
				if err := write(": ping\n\n"); err != nil {
					closeDone()
					break loop
				}
			}
		}

		// Handler has returned (naturally or via disconnect) -- the
		// terminating `complete` frame, per graphql-sse's own PROTOCOL.md
		// (see writeSSEDistinctResult's own doc comment on the required
		// empty `data: ` field). Best-effort: if the client is already
		// gone this write simply fails and is ignored, same as every other
		// write in this function past disconnect.
		_ = write("event: complete\ndata: \n\n")
	})
}

// writeSSEDistinctResult sets the SSE response headers and streams exactly
// one `event: next` frame (the {data, errors} result, same shape Execute
// already returns) followed by one `event: complete` frame, via res.Stream
// -- reusing sse.go's own bufio.Writer-based frame-writing mechanics
// (WriteString + Flush, first write failure aborts the rest) rather than
// reimplementing them.
//
// The terminating `complete` event includes an explicit, empty `data: `
// field (`event: complete\ndata: \n\n`), per graphql-sse's own PROTOCOL.md
// (github.com/enisdenjo/graphql-sse, "Include an empty `data: ` field when
// sending the message to a client that uses EventSource. If the field is
// omitted, the complete event won't trigger the listener.") -- confirmed by
// reading PROTOCOL.md directly. Omitting the field (an earlier version of
// this function did) is a real bug, not a neutral choice: browser
// EventSource never fires its listener for an event with no `data:` line at
// all.
func writeSSEDistinctResult(res *execution.Reply, data any, errs []map[string]any) {
	res.SetHeader("Content-Type", "text/event-stream")
	res.SetHeader("Cache-Control", "no-cache")
	res.SetHeader("Connection", "keep-alive")

	res.Stream(func(w *bufio.Writer) {
		// A panic here (e.g. an unexpected marshal failure) must not crash
		// the process -- same recover contract sse.go's own SSEHandler
		// documents for the same reason.
		defer func() { _ = recover() }()

		payload, err := json.Marshal(map[string]any{"data": data, "errors": errs})
		if err != nil {
			payload = []byte(`{"data":null,"errors":[{"message":"gonest: failed to marshal result"}]}`)
		}

		if _, err := w.WriteString(fmt.Sprintf("event: next\ndata: %s\n\n", payload)); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		if _, err := w.WriteString("event: complete\ndata: \n\n"); err != nil {
			return
		}
		_ = w.Flush()
	})
}
