package resolver

import (
	"context"
	"sync"
)

// RequestScope is the isolated caching mechanism for scope.Request
// providers: it lets the SAME provider resolve to the SAME instance within
// one "request context", and to a DIFFERENT instance across different
// request contexts.
//
// STUB: wiring real com HTTP Pipeline é feature futura (Milestone 3, "App
// Bootstrap & Listen"/Request Pipeline). This type has no dependency on
// net/http or any real request lifecycle -- it is a standalone,
// context.Context-keyed cache, deliberately NOT wired into
// internal/resolver's Stage 3 (Resolve, in stage3.go) or into
// internal/resolve's pending-edge bookkeeping. A future Pipeline feature is
// expected to (a) call WithRequestID once per inbound HTTP request to tag
// that request's ctx, and (b) consult a RequestScope's Get/Set from within
// MustResolve-style resolution when a target provider's scope.Scope is
// scope.Request -- neither of those integration points exists yet, and
// building them now would require inventing a "resolve on demand mid
// request" flow that is out of this task's scope (see spec.md's P3 story
// and design.md).
//
// Callers identify a cache entry with an opaque key (analogous to
// module.ProviderRef identity in the real resolver, but RequestScope itself
// stays ignorant of internal/module/internal/provider types -- any
// comparable value works, keeping this mechanism reusable regardless of
// what a future Pipeline chooses as its provider identity).
type RequestScope interface {
	// Get returns the instance cached for key within ctx's request context,
	// and whether one was found. ok is false both when ctx carries no
	// request ID (see WithRequestID) and when ctx has a request ID but no
	// value was ever Set for key within it.
	Get(ctx context.Context, key any) (instance any, ok bool)

	// Set records instance for key within ctx's request context. Calling
	// Set with a ctx that carries no request ID is a no-op (there is no
	// request bucket to store into) -- callers are expected to have tagged
	// ctx via WithRequestID first.
	Set(ctx context.Context, key any, instance any)
}

// requestIDKey is the unexported context.Context key type used by
// WithRequestID/requestIDFrom. Unexported (and a distinct named type, not a
// bare string) so no other package's context value can collide with it --
// the same defensive pattern context.Context's own documentation
// recommends for context keys.
type requestIDKey struct{}

// WithRequestID returns a copy of parent tagged with id as its request
// context identifier. This is the mocked stand-in for "a real HTTP
// Pipeline assigns a unique ID to every inbound request and threads it
// through ctx" -- see RequestScope's doc comment for why the real wiring is
// deferred. id only needs to be comparable (used as a map key internally);
// any value that uniquely identifies one in-flight request works.
func WithRequestID(parent context.Context, id any) context.Context {
	return context.WithValue(parent, requestIDKey{}, id)
}

// requestIDFrom extracts the request ID tagged onto ctx via WithRequestID,
// and whether one was present.
func requestIDFrom(ctx context.Context) (id any, ok bool) {
	id = ctx.Value(requestIDKey{})
	if id == nil {
		return nil, false
	}
	return id, true
}

// requestScope is RequestScope's only implementation: a mutex-guarded,
// two-level map (request ID -> key -> instance). Mutex-guarded map is the
// same defensive style internal/resolve.pendingEdges (see
// internal/resolve/resolve.go) and internal/resolver's other package-level
// state use elsewhere in this codebase, kept here for consistency even
// though this is an independent, non-shared mechanism (each NewRequestScope
// call returns its own instance -- unlike internal/resolve's
// process-global pendingEdges, there is no package-level var here, so no
// Reset()-style bootstrap contract is needed).
type requestScope struct {
	mu    sync.Mutex
	byReq map[any]map[any]any
}

// NewRequestScope returns a new, empty RequestScope ready for use. Safe for
// concurrent use by multiple goroutines.
func NewRequestScope() RequestScope {
	return &requestScope{byReq: make(map[any]map[any]any)}
}

func (r *requestScope) Get(ctx context.Context, key any) (any, bool) {
	id, ok := requestIDFrom(ctx)
	if !ok {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.byReq[id]
	if !ok {
		return nil, false
	}

	instance, ok := bucket[key]
	return instance, ok
}

func (r *requestScope) Set(ctx context.Context, key any, instance any) {
	id, ok := requestIDFrom(ctx)
	if !ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.byReq[id]
	if !ok {
		bucket = make(map[any]any)
		r.byReq[id] = bucket
	}
	bucket[key] = instance
}
