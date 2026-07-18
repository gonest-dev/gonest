package graphql

import (
	"bufio"
	"encoding/json"
	"fmt"

	gql "github.com/graphql-go/graphql"

	"gonest.dev/gonest/internal/execution"
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
// ends. A root field that resolves to a registered Subscription is T10's
// own scope (real streaming, multiple Next frames over time) -- see the
// SPEC_DEVIATION below on this function's placeholder behavior for that
// case.
//
// GraphQL-over-HTTP SSE's own constraint (PROTOCOL.md, and this task's own
// brief): a client using the native EventSource API can never read a non-2xx
// response's body, so ANY error -- a syntactically invalid query, an unknown
// field, a Subscription hit before T10 -- must still arrive as a well-formed
// `event: next` frame carrying the error in `data.errors`, never as a bare
// HTTP 4xx/5xx status. This function never calls res.Status at all,
// including for its own ?variables= JSON-decode failure.
func SSEDistinctHandler(sch *gql.Schema, subs map[string]*Subscription) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
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

		if _, isSubscription := subs[rootField]; isSubscription {
			// SPEC_DEVIATION (explicitly allowed by T9's brief): real
			// streaming dispatch for a Subscription root field over Distinct
			// SSE is T10's own scope. Minimum acceptable behavior here is a
			// single, well-formed next+complete pair carrying an error --
			// never hanging, never panicking, never a bare HTTP error status.
			writeSSEDistinctResult(res, nil, []map[string]any{
				{"message": fmt.Sprintf("gonest: subscription %q streaming is not yet supported over GraphQL-over-HTTP SSE", rootField)},
			})
			return
		}

		data, errs := Execute(sch, query, variables, operationName)
		writeSSEDistinctResult(res, data, errs)
	}
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
func writeSSEDistinctResult(res *execution.Response, data any, errs []map[string]any) {
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
