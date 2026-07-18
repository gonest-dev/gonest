package graphql_test

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
)

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
