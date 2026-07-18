package graphql_test

import (
	"bufio"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
)

// newSSESingleTestSchema builds a minimal *gql.Schema exposing one Query
// field ("ping" -> "pong") -- same fixture shape ssedistinct_test.go's own
// newSSEDistinctTestSchema uses, duplicated here (unexported, test-file-
// local) per this package's existing "each test file owns its own
// fixtures" convention.
func newSSESingleTestSchema(t *testing.T) *graphql.Resolver {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})
	res.Declare()
	return res
}

// attachAndDrainToken opens the reservation's GET connection (via
// SSESingleConnectHandler) against a fresh fakeSSEResponder and returns a
// bufio.Reader already positioned to read whatever frames arrive over it --
// same "concurrent read/write, no sequential deadlock" shape T12's own
// TestSSESingleConnectHandler_ValidToken_AttachesConnection documents:
// io.Pipe (fakeSSEResponder's transport) is unbuffered, so the caller MUST
// read from the returned reader concurrently with whatever POST triggers a
// write, never after it, or a write blocks forever with nothing reading yet.
func attachAndDrainToken(t *testing.T, reg *graphql.ReservationRegistry, token string) *bufio.Reader {
	t.Helper()
	connResponder := newFakeSSEResponder("", map[string]string{"token": token})
	connReq, connRes := execution.New(connResponder)
	graphql.SSESingleConnectHandler(reg)(connReq, connRes)

	deadline := time.After(2 * time.Second)
	for {
		if _, ok := reg.Route(token, "warmup"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("reg.Route never became ok -- GET handler did not attach the connection")
		case <-time.After(10 * time.Millisecond):
		}
	}

	return bufio.NewReader(connResponder.pr)
}

func TestSSESingleOperationHandler_QueryOperation_RoutesNextCompleteToToken(t *testing.T) {
	resolver := newSSESingleTestSchema(t)
	sch, err := graphql.Build(resolver.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()

	r := attachAndDrainToken(t, reg, token)

	body := `{"query":"{ ping }","extensions":{"operationId":"op-1"}}`
	postResponder := newFakeSSEResponder("", map[string]string{"token": token})
	postResponder.body = []byte(body)
	postReq, postRes := execution.New(postResponder)

	// The handler's own write of the `next`/`complete` frames blocks on the
	// unbuffered io.Pipe until this test's own readLine calls below read the
	// other end -- so the handler must run concurrently with those reads,
	// never before them (see T12's own doc comment on this exact deadlock
	// shape, attachAndDrainToken's doc comment, and this task's own
	// briefing).
	postDone := make(chan struct{})
	go func() {
		defer close(postDone)
		graphql.SSESingleOperationHandler(sch, map[string]*graphql.Subscription{}, reg)(postReq, postRes)
	}()

	eventLine := readLine(t, r, time.Second)
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}
	dataLine := readLine(t, r, time.Second)
	var next struct {
		Id      string `json:"id"`
		Payload struct {
			Data   map[string]any   `json:"data"`
			Errors []map[string]any `json:"errors"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &next); err != nil {
		t.Fatalf("failed to decode next frame: %v (line=%q)", err, dataLine)
	}
	if next.Id != "op-1" {
		t.Fatalf("next.Id = %q, want %q", next.Id, "op-1")
	}
	if next.Payload.Data["ping"] != "pong" {
		t.Fatalf("next.Payload.Data = %+v, want ping=pong", next.Payload.Data)
	}

	readLine(t, r, time.Second) // blank line terminating the next frame

	completeEventLine := readLine(t, r, time.Second)
	if strings.TrimSpace(completeEventLine) != "event: complete" {
		t.Fatalf("event line = %q, want %q", completeEventLine, "event: complete")
	}
	completeDataLine := readLine(t, r, time.Second)
	var complete struct {
		Id      string          `json:"id"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(completeDataLine), "data: ")), &complete); err != nil {
		t.Fatalf("failed to decode complete frame: %v (line=%q)", err, completeDataLine)
	}
	if complete.Id != "op-1" {
		t.Fatalf("complete.Id = %q, want %q", complete.Id, "op-1")
	}
	if complete.Payload != nil {
		t.Fatalf("complete.Payload = %s, want absent (nil) per PROTOCOL.md", complete.Payload)
	}

	select {
	case <-postDone:
	case <-time.After(time.Second):
		t.Fatal("SSESingleOperationHandler never returned")
	}
	if postResponder.GetStatus() != 202 {
		t.Fatalf("GetStatus() = %d, want 202", postResponder.GetStatus())
	}
}

