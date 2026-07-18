package graphql

import (
	"sync"
	"testing"
)

func TestReservationRegistry_Reserve_ReturnsUniqueToken(t *testing.T) {
	registry := NewReservationRegistry()

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := registry.Reserve()
		if token == "" {
			t.Fatalf("Reserve returned empty token on iteration %d", i)
		}
		if seen[token] {
			t.Fatalf("Reserve returned duplicate token %q on iteration %d", token, i)
		}
		seen[token] = true
	}
}

func TestReservationRegistry_AttachThenRoute_ReturnsWriteFunc(t *testing.T) {
	registry := NewReservationRegistry()
	token := registry.Reserve()

	var got string
	writeFn := func(frame string) error {
		got = frame
		return nil
	}

	if ok := registry.Attach(token, writeFn); !ok {
		t.Fatalf("Attach(%q) = false, want true for a reserved token", token)
	}

	write, ok := registry.Route(token, "op-1")
	if !ok {
		t.Fatalf("Route(%q) ok = false, want true after Attach", token)
	}
	if write == nil {
		t.Fatalf("Route(%q) returned nil write func", token)
	}

	if err := write("hello"); err != nil {
		t.Fatalf("write(\"hello\") returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("write func routed to wrong connection: got %q, want %q", got, "hello")
	}
}

func TestReservationRegistry_RouteBeforeAttach_ReturnsNotOk(t *testing.T) {
	registry := NewReservationRegistry()

	t.Run("reserved but not attached", func(t *testing.T) {
		token := registry.Reserve()

		if _, ok := registry.Route(token, "op-1"); ok {
			t.Fatalf("Route(%q) ok = true, want false before Attach", token)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		if _, ok := registry.Route("does-not-exist", "op-1"); ok {
			t.Fatalf("Route on unknown token ok = true, want false")
		}
	})

	t.Run("attach on unknown token fails", func(t *testing.T) {
		if ok := registry.Attach("does-not-exist", func(string) error { return nil }); ok {
			t.Fatalf("Attach on unknown token ok = true, want false")
		}
	})

	t.Run("released token is no longer routable", func(t *testing.T) {
		token := registry.Reserve()
		registry.Attach(token, func(string) error { return nil })
		registry.Release(token)

		if _, ok := registry.Route(token, "op-1"); ok {
			t.Fatalf("Route(%q) ok = true after Release, want false", token)
		}
	})
}

func TestReservationRegistry_ConcurrentReserveAttachRoute_NoRace(t *testing.T) {
	registry := NewReservationRegistry()

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			token := registry.Reserve()
			registry.Attach(token, func(frame string) error { return nil })
			registry.Route(token, "op")
			registry.Release(token)
		}()
	}

	wg.Wait()
}
