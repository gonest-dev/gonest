package shared

import "github.com/gonest-dev/gonest"

// AuthGuard demonstrates a Guard shared across multiple controllers/
// modules (attached to user/post/comment's write routes below) -- a
// trivial static-token check, not real auth, purely to dogfood the
// union-scoped Guard mechanism (AD-015's 3-phase bootstrap).
var AuthGuard = gonest.NewGuard(func(guard *gonest.Guard) {
	guard.Handler(func(ctx *gonest.RestContext) bool {
		return ctx.Header("Authorization") == "Bearer demo-token"
	})
})
