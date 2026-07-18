package emitter

import (
	"runtime"
	"testing"
	"time"
)

type subscribeTestEvent struct {
	Value string
}

func TestSubscribe_ReceivesFutureEmit(t *testing.T) {
	e := New()
	done := make(chan struct{})
	defer close(done)

	ch := Subscribe[subscribeTestEvent](e, done)

	e.Emit(subscribeTestEvent{Value: "hello"})

	select {
	case got := <-ch:
		if got.Value != "hello" {
			t.Fatalf("got %+v, want Value=hello", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscribed event")
	}
}

func TestSubscribe_MultipleEmits_AllReceivedInOrder(t *testing.T) {
	e := New()
	done := make(chan struct{})
	defer close(done)

	ch := Subscribe[subscribeTestEvent](e, done)

	e.Emit(subscribeTestEvent{Value: "one"})
	e.Emit(subscribeTestEvent{Value: "two"})
	e.Emit(subscribeTestEvent{Value: "three"})

	want := []string{"one", "two", "three"}
	for _, w := range want {
		select {
		case got := <-ch:
			if got.Value != w {
				t.Fatalf("got %q, want %q", got.Value, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q", w)
		}
	}
}

func TestSubscribe_ClosingDone_ClosesReturnedChannel(t *testing.T) {
	e := New()
	done := make(chan struct{})

	ch := Subscribe[subscribeTestEvent](e, done)

	close(done)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed, got a value instead")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel to close after done fired")
	}
}

func TestSubscribe_DifferentTypesDontCrossDeliver(t *testing.T) {
	type otherEvent struct{ N int }

	e := New()
	done := make(chan struct{})
	defer close(done)

	ch := Subscribe[subscribeTestEvent](e, done)

	e.Emit(otherEvent{N: 1})
	e.Emit(subscribeTestEvent{Value: "real"})

	select {
	case got := <-ch:
		if got.Value != "real" {
			t.Fatalf("got %+v, want Value=real (otherEvent must not cross-deliver)", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the correctly-typed event")
	}
}

// TestSubscribe_DoneClose_DoesNotLeakGoroutine is the regression test
// tasks.md's T8 explicitly asks for: closing done must let Subscribe's own
// internal goroutine exit, not leak for the process's lifetime.
func TestSubscribe_DoneClose_DoesNotLeakGoroutine(t *testing.T) {
	e := New()

	before := runtime.NumGoroutine()

	const n = 50
	dones := make([]chan struct{}, n)
	for i := range dones {
		dones[i] = make(chan struct{})
		Subscribe[subscribeTestEvent](e, dones[i])
	}

	for _, d := range dones {
		close(d)
	}

	// Give the goroutines a chance to observe done and return -- polling
	// rather than a fixed sleep, condition-based per this project's own
	// testing conventions.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 { // small slack for test runner noise
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("goroutine count did not return to baseline: before=%d, after=%d", before, runtime.NumGoroutine())
}
