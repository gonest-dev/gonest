package emitter

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gonest.dev/gonest/internal/inject"
)

type quxEvent struct{ Value int }

// withBootstrapEmitter registers e as the current bootstrap's Emitter
// singleton for the duration of the test -- the same registration
// internal/app performs once per real NewApp/MustNewTestApp call, but
// Listener.Declare only needs the registry to be populated, not a full
// bootstrap.
func withBootstrapEmitter(t *testing.T, e *Emitter) {
	t.Helper()
	inject.RegisterGlobalSingleton(emitterType, reflect.ValueOf(e))
	t.Cleanup(inject.Reset)
}

func TestListener_On_HandlerErrorLogged_NeverPropagates(t *testing.T) {
	e := New()
	withBootstrapEmitter(t, e)

	ran := make(chan struct{})
	l := NewListener(func(l *Listener[quxEvent]) {
		l.On(func(ctx context.Context, event quxEvent) error {
			defer close(ran)
			return errors.New("boom")
		})
	})
	l.Declare()

	e.Emit(quxEvent{Value: 1}) // must not panic/return an error here

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("listener did not run")
	}
}

func TestListener_MustOn_WrapsHandlerWithNilError(t *testing.T) {
	e := New()
	withBootstrapEmitter(t, e)

	got := make(chan int, 1)
	l := NewListener(func(l *Listener[quxEvent]) {
		l.MustOn(func(ctx context.Context, event quxEvent) {
			got <- event.Value
		})
	})
	l.Declare()

	e.Emit(quxEvent{Value: 42})

	select {
	case v := <-got:
		if v != 42 {
			t.Fatalf("handler received Value=%d, want 42", v)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not run")
	}
}

func TestListener_Declare_NoOpWhenOnNeverCalled(t *testing.T) {
	e := New()
	withBootstrapEmitter(t, e)

	l := NewListener(func(l *Listener[quxEvent]) {
		// never calls On/MustOn
	})
	l.Declare() // must not panic

	if got := e.handlersFor(reflect.TypeFor[quxEvent]()); len(got) != 0 {
		t.Fatalf("expected no handler registered for quxEvent, got %d", len(got))
	}
}

func TestListener_Declare_Idempotent(t *testing.T) {
	e := New()
	withBootstrapEmitter(t, e)

	calls := 0
	l := NewListener(func(l *Listener[quxEvent]) {
		calls++
		l.MustOn(func(ctx context.Context, event quxEvent) {})
	})
	l.Declare()
	l.Declare()
	l.Declare()

	if calls != 1 {
		t.Fatalf("builder fn ran %d times, want 1", calls)
	}
}
