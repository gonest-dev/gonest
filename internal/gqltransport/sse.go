// Package gqltransport implements the Subscription transports
// graphql-go/graphql itself never shipped (graphql-support feature,
// Milestone 17, D7/context.md's research: graphql-go's own subscription
// support only ever resolved the SYNTAX, never an execution/streaming
// engine) -- SSE (this file) and WebSocket (ws.go). Both bypass
// graphql-go's own execution engine entirely: they call a
// *gqlresolver.Subscription's own HandlerFunc directly, emitting values
// over the wire as emit is called, exactly like internal/graphqlgen's
// Query/Mutation Resolve callbacks call HandlerFunc directly instead of
// going through graphql-go's own field-resolution machinery.
package gqltransport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/validate"
)

// sseHeartbeatInterval is how often SSEHandler writes a comment-only frame
// (`: ping\n\n`, ignored by every SSE client per the spec) purely to
// detect a disconnected client -- a write failure only surfaces on an
// actual write attempt, so a Subscription whose Handler never itself
// emits (or emits rarely) would otherwise never notice its client is gone
// until something else tries to write.
const sseHeartbeatInterval = 15 * time.Second

// SSEHandler builds the GET /graphql/stream/:name HTTP handler, serving
// Server-Sent Events for whichever Subscription subs[name] resolves to.
//
// Args format (SPEC_DEVIATION -- not decided by design.md, decided HERE):
// a GET request has no body, so a Subscription's Args (if any) are read
// from a single `?args=<JSON object>` query parameter -- e.g.
// `/graphql/stream/onUserCreated?args={"userId":1}` -- rather than one
// query parameter per arg field (which would require a NEW, source-
// specific tag family/coercion path, same class of work design.md's Tech
// Decisions already deferred for Refine's params/query/form/headers
// scope). Reuses validate.NewGraphqlArgsSource unchanged: the JSON object
// decodes into the exact map[string]any shape that function already
// expects (same shape graphql-go's own already-decoded ResolveParams.Args
// has for POST /graphql).
func SSEHandler(subs map[string]*gqlresolver.Subscription) func(req *execution.Request, res *execution.Response) {
	return func(req *execution.Request, res *execution.Response) {
		name := req.Param("name")
		sub, ok := subs[name]
		if !ok {
			res.Status(404)
			res.Json(map[string]any{"message": fmt.Sprintf("gonest: no subscription named %q", name)})
			return
		}

		var argsParseable execution.Parseable
		if sub.ArgsSchema() != nil {
			var parsed map[string]any
			if raw := req.Queries()["args"]; raw != "" {
				if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
					res.Status(400)
					res.Json(map[string]any{"message": "gonest: invalid ?args= JSON: " + err.Error()})
					return
				}
			}
			argsParseable = validate.NewGraphqlArgsSource(parsed)
		}

		res.SetHeader("Content-Type", "text/event-stream")
		res.SetHeader("Cache-Control", "no-cache")
		res.SetHeader("Connection", "keep-alive")

		// done closes the moment a write to the client fails (detected
		// either by the Handler's own emit calls or by the heartbeat
		// below) -- ctx.Done() IS this channel, so a cooperative Handler
		// observing <-ctx.Done() can release its own resources (spec.md
		// AC5) as soon as the disconnect is noticed, not only once this
		// whole function eventually returns.
		done := make(chan struct{})
		var closeOnce sync.Once
		closeDone := func() { closeOnce.Do(func() { close(done) }) }
		ctx := gqlresolver.NewGraphqlContext(argsParseable, done)

		res.Stream(func(w *bufio.Writer) {
			// A panic inside Handler (or this function) is a recognized,
			// NOT fully solved gap (spec.md's Edge Cases, design.md's
			// Error Handling Strategy): minimum acceptable behavior is
			// recovering here so the goroutine for THIS connection ends
			// gracefully without crashing the process -- it does not
			// (yet) notify the client with a GraphQL-shaped error frame
			// (Deferred Idea in context.md).
			defer func() { _ = recover() }()

			// w is shared between Handler's own goroutine (via emit) and
			// this function's heartbeat loop -- bufio.Writer is not safe
			// for concurrent use, so every write goes through write,
			// serialized by mu.
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
				payload, err := json.Marshal(v)
				if err != nil {
					return
				}
				if err := write(fmt.Sprintf("data: %s\n\n", payload)); err != nil {
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
			for {
				select {
				case <-handlerDone:
					closeDone()
					return
				case <-ticker.C:
					if err := write(": ping\n\n"); err != nil {
						closeDone()
						return
					}
				}
			}
		})
	}
}
