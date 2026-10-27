package emitter

import "reflect"

// subscribeBufferSize is the buffer depth of both the internal forwarding
// channel and the caller-facing channel Subscribe returns -- large enough
// to absorb a short burst without Emit's own non-blocking send (see
// Emit's own doc comment) silently dropping events under normal load,
// small enough to bound memory for a connection that never reads.
const subscribeBufferSize = 16

// subscribersFor returns a copy of the raw forwarding channels registered
// for t. Read-only: mutating the returned slice does not affect this
// Emitter's internal state (same defensive-copy pattern as handlersFor).
func (e *Emitter) subscribersFor(t reflect.Type) []chan any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]chan any(nil), e.subscribers[t]...)
}

// removeSubscriber removes raw from e.subscribers[t] the first time it's
// found -- called by Subscribe's own internal goroutine once done fires.
// raw is deliberately never closed (see Subscribe's own doc comment on
// why) -- removing it from the map is enough to stop it receiving any
// FUTURE Emit call's non-blocking send; a send already in flight when
// removal happens is harmless (it either succeeds into a channel nothing
// reads anymore, or is dropped by Emit's own select+default).
func (e *Emitter) removeSubscriber(t reflect.Type, raw chan any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	subs := e.subscribers[t]
	for i, c := range subs {
		if c == raw {
			e.subscribers[t] = append(subs[:i:i], subs[i+1:]...)
			return
		}
	}
}

// Subscribe registers a dynamic, per-connection channel that receives
// every future Emit(T{...}) call until done closes.
func (e *Emitter) Subscribe[T any](done <-chan struct{}) <-chan T {
	t := reflect.TypeFor[T]()
	raw := make(chan any, subscribeBufferSize)
	out := make(chan T, subscribeBufferSize)

	e.mu.Lock()
	e.subscribers[t] = append(e.subscribers[t], raw)
	e.mu.Unlock()

	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				e.removeSubscriber(t, raw)
				return
			case v := <-raw:
				select {
				case out <- v.(T):
				case <-done:
					e.removeSubscriber(t, raw)
					return
				}
			}
		}
	}()

	return out
}
