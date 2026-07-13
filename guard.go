package gonest

import "github.com/gonest-dev/gonest/internal/guard"

// Guard represents a single authorization-check unit: it holds the
// ctx-in/bool-out handler function registered via Handler. Unlike
// Middleware, a Guard doesn't decorate/wrap a continuation -- it gates:
// true means continue, false (or a panic'd Exception) means stop. It is
// attached via Controller.Guards (e.g. INSIGHT.md's
// `controller.Guards(AuthGuard)`). See internal/guard.Guard's doc comment
// for the full contract.
type Guard = guard.Guard

// NewGuard creates a Guard and runs fn on it immediately (unlike
// Provider/Module/Controller/Pipe, which defer fn until bootstrap). This
// feature deliberately has no MustInject support (see this feature's own
// design.md's Tech Decisions: a *Guard can be attached to multiple
// controllers across different modules, with no clean single "owner" to
// resolve MustInject against), so there is no bootstrap stage left to
// usefully defer fn to. Like NewMiddleware/NewHttpException, New here is
// not generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed (root package is the only public door since
// Go blocks external import of internal/*, per AD-004 in STATE.md). See
// internal/guard.New's doc comment for the full contract.
var NewGuard = guard.New
