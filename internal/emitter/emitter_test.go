package emitter

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type fooEvent struct{ ID int }
type barEvent struct{ ID int }

func TestEmit_DispatchesToRegisteredListener_Async(t *testing.T) {
	e := New()

	done := make(chan fooEvent, 1)
	handler := func(ctx context.Context, ev fooEvent) { done <- ev }
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(handler))

	e.Emit(fooEvent{ID: 42})

	select {
	case got := <-done:
		if got.ID != 42 {
			t.Fatalf("listener received ID=%d, want 42", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not run within 1s")
	}
}

func TestEmit_ReturnsImmediately_DoesNotBlockOnListener(t *testing.T) {
	e := New()
	release := make(chan struct{})

	handler := func(ctx context.Context, ev fooEvent) { <-release }
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(handler))

	emitDone := make(chan struct{})
	go func() {
		e.Emit(fooEvent{})
		close(emitDone)
	}()

	select {
	case <-emitDone:
	case <-time.After(time.Second):
		t.Fatal("Emit blocked on a listener that never released, want fire-and-forget")
	}
	close(release)
}

func TestEmit_ListenerPanic_NeverPropagatesToCaller(t *testing.T) {
	e := New()
	ran := make(chan struct{})

	handler := func(ctx context.Context, ev fooEvent) {
		defer close(ran)
		panic("boom")
	}
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(handler))

	e.Emit(fooEvent{}) // must not panic here, even though the listener does

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("listener did not run")
	}
}

// TestEmit_ListenerReturnsError_NeverPropagatesToCaller proves a listener's
// non-nil error return gets the same swallow-and-continue treatment as a
// recovered panic (see Emit's own doc comment) -- the error-returning
// handler shape On/MustOn register (listener.go), exercised here directly
// via e.on to keep this test scoped to Emit's own dispatch logic.
func TestEmit_ListenerReturnsError_NeverPropagatesToCaller(t *testing.T) {
	e := New()
	ran := make(chan struct{})

	handler := func(ctx context.Context, ev fooEvent) error {
		defer close(ran)
		return errors.New("boom")
	}
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(handler))

	e.Emit(fooEvent{}) // must not surface the error here

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("listener did not run")
	}
}

func TestEmit_OnlyMatchingExactType_OtherListenersNeverRun(t *testing.T) {
	e := New()
	var mu sync.Mutex
	var barRan bool

	barHandler := func(ctx context.Context, ev barEvent) {
		mu.Lock()
		barRan = true
		mu.Unlock()
	}
	e.on(reflect.TypeOf(barEvent{}), reflect.ValueOf(barHandler))

	fooDone := make(chan struct{})
	fooHandler := func(ctx context.Context, ev fooEvent) { close(fooDone) }
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(fooHandler))

	e.Emit(fooEvent{})

	select {
	case <-fooDone:
	case <-time.After(time.Second):
		t.Fatal("fooEvent listener did not run")
	}

	mu.Lock()
	got := barRan
	mu.Unlock()
	if got {
		t.Fatal("barEvent listener ran for a fooEvent Emit, want isolation by exact type")
	}
}

func TestEmit_MultipleListenersForSameType_AllRun(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	wg.Add(2)

	h1 := func(ctx context.Context, ev fooEvent) { wg.Done() }
	h2 := func(ctx context.Context, ev fooEvent) { wg.Done() }
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(h1))
	e.on(reflect.TypeOf(fooEvent{}), reflect.ValueOf(h2))

	e.Emit(fooEvent{})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("not all listeners ran within 1s")
	}
}

func TestEmit_NoListenersRegistered_DoesNotPanic(t *testing.T) {
	e := New()
	e.Emit(fooEvent{})
}