func TestSSESingleOperationHandler_SubscriptionOperation_RoutesMultipleNextToToken(t *testing.T) {
	sub := graphql.New(func(r *graphql.Resolver) {
		r.Subscription("onCreated", func(s *graphql.Subscription) {
			s.Handler(func(ctx *graphql.GraphqlContext, emit func(any)) {
				for i := 0; i < 3; i++ {
					emit(map[string]any{"n": i})
				}
				<-ctx.Done()
			})
		})
	})
	sub.Declare()
	subs := map[string]*graphql.Subscription{"onCreated": sub.OwnSubscriptions()[0]}

	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()

	r := attachAndDrainToken(t, reg, token)
	// The Handler above blocks on <-ctx.Done() forever after its 3 emits --
	// StopOperation is what closes done, so it must run once this test is
	// through observing all 3 frames, or the goroutine leaks past the test.
	t.Cleanup(func() { reg.StopOperation(token, "op-2") })

	body := `{"query":"{ onCreated }","extensions":{"operationId":"op-2"}}`
	postResponder := newFakeSSEResponder("", map[string]string{"token": token})
	postResponder.body = []byte(body)
	postReq, postRes := execution.New(postResponder)

	graphql.SSESingleOperationHandler(nil, subs, reg)(postReq, postRes)

	if postResponder.GetStatus() != 202 {
		t.Fatalf("GetStatus() = %d, want 202", postResponder.GetStatus())
	}

	for i := 0; i < 3; i++ {
		eventLine := readLine(t, r, time.Second)
		if strings.TrimSpace(eventLine) != "event: next" {
			t.Fatalf("iteration %d: event line = %q, want %q", i, eventLine, "event: next")
		}
		dataLine := readLine(t, r, time.Second)
		var next struct {
			Id      string `json:"id"`
			Payload struct {
				Data map[string]any `json:"data"`
			} `json:"payload"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &next); err != nil {
			t.Fatalf("iteration %d: failed to decode next frame: %v", i, err)
		}
		if next.Id != "op-2" {
			t.Fatalf("iteration %d: next.Id = %q, want %q", i, next.Id, "op-2")
		}
		got, ok := next.Payload.Data["onCreated"].(map[string]any)
		if !ok {
			t.Fatalf("iteration %d: onCreated payload = %+v, want a map", i, next.Payload.Data["onCreated"])
		}
		if int(got["n"].(float64)) != i {
			t.Fatalf("iteration %d: n = %v, want %d", i, got["n"], i)
		}
		readLine(t, r, time.Second) // blank line terminating the frame
	}
}

func TestSSESingleOperationHandler_UnknownToken_RespondsError(t *testing.T) {
	reg := graphql.NewReservationRegistry()

	t.Run("token never reserved", func(t *testing.T) {
		body := `{"query":"{ ping }","extensions":{"operationId":"op-1"}}`
		postResponder := newFakeSSEResponder("", map[string]string{"token": "does-not-exist"})
		postResponder.body = []byte(body)
		postReq, postRes := execution.New(postResponder)

		graphql.SSESingleOperationHandler(nil, map[string]*graphql.Subscription{}, reg)(postReq, postRes)

		if postResponder.GetStatus() != 409 {
			t.Fatalf("GetStatus() = %d, want 409", postResponder.GetStatus())
		}
	})

	t.Run("token reserved but never attached (no GET yet)", func(t *testing.T) {
		token := reg.Reserve()

		body := `{"query":"{ ping }","extensions":{"operationId":"op-1"}}`
		postResponder := newFakeSSEResponder("", map[string]string{"token": token})
		postResponder.body = []byte(body)
		postReq, postRes := execution.New(postResponder)

		graphql.SSESingleOperationHandler(nil, map[string]*graphql.Subscription{}, reg)(postReq, postRes)

		if postResponder.GetStatus() != 409 {
			t.Fatalf("GetStatus() = %d, want 409", postResponder.GetStatus())
		}
	})
}

// TestSSESingleOperationHandler_ConcurrentOperations_NoRace proves multiple
// POST operations against the same token's connection (a Query/Mutation
// and a streaming Subscription racing their own writes) never corrupt the
// underlying *bufio.Writer -- exercised primarily for -race, not for frame
// content (already covered by the tests above).
func TestSSESingleOperationHandler_ConcurrentOperations_NoRace(t *testing.T) {
	resolver := newSSESingleTestSchema(t)
	sch, err := graphql.Build(resolver.OwnQueries(), nil, nil)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	sub := graphql.New(func(r *graphql.Resolver) {
		r.Subscription("onCreated", func(s *graphql.Subscription) {
			s.Handler(func(ctx *graphql.GraphqlContext, emit func(any)) {
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						emit(map[string]any{"ok": true})
					}
				}
			})
		})
	})
	sub.Declare()
	subs := map[string]*graphql.Subscription{"onCreated": sub.OwnSubscriptions()[0]}

	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()

	r := attachAndDrainToken(t, reg, token)

	// Drain concurrently so the writer side never blocks on the unbuffered
	// pipe -- same requirement attachAndDrainToken's own doc comment states.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		buf := make([]byte, 4096)
		for {
			if _, err := r.Read(buf); err != nil {
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"query":"{ ping }","extensions":{"operationId":"q-` + string(rune('a'+i)) + `"}}`
			postResponder := newFakeSSEResponder("", map[string]string{"token": token})
			postResponder.body = []byte(body)
			postReq, postRes := execution.New(postResponder)
			graphql.SSESingleOperationHandler(sch, map[string]*graphql.Subscription{}, reg)(postReq, postRes)
		}(i)
	}

	subBody := `{"query":"{ onCreated }","extensions":{"operationId":"sub-1"}}`
	subResponder := newFakeSSEResponder("", map[string]string{"token": token})
	subResponder.body = []byte(subBody)
	subReq, subRes := execution.New(subResponder)
	graphql.SSESingleOperationHandler(sch, subs, reg)(subReq, subRes)

	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	reg.StopOperation(token, "sub-1")
}

