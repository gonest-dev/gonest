// Package emitter implements the declarative Emitter/Listener API: a
// typed-event (struct, not string) pub/sub mechanism, asynchronous
// fire-and-forget emission, equivalent to @nestjs/event-emitter. See
// ROADMAP.md's Milestone 9.
package emitter

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"gonest.dev/gonest/internal/logger"
)

// Emitter holds every registered listener (via MustOn), keyed by the exact
// event type it was registered for. It is a FRAMEWORK singleton -- never a
// user-registered Provider -- see internal/inject.RegisterGlobalSingleton,
// called once per bootstrap by internal/app, so MustInject[*Emitter]
// resolves from ANY module without explicit registration anywhere.
type Emitter struct {
	mu        sync.Mutex
	listeners map[reflect.Type][]reflect.Value

	// subscribers holds every dynamic channel registered via Subscribe[T]
	// (graphql-support feature, Milestone 17), keyed by the exact event
	// type it was registered for -- same keying as listeners, but a
	// fundamentally different lifecycle: a subscriber is dynamic (lives
	// only as long as the caller's own done channel stays open, e.g. one
	// GraphQL Subscription connection), while a listener registered via
	// MustOn is static (lives for the whole app). Kept as a SEPARATE map
	// rather than folding into listeners so Emit's hot path never needs to
	// distinguish "is this entry a MustOn handler or a Subscribe channel"
	// per event.
	subscribers map[reflect.Type][]chan any
}

// New builds an empty Emitter. Called once per bootstrap by internal/app
// (NewApp/MustNewTestApp), never directly by user code.
func New() *Emitter {
	return &Emitter{
		listeners:   make(map[reflect.Type][]reflect.Value),
		subscribers: make(map[reflect.Type][]chan any),
	}
}

// on registers handler (a func(context.Context, T) value, T == t) for t.
// Unexported -- callers register via the free function MustOn, which
// derives t from its own type parameter and reaches the CURRENT bootstrap's
// Emitter singleton via internal/inject's global-singleton registry (see
// MustOn's own doc comment for why this isn't a method).
func (e *Emitter) on(t reflect.Type, handler reflect.Value) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[t] = append(e.listeners[t], handler)
}

// handlersFor returns a copy of the handlers registered for t, in
// registration order. Read-only: mutating the returned slice does not
// affect this Emitter's internal state.
func (e *Emitter) handlersFor(t reflect.Type) []reflect.Value {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]reflect.Value(nil), e.listeners[t]...)
}

// Emit dispatches event to every listener registered (via MustOn) for
// event's EXACT concrete type, one goroutine per listener, and returns
// immediately -- fire-and-forget, never blocks the caller (spec.md's own
// explicit requirement: "não bloqueia quem chamou Emit"). A listener that
// panics is recovered INSIDE its own goroutine and never propagates to the
// caller of Emit or to any other listener's goroutine -- the recovered
// value is logged via internal/logger.Error (Nest's own equivalent
// behavior: an event handler failing surfaces in the log, not silently).
func (e *Emitter) Emit(event any) {
	t := reflect.TypeOf(event)
	eventValue := reflect.ValueOf(event)

	for _, h := range e.handlersFor(t) {
		h := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error(fmt.Sprintf("listener for event %s panicked: %v", t, r))
				}
			}()
			h.Call([]reflect.Value{reflect.ValueOf(context.Background()), eventValue})
		}()
	}

	// graphql-support feature: forward to every Subscribe[T] channel
	// registered for t, non-blocking (select+default) -- same "never
	// blocks the caller" contract Emit already documents above. A full or
	// abandoned-but-not-yet-removed channel simply drops the event rather
	// than stalling Emit.
	for _, ch := range e.subscribersFor(t) {
		select {
		case ch <- event:
		default:
		}
	}
}
