package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	fasthttpws "github.com/fasthttp/websocket"

	"gonest.dev/gonest/internal/adapter/fiber"
	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/schema"
)

type graphqlUserEntity struct {
	Id    int64  `json:"id"`
	Email string `json:"email"`
}

func TestNewApp_GraphqlQuery_RealHTTPDispatch_HappyPath(t *testing.T) {
	zero := &graphqlUserEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	userSchema := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	userSchema.Property(&zero.Id).Integer().Required()
	userSchema.Property(&zero.Email).Email().Required()

	userResolver := graphql.New(func(r *graphql.Resolver) {
		r.Query("user", func(q *graphql.Query) {
			q.Returns(userSchema)
			q.Handler(func(ctx *graphql.GraphqlContext) any {
				return map[string]any{"id": int64(1), "email": "john@example.com"}
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(userResolver)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{
		"query": `{ user { id email } }`,
	})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var out struct {
		Data struct {
			User struct {
				Id    int64  `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", out.Errors)
	}
	if out.Data.User.Id != 1 || out.Data.User.Email != "john@example.com" {
		t.Fatalf("unexpected data: %+v", out.Data)
	}
}

func TestNewApp_GraphqlMutation_InvalidArgs_ProducesGraphqlError(t *testing.T) {
	type createUserArgs struct {
		Email string `json:"email"`
	}
	argsZero := &createUserArgs{}
	argsTyp := reflect.TypeOf(*argsZero)
	t.Cleanup(func() { schema.Deregister(argsTyp) })
	argsSchema := schema.New(argsTyp, uintptr(unsafe.Pointer(argsZero)))
	argsSchema.Property(&argsZero.Email).Email().Required()

	userResolver := graphql.New(func(r *graphql.Resolver) {
		r.Mutation("createUser", func(m *graphql.Mutation) {
			m.Args(argsSchema)
			m.Handler(func(ctx *graphql.GraphqlContext) any {
				var args createUserArgs
				if err := ctx.Args().ParseInto(&args, argsSchema); err != nil {
					panic(err)
				}
				return args.Email
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(userResolver)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{
		"query":     `mutation($email: String!) { createUser(email: $email) }`,
		"variables": map[string]any{"email": "not-an-email"},
	})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Errors []any `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Errors) == 0 {
		t.Fatal("expected a GraphQL error for an invalid email arg, got none")
	}
}

func TestNewApp_GraphqlPath_OverriddenViaAppOptions(t *testing.T) {
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})

	root := module.New(func(m *module.Module) {
		m.Resolvers(res)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{GraphqlPath: "/api/gql"})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	fiberAdapter := app.Adapter().(*fiber.FiberApp)

	body, _ := json.Marshal(map[string]any{"query": `{ ping }`})

	// The default path must NOT be registered when overridden.
	defaultReq := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	defaultReq.Header.Set("Content-Type", "application/json")
	defaultResp, err := fiberAdapter.FiberApp().Test(defaultReq)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer defaultResp.Body.Close()
	if defaultResp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /graphql (default path, should be unregistered) status = %d, want 404", defaultResp.StatusCode)
	}

	// The overridden path must work.
	customReq := httptest.NewRequest(http.MethodPost, "/api/gql", bytes.NewReader(body))
	customReq.Header.Set("Content-Type", "application/json")
	customResp, err := fiberAdapter.FiberApp().Test(customReq)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer customResp.Body.Close()
	if customResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/gql status = %d, want 200", customResp.StatusCode)
	}

	var out struct {
		Data struct {
			Ping string `json:"ping"`
		} `json:"data"`
	}
	if err := json.NewDecoder(customResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Data.Ping != "pong" {
		t.Fatalf("data.ping = %q, want %q", out.Data.Ping, "pong")
	}
}

// newPingOnlyApp builds a minimal app with a single "ping" -> "pong" Query
// resolver on the default /graphql path -- fixture shared by the T15
// dispatcher tests below, which only need to prove ROUTING (which of the 4
// registerGraphql handlers a given method/header/token combination reaches),
// not GraphQL execution correctness (already covered by the tests above and
// by internal/graphql's own per-handler unit tests).
func newPingOnlyApp(t *testing.T) *App {
	t.Helper()
	res := graphql.New(func(r *graphql.Resolver) {
		r.Query("ping", func(q *graphql.Query) {
			q.Handler(func(ctx *graphql.GraphqlContext) any { return "pong" })
		})
	})
	root := module.New(func(m *module.Module) {
		m.Resolvers(res)
	})
	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	return app
}

// listenOnEphemeralPort binds app to a real, ephemeral TCP port (same
// pattern TestRegisterRoute_WebSocketUpgrade_CompletesRealHandshake in
// internal/adapter/fiber/fiber_test.go uses -- app.Test cannot observe a
// WebSocket upgrade or true streaming, per TESTING.md) and returns the
// bound address, blocking until Listen's own onListen fires. Shutdown is
// registered via t.Cleanup.
func listenOnEphemeralPort(t *testing.T, app *App) (addr string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve an ephemeral port: %v", err)
	}
	addr = ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}

	fired := make(chan struct{})
	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- app.Listen(addr, func() { close(fired) })
	}()

	fiberAdapter := app.Adapter().(*fiber.FiberApp)
	t.Cleanup(func() {
		if err := fiberAdapter.FiberApp().Shutdown(); err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
	})

	select {
	case <-fired:
	case err := <-listenErrCh:
		t.Fatalf("Listen returned before onListen fired: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the server to start listening")
	}

	return addr
}

// TestNewApp_GraphqlGet_WebSocketUpgrade_DispatchesWSProtocolHandler proves
// registerGraphql's GET dispatcher picks graphql.WSProtocolHandler for a
// real WebSocket upgrade handshake on the SAME /graphql path POST/PUT/DELETE
// also use -- the "no app.Use interception" central decision design.md
// documents. A real TCP dial is required (app.Test cannot observe a
// protocol upgrade, TESTING.md). connection_init -> connection_ack proves
// the real graphql-transport-ws state machine (not some stub) answered.
func TestNewApp_GraphqlGet_WebSocketUpgrade_DispatchesWSProtocolHandler(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	dialer := fasthttpws.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, resp, err := dialer.Dial("ws://"+addr+"/graphql", nil)
	if err != nil {
		t.Fatalf("expected the WebSocket handshake to succeed, got error: %v", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected status 101 Switching Protocols, got %d", resp.StatusCode)
	}

	if err := conn.WriteMessage(1, []byte(`{"type":"connection_init"}`)); err != nil {
		t.Fatalf("failed to write connection_init: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read connection_ack: %v", err)
	}

	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode message: %v (raw=%s)", err, data)
	}
	if msg.Type != "connection_ack" {
		t.Fatalf("message type = %q, want %q", msg.Type, "connection_ack")
	}
}

// TestNewApp_GraphqlGet_NoUpgradeNoToken_DispatchesSSEDistinctHandler proves
// registerGraphql's GET dispatcher falls through to graphql.SSEDistinctHandler
// for a plain (non-upgrade, no reservation token) GET carrying
// query/operationName as query-string params -- GraphQL-over-HTTP's own GET
// convention. A real TCP dial is used (not app.Test) because the response
// body IS a live SSE stream (res.Stream), and this test reads it frame by
// frame as it arrives rather than waiting for the connection to fully close
// first.
func TestNewApp_GraphqlGet_NoUpgradeNoToken_DispatchesSSEDistinctHandler(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	defer conn.Close()

	req := "GET /graphql?query=" + "%7B%20ping%20%7D" + " HTTP/1.1\r\nHost: " + addr + "\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	r := bufio.NewReader(conn)

	httpResp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatalf("failed to read response headers: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}
	if ct := httpResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	body := bufio.NewReader(httpResp.Body)
	eventLine, err := readLineWithDeadline(t, conn, body, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}

	dataLine, err := readLineWithDeadline(t, conn, body, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}
	if !strings.Contains(dataLine, `"ping":"pong"`) {
		t.Fatalf("data line = %q, want it to contain %q", dataLine, `"ping":"pong"`)
	}
}

// readLineWithDeadline reads a single line from body, bounding the read via
// conn's own SetReadDeadline (bufio.Reader has no deadline of its own) --
// used by the streaming dispatcher tests to avoid hanging forever if a
// dispatch bug ever routes to the wrong (non-responding) handler.
func readLineWithDeadline(t *testing.T, conn net.Conn, body *bufio.Reader, timeout time.Duration) (string, error) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	line, err := body.ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	return line, err
}

// TestNewApp_GraphqlSingleConnectionFlow_RealDial_PutGetPost proves the
// remaining 2 dispatch branches together, end to end, through real app
// routing (not by calling the internal/graphql handlers directly, which is
// already covered by ssesingle_test.go's own unit tests): PUT /graphql
// reserves a token (graphql.SSESingleReserveHandler); GET /graphql with that
// token (header) dispatches to graphql.SSESingleConnectHandler, NOT
// SSEDistinctHandler (proven by the connection staying open with no
// immediate frame, then actually carrying the POST's own next/complete
// pair); POST /graphql with the SAME token plus a body carrying
// extensions.operationId dispatches to graphql.SSESingleOperationHandler,
// NOT the plain JSON graphqlHandler (proven by the 202 status and the
// operationId-tagged frame arriving over the GET's own connection).
func TestNewApp_GraphqlSingleConnectionFlow_RealDial_PutGetPost(t *testing.T) {
	app := newPingOnlyApp(t)
	addr := listenOnEphemeralPort(t, app)

	// PUT -- reserve a token.
	putReq, err := http.NewRequest(http.MethodPut, "http://"+addr+"/graphql", nil)
	if err != nil {
		t.Fatalf("failed to build PUT request: %v", err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT /graphql failed: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT status = %d, want 201", putResp.StatusCode)
	}
	var reserved struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&reserved); err != nil {
		t.Fatalf("failed to decode PUT response: %v", err)
	}
	if reserved.Token == "" {
		t.Fatal("PUT response carried an empty token")
	}

	// GET (with the token) -- open the Single connection mode SSE stream on
	// its own real TCP dial, read frames from it as they arrive.
	getConn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	defer getConn.Close()

	getReqLine := "GET /graphql HTTP/1.1\r\nHost: " + addr + "\r\nX-GraphQL-Event-Stream-Token: " + reserved.Token + "\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n"
	if _, err := getConn.Write([]byte(getReqLine)); err != nil {
		t.Fatalf("failed to write GET request: %v", err)
	}

	// fasthttp/Fiber's SetBodyStreamWriter (Responder.WriteStream's own
	// implementation) does not flush the GET response's HEADERS to the
	// wire until the FIRST byte is actually written to the stream --
	// SSESingleConnectHandler's own connect loop writes nothing at all
	// until either the heartbeat ticker fires (15s) or an external write
	// arrives via reg.Route (a POST). Reading the GET response headers
	// must therefore never be sequenced BEFORE the POST that is meant to
	// trigger that very first write -- doing so deadlocks until the 15s
	// heartbeat instead (empirically confirmed while writing this test).
	// The POST is fired in its own goroutine, concurrently with reading
	// the GET response, exactly to break that ordering trap.
	postBody, _ := json.Marshal(map[string]any{
		"query": `{ ping }`,
		"extensions": map[string]any{
			"operationId": "op-1",
		},
	})
	postStatusCh := make(chan int, 1)
	postErrCh := make(chan error, 1)
	go func() {
		// A tiny window exists between the GET request's bytes reaching the
		// server and SSESingleConnectHandler's own reg.Attach call actually
		// registering this token's write function (Attach runs before
		// res.Stream is ever called) -- same race
		// ssesingle_test.go's own attachAndDrainToken helper polls reg.Route
		// for internally; this test has no direct access to reg (it's
		// private to registerGraphql), so a short, generous sleep stands in
		// for that same synchronization instead of asserting on a fixed
		// race window.
		time.Sleep(200 * time.Millisecond)

		postReq, err := http.NewRequest(http.MethodPost, "http://"+addr+"/graphql", bytes.NewReader(postBody))
		if err != nil {
			postErrCh <- err
			return
		}
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("X-GraphQL-Event-Stream-Token", reserved.Token)
		postResp, err := http.DefaultClient.Do(postReq)
		if err != nil {
			postErrCh <- err
			return
		}
		defer postResp.Body.Close()
		postStatusCh <- postResp.StatusCode
	}()

	getBufReader := bufio.NewReader(getConn)
	getHTTPResp, err := http.ReadResponse(getBufReader, nil)
	if err != nil {
		t.Fatalf("failed to read GET response headers: %v", err)
	}
	if getHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getHTTPResp.StatusCode)
	}
	if ct := getHTTPResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("GET Content-Type = %q, want %q", ct, "text/event-stream")
	}
	getBody := bufio.NewReader(getHTTPResp.Body)

	select {
	case status := <-postStatusCh:
		if status != http.StatusAccepted {
			t.Fatalf("POST status = %d, want 202 (proves dispatch to SSESingleOperationHandler, not the plain JSON handler)", status)
		}
	case err := <-postErrCh:
		t.Fatalf("POST /graphql failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the POST to complete")
	}

	// The GET's own SSE connection must now carry op-1's next/complete pair.
	eventLine, err := readLineWithDeadline(t, getConn, getBody, 3*time.Second)
	if err != nil {
		t.Fatalf("failed to read event line from the GET connection: %v", err)
	}
	if strings.TrimSpace(eventLine) != "event: next" {
		t.Fatalf("first line = %q, want %q", eventLine, "event: next")
	}
	dataLine, err := readLineWithDeadline(t, getConn, getBody, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read data line from the GET connection: %v", err)
	}
	var next struct {
		Id      string `json:"id"`
		Payload struct {
			Data map[string]any `json:"data"`
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
}
