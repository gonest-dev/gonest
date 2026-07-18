// reservation.go implements the token -> reservation registry used by the
// graphql-sse protocol's "Single connection mode" (as opposed to "Distinct
// connections mode", which needs no reservation at all -- see
// ssedistinct.go). In Single connection mode the client performs a PUT to
// reserve a token, then a single GET (using that token) opens the one SSE
// connection that will carry every subscription event for that token; all
// subsequent POST (start operation) / DELETE (stop operation) requests
// reference the token instead of holding their own connection, and must be
// routed to the GET's SSE connection. This file owns only the token <->
// connection bookkeeping; the actual PUT/GET/POST/DELETE HTTP handlers and
// operation multiplexing are future tasks.
//
// Known v1 gap (documented, intentionally unresolved -- YAGNI): a token
// reserved via Reserve but never attached (client sent PUT but never
// followed up with GET) stays in the registry forever. There is no TTL or
// expiration sweep in this version.
package graphql

import (
	"sync"

	"github.com/google/uuid"
)

// reservation tracks the state of a single graphql-sse Single connection
// mode token: reserved (write is nil, no GET has attached yet) or attached
// (write is the frame-emitting function for the token's one SSE
// connection).
type reservation struct {
	write func(frame string) error
}

// ReservationRegistry is a thread-safe registry of graphql-sse Single
// connection mode tokens, mapping each token to the (possibly not-yet-
// attached) SSE connection that will carry its events.
type ReservationRegistry struct {
	mu    sync.Mutex
	byTok map[string]*reservation
}

// NewReservationRegistry constructs an empty ReservationRegistry.
func NewReservationRegistry() *ReservationRegistry {
	return &ReservationRegistry{
		byTok: make(map[string]*reservation),
	}
}

// Reserve generates a unique token and registers an empty reservation for
// it (no SSE connection attached yet). Callers hand this token back to the
// PUT response body per the graphql-sse Single connection mode handshake.
func (r *ReservationRegistry) Reserve() (token string) {
	token = uuid.NewString()

	r.mu.Lock()
	r.byTok[token] = &reservation{}
	r.mu.Unlock()

	return token
}

// Attach associates the write function of the token's single SSE
// connection (opened via GET) with an already-reserved token. It reports
// ok=false if the token was never reserved (or was already released).
func (r *ReservationRegistry) Attach(token string, write func(frame string) error) (ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	res, exists := r.byTok[token]
	if !exists {
		return false
	}

	res.write = write
	return true
}

// Route resolves the write function for the SSE connection attached to
// token, for use by subsequent POST/DELETE operation requests. It reports
// ok=false if the token is unknown or has not been attached yet (client
// sent PUT but the GET connection hasn't been established).
func (r *ReservationRegistry) Route(token, operationId string) (write func(frame string) error, ok bool) {
	_ = operationId // reserved for future per-operation routing/bookkeeping

	r.mu.Lock()
	defer r.mu.Unlock()

	res, exists := r.byTok[token]
	if !exists || res.write == nil {
		return nil, false
	}

	return res.write, true
}

// Release removes a token's reservation, called once the token's SSE
// connection (opened via GET) closes.
func (r *ReservationRegistry) Release(token string) {
	r.mu.Lock()
	delete(r.byTok, token)
	r.mu.Unlock()
}