func TestSSESingleReserveHandler_Put_Responds201WithToken(t *testing.T) {
	reg := graphql.NewReservationRegistry()

	responder := newFakeSSEResponder("", nil)
	req, res := execution.New(responder)

	graphql.SSESingleReserveHandler(reg)(req, res)

	if responder.GetStatus() != 201 {
		t.Fatalf("GetStatus() = %d, want 201", responder.GetStatus())
	}

	if responder.jsonBody == nil {
		t.Fatal("no JSON body was written")
	}
	var body struct {
		Token string `json:"token"`
	}
	raw, err := json.Marshal(responder.jsonBody)
	if err != nil {
		t.Fatalf("failed to re-marshal captured JSON body: %v", err)
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body.Token == "" {
		t.Fatal("body.Token is empty, want a reservation token")
	}

	// The token returned must actually be usable to Attach a connection --
	// i.e. it really was registered via reg.Reserve, not just some random
	// string shaped like one.
	if ok := reg.Attach(body.Token, func(string) error { return nil }); !ok {
		t.Fatalf("Attach(%q) = false, want true for a token just reserved via the PUT handler", body.Token)
	}
}

func TestSSESingleConnectHandler_ValidToken_AttachesConnection(t *testing.T) {
	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()

	responder := newFakeSSEResponder("", map[string]string{"token": token})
	req, res := execution.New(responder)

	graphql.SSESingleConnectHandler(reg)(req, res)

	// io.Pipe (fakeSSEResponder's transport) is unbuffered: a write only
	// returns once something reads the other end, so the retry-until-ready
	// write below must run concurrently with the read loop, not before it
	// -- otherwise the first write blocks forever with nothing reading yet.
	r := bufio.NewReader(responder.pr)

	go func() {
		deadline := time.After(2 * time.Second)
		for {
			write, ok := reg.Route(token, "op-1")
			if ok {
				if err := write("event: next\ndata: hello\n\n"); err == nil {
					return
				}
			}
			select {
			case <-deadline:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	line := readLine(t, r, time.Second)
	if strings.TrimSpace(line) != "event: next" {
		t.Fatalf("first line = %q, want %q", line, "event: next")
	}
	dataLine := readLine(t, r, time.Second)
	if strings.TrimSpace(dataLine) != "data: hello" {
		t.Fatalf("data line = %q, want %q", dataLine, "data: hello")
	}
}

func TestSSESingleConnectHandler_UnknownToken_RespondsError(t *testing.T) {
	reg := graphql.NewReservationRegistry()

	t.Run("token never reserved", func(t *testing.T) {
		responder := newFakeSSEResponder("", map[string]string{"token": "does-not-exist"})
		req, res := execution.New(responder)

		graphql.SSESingleConnectHandler(reg)(req, res)

		if responder.GetStatus() != 404 {
			t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
		}
	})

	t.Run("token missing entirely", func(t *testing.T) {
		responder := newFakeSSEResponder("", nil)
		req, res := execution.New(responder)

		graphql.SSESingleConnectHandler(reg)(req, res)

		if responder.GetStatus() != 404 {
			t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
		}
	})
}

// TestSSESingleCancelHandler_ActiveOperationId_StopsThatSubscriptionOnly
// proves a DELETE against one operationId cancels exactly that Subscription
// -- via reg.StopOperation, which closes its ctx.Done() and unblocks its
// Handler -- while a second Subscription active on the SAME token, under a
// different operationId, keeps running (observed here by it still emitting
// a `next` frame after the DELETE).
func TestSSESingleCancelHandler_ActiveOperationId_StopsThatSubscriptionOnly(t *testing.T) {
	stopped := make(chan struct{})
	sub := graphql.New(func(r *graphql.Resolver) {
		r.Subscription("onCreated", func(s *graphql.Subscription) {
			s.Handler(func(ctx *graphql.GraphqlContext, emit func(any)) {
				<-ctx.Done()
				close(stopped)
			})
		})
	})
	sub.Declare()

	other := graphql.New(func(r *graphql.Resolver) {
		r.Subscription("onOther", func(s *graphql.Subscription) {
			s.Handler(func(ctx *graphql.GraphqlContext, emit func(any)) {
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						emit(map[string]any{"ok": true})
					}
				}
			})
		})
	})
	other.Declare()

	subs := map[string]*graphql.Subscription{
		"onCreated": sub.OwnSubscriptions()[0],
		"onOther":   other.OwnSubscriptions()[0],
	}

	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()
	r := attachAndDrainToken(t, reg, token)
	t.Cleanup(func() { reg.StopOperation(token, "op-other") })

	// op-other's ticker-driven emits write frames over the same unbuffered
	// io.Pipe as everything else on this token -- drain them continuously
	// (content is irrelevant to this test) so those writes never block
	// forever with nothing reading.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		buf := make([]byte, 4096)
		for {
			if _, err := r.Read(buf); err != nil {
				return
			}
		}
	}()

	body1 := `{"query":"{ onCreated }","extensions":{"operationId":"op-cancel"}}`
	postResponder1 := newFakeSSEResponder("", map[string]string{"token": token})
	postResponder1.body = []byte(body1)
	postReq1, postRes1 := execution.New(postResponder1)
	graphql.SSESingleOperationHandler(nil, subs, reg)(postReq1, postRes1)

	body2 := `{"query":"{ onOther }","extensions":{"operationId":"op-other"}}`
	postResponder2 := newFakeSSEResponder("", map[string]string{"token": token})
	postResponder2.body = []byte(body2)
	postReq2, postRes2 := execution.New(postResponder2)
	graphql.SSESingleOperationHandler(nil, subs, reg)(postReq2, postRes2)

	// Wait for op-other to actually be registered before op-cancel is
	// DELETEd, so StopOperation(token, "op-cancel") below can never race
	// ahead of StartOperation registering it.
	deadline := time.After(2 * time.Second)
	for {
		if _, ok := reg.Route(token, "op-other"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("op-other never routed")
		case <-time.After(5 * time.Millisecond):
		}
	}

	delResponder := newFakeSSEResponder("", map[string]string{"token": token, "operationId": "op-cancel"})
	delReq, delRes := execution.New(delResponder)
	graphql.SSESingleCancelHandler(reg)(delReq, delRes)

	if delResponder.GetStatus() != 200 {
		t.Fatalf("GetStatus() = %d, want 200", delResponder.GetStatus())
	}

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("op-cancel's Subscription Handler was never unblocked by the DELETE")
	}

	// op-other must still be active: a further StopOperation call on it
	// must report ok=true (it hasn't already finished/been removed).
	if ok := reg.StopOperation(token, "op-other"); !ok {
		t.Fatal("op-other was stopped too -- DELETE for op-cancel must not affect other operations on the same token")
	}
}

