package gonest

import "github.com/gonest-dev/gonest/internal/interceptor"

// Interceptor represents a single reusable before/after-Handler-execution
// unit with a continuation-passing (ctx, next) shape, wrapping a route's
// Handler for AOP-style pre/post-processing (timing, transformation,
// caching, etc.), mirroring Nest interceptors. It is attached via
// Controller.Interceptors (e.g. INSIGHT.md's
// `controller.Interceptors(TimingInterceptor)`). See
// internal/interceptor.Interceptor's doc comment for the full contract.
type Interceptor = interceptor.Interceptor

// NewInterceptor creates an Interceptor and runs fn on it immediately
// (unlike Provider/Module/Controller/Pipe, which defer fn until
// bootstrap). This feature deliberately has no MustInject support (AD-008
// in STATE.md: pipeline-stage types don't support MustInject in v1, same
// reasoning as NewGuard/NewMiddleware), so there is no bootstrap stage
// left to usefully defer fn to. Like NewGuard/NewMiddleware/
// NewHttpException, New here is not generic, so Go allows aliasing the
// plain func directly via var -- no wrapper function is needed (root
// package is the only public door since Go blocks external import of
// internal/*, per AD-004 in STATE.md). See internal/interceptor.New's doc
// comment for the full contract.
var NewInterceptor = interceptor.New