// TestSSESingleCancelHandler_UnknownOperationId_RespondsError proves a
// DELETE for an operationId that was never started (or a token that was
// never reserved) responds with a clear HTTP error (404), never panics.
func TestSSESingleCancelHandler_UnknownOperationId_RespondsError(t *testing.T) {
	reg := graphql.NewReservationRegistry()

	t.Run("token never reserved", func(t *testing.T) {
		responder := newFakeSSEResponder("", map[string]string{"token": "does-not-exist", "operationId": "op-1"})
		req, res := execution.New(responder)

		graphql.SSESingleCancelHandler(reg)(req, res)

		if responder.GetStatus() != 404 {
			t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
		}
	})

	t.Run("token reserved, operationId never started", func(t *testing.T) {
		token := reg.Reserve()
		responder := newFakeSSEResponder("", map[string]string{"token": token, "operationId": "does-not-exist"})
		req, res := execution.New(responder)

		graphql.SSESingleCancelHandler(reg)(req, res)

		if responder.GetStatus() != 404 {
			t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
		}
	})

	t.Run("missing operationId query parameter", func(t *testing.T) {
		token := reg.Reserve()
		responder := newFakeSSEResponder("", map[string]string{"token": token})
		req, res := execution.New(responder)

		graphql.SSESingleCancelHandler(reg)(req, res)

		if responder.GetStatus() != 404 {
			t.Fatalf("GetStatus() = %d, want 404", responder.GetStatus())
		}
	})
}

func TestSSESingleConnectHandler_ClientDisconnects_ReleasesReservation(t *testing.T) {
	reg := graphql.NewReservationRegistry()
	token := reg.Reserve()

	responder := newFakeSSEResponder("", map[string]string{"token": token})
	req, res := execution.New(responder)

	graphql.SSESingleConnectHandler(reg)(req, res)

	// Wait for the connection to actually attach.
	deadline := time.After(2 * time.Second)
	for {
		if _, ok := reg.Route(token, "op-1"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("reg.Route never became ok -- GET handler did not attach the connection")
		case <-time.After(10 * time.Millisecond):
		}
	}

	responder.disconnect()

	// A write failure (detected via reg.Route's write func, simulating a
	// POST-triggered write per T13) must release the reservation -- same
	// "next write attempt notices the disconnect" reasoning
	// ssedistinct_test.go's own disconnect test documents.
	deadline = time.After(2 * time.Second)
	for {
		write, ok := reg.Route(token, "op-1")
		if !ok {
			// Already released by the time we got here.
			return
		}
		_ = write("event: next\ndata: x\n\n")

		if _, ok := reg.Route(token, "op-1"); !ok {
			return
		}

		select {
		case <-deadline:
			t.Fatal("reservation was not released after the client disconnected")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
