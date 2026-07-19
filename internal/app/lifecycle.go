package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"gonest.dev/gonest/internal/module"
)

// moduleInitRunner is the minimal interface a registered provider must
// satisfy for runModuleInitPhase to invoke its OnModuleInit hook. Only
// *provider.Provider implements it in practice (see
// internal/provider/lifecycle.go's RunModuleInit), but this package
// deliberately type-asserts against a tiny local interface rather than
// importing internal/provider's concrete type -- same cross-package
// decoupling rationale declareProviders already follows via the
// package-local `declarable` interface.
type moduleInitRunner interface {
	RunModuleInit(ctx context.Context) error
}

// applicationBootstrapRunner is moduleInitRunner's counterpart for the
// OnApplicationBootstrap phase. Kept as a separate interface (rather than
// folding both methods into one) so a provider type could in principle
// implement just one phase's hook without being forced to also implement
// the other -- mirrors how Provider itself keeps RunModuleInit and
// RunApplicationBootstrap as independent methods.
type applicationBootstrapRunner interface {
	RunApplicationBootstrap(ctx context.Context) error
}

// runModuleInitPhase runs every registered provider's OnModuleInit hook
// (Nest's per-module init phase) across modules, LEAF-FIRST: modules
// arrives root-first (module.Assemble's BFS appends root to its returned
// order before any imported module -- see assemble.go), but a leaf module's
// providers are the ones importing modules are most likely to depend on, so
// they must finish initializing first. Achieved by iterating a REVERSED
// COPY of modules rather than mutating the caller's slice in place.
//
// Within a single module, providers run in OwnProviders' declaration order
// (same order declareProviders in app.go already relies on).
//
// Fails fast: the first hook to return a non-nil error stops the ENTIRE
// phase immediately -- no further providers run, whether they belong to the
// same module or a later one in the reversed walk. This mirrors Nest's own
// behavior (a failing lifecycle hook aborts application startup) and keeps
// the semantics simple: callers get a single error indicating the phase did
// not fully complete, rather than a partial-completion result they would
// have to reason about.
//
// A provider whose concrete type does not implement moduleInitRunner (in
// practice, this never happens for a real *provider.Provider, which always
// implements it) is silently skipped -- not an error, just nothing to run.
func runModuleInitPhase(ctx context.Context, modules []*module.Module) error {
	reversed := make([]*module.Module, len(modules))
	for i, m := range modules {
		reversed[len(modules)-1-i] = m
	}

	for _, m := range reversed {
		for _, p := range m.OwnProviders() {
			runner, ok := p.(moduleInitRunner)
			if !ok {
				continue
			}
			if err := runner.RunModuleInit(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// runApplicationBootstrapPhase runs every registered provider's
// OnApplicationBootstrap hook (Nest's whole-app bootstrap phase, which runs
// after every provider's OnModuleInit has completed) across modules. Same
// leaf-first traversal and fail-fast semantics as runModuleInitPhase -- see
// that function's doc comment for the full reasoning. The two phases are
// sequenced (Init phase fully, then Bootstrap phase, only if Init
// succeeded) by a later task's caller in NewApp; this function only defines
// the standalone Bootstrap-phase walk.
func runApplicationBootstrapPhase(ctx context.Context, modules []*module.Module) error {
	reversed := make([]*module.Module, len(modules))
	for i, m := range modules {
		reversed[len(modules)-1-i] = m
	}

	for _, m := range reversed {
		for _, p := range m.OwnProviders() {
			runner, ok := p.(applicationBootstrapRunner)
			if !ok {
				continue
			}
			if err := runner.RunApplicationBootstrap(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// moduleDestroyRunner is runModuleDestroyPhase's counterpart to
// moduleInitRunner -- same "tiny local interface, type-assert
// OwnProviders() entries against it" pattern, targeting
// *provider.Provider's RunModuleDestroy (see
// internal/provider/lifecycle.go).
type moduleDestroyRunner interface {
	RunModuleDestroy(ctx context.Context) error
}

// beforeApplicationShutdownRunner is runBeforeApplicationShutdownPhase's
// counterpart to moduleInitRunner, targeting *provider.Provider's
// RunBeforeApplicationShutdown.
type beforeApplicationShutdownRunner interface {
	RunBeforeApplicationShutdown(ctx context.Context, signal string) error
}

// applicationShutdownRunner is runApplicationShutdownPhase's counterpart to
// moduleInitRunner, targeting *provider.Provider's RunApplicationShutdown --
// the extra signal parameter is threaded through identically to
// RunBeforeApplicationShutdown's.
type applicationShutdownRunner interface {
	RunApplicationShutdown(ctx context.Context, signal string) error
}

// runModuleDestroyPhase runs every registered provider's OnModuleDestroy
// hook (Nest's per-module teardown phase, the shutdown mirror of
// runModuleInitPhase) across modules, ROOT-FIRST -- the OPPOSITE traversal
// order of runModuleInitPhase/runApplicationBootstrapPhase. modules already
// arrives root-first from root.Assemble() (see assemble.go), so unlike
// those two init-phase functions this one walks modules AS-IS, with no
// reversal: teardown runs in the reverse order of startup (init went
// leaf-first, so destroy goes root-first), matching Nest's own shutdown
// convention where the outermost/most-dependent modules release their
// resources before the leaf modules they depend on.
//
// Within a single module, providers run in OwnProviders' declaration order,
// same as every other phase function in this file.
//
// Fails fast, same contract as runModuleInitPhase: the first hook to return
// a non-nil error stops the entire phase immediately. A provider whose
// concrete type does not implement moduleDestroyRunner is silently skipped.
func runModuleDestroyPhase(ctx context.Context, modules []*module.Module) error {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			runner, ok := p.(moduleDestroyRunner)
			if !ok {
				continue
			}
			if err := runner.RunModuleDestroy(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// runBeforeApplicationShutdownPhase runs every registered provider's
// BeforeApplicationShutdown hook (Nest's "still fully live" pre-teardown
// phase) across modules, root-first -- see runModuleDestroyPhase's doc
// comment for the full traversal-order rationale, which applies here
// identically. signal is forwarded to every hook invocation unchanged
// (RunBeforeApplicationShutdown's own contract -- see
// internal/provider/lifecycle.go).
//
// Fails fast, same contract as runModuleDestroyPhase. A provider whose
// concrete type does not implement beforeApplicationShutdownRunner is
// silently skipped.
func runBeforeApplicationShutdownPhase(ctx context.Context, modules []*module.Module, signal string) error {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			runner, ok := p.(beforeApplicationShutdownRunner)
			if !ok {
				continue
			}
			if err := runner.RunBeforeApplicationShutdown(ctx, signal); err != nil {
				return err
			}
		}
	}

	return nil
}

// runApplicationShutdownPhase runs every registered provider's
// OnApplicationShutdown hook (Nest's final teardown phase, the last hook to
// fire before the process exits) across modules, root-first -- see
// runModuleDestroyPhase's doc comment for the full traversal-order
// rationale, which applies here identically. signal is forwarded to every
// hook invocation unchanged, same as runBeforeApplicationShutdownPhase.
//
// Fails fast, same contract as the other two destroy-phase functions. A
// provider whose concrete type does not implement
// applicationShutdownRunner is silently skipped.
func runApplicationShutdownPhase(ctx context.Context, modules []*module.Module, signal string) error {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			runner, ok := p.(applicationShutdownRunner)
			if !ok {
				continue
			}
			if err := runner.RunApplicationShutdown(ctx, signal); err != nil {
				return err
			}
		}
	}

	return nil
}

// signalName maps an os.Signal received by the shutdown goroutine below to
// the string handed to the BeforeApplicationShutdown/OnApplicationShutdown
// hooks' trailing `signal string` parameter. A manual mapping (rather than
// sig.String(), which on some platforms/signals renders as lowercase
// human-readable text like "interrupt") is used here so hook authors get
// the conventional, platform-stable POSIX names ("SIGINT"/"SIGTERM") Nest's
// own equivalent hooks use -- these are the only 2 signals EnableShutdownHooks
// listens for, so the mapping is a small, exhaustive switch rather than a
// general os.Signal-to-string helper.
func signalName(sig os.Signal) string {
	switch sig {
	case os.Interrupt:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return sig.String()
	}
}

// EnableShutdownHooks arms this App to run its 3 destroy-phase lifecycle
// hooks (OnModuleDestroy -> BeforeApplicationShutdown -> OnApplicationShutdown)
// when the process receives os.Interrupt (SIGINT) or syscall.SIGTERM --
// mirroring Nest's own app.enableShutdownHooks(), which is opt-in (Nest
// does not listen for OS signals by default, since a caller embedding Nest
// inside a larger process may want to manage its own signal handling).
// Without calling this, an OS signal has NO special handling from gonest at
// all -- the process terminates via Go's default signal behavior, and
// a.shutdown's 3 hook phases never run (though a.adapter.Shutdown is still
// always reachable directly via Close, since HTTP draining is not gated by
// this flag -- see shutdown's own doc comment).
//
// It returns the same *App it was called on so callers can chain it
// directly onto NewApp/MustNewApp, e.g.
// `app := gonest.MustNewApp[...](root); app.EnableShutdownHooks()`.
//
// Starts exactly one background goroutine (per call) that blocks on
// signal.Notify's channel until either signal arrives, then calls
// a.shutdown(context.Background(), signalName) exactly once -- a.shutdown's
// own sync.Once guard (see below) means calling EnableShutdownHooks more
// than once, or a signal arriving after Close was already called manually,
// is harmless: only the FIRST caller of a.shutdown (whichever it is) ever
// actually runs the teardown sequence.
func (a *App) EnableShutdownHooks() *App {
	a.shutdownHooksEnabled = true

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		a.shutdown(context.Background(), signalName(sig))
	}()

	return a
}

// Close gracefully shuts this App down: drains the underlying HTTP adapter
// and, if EnableShutdownHooks was called, runs the 3 destroy-phase lifecycle
// hooks -- the programmatic counterpart to an OS signal arriving, for
// callers (tests, embedders managing their own signal handling) that want
// to trigger the exact same shutdown sequence without actually sending a
// signal. signal is always "" for a Close-triggered shutdown (there is no
// real OS signal to report) -- BeforeApplicationShutdown/
// OnApplicationShutdown hooks that branch on signal simply see an empty
// string in this path, same as Nest's own app.close() passing no signal.
//
// Calling Close (or receiving a signal, or calling both) more than once is
// safe -- see shutdown's own doc comment for the sync.Once guarantee -- and
// every call after the first returns the SAME error the first invocation
// computed.
func (a *App) Close(ctx context.Context) error {
	return a.shutdown(ctx, "")
}

// shutdown is the single entry point BOTH Close and EnableShutdownHooks'
// signal-handling goroutine fund into -- guaranteeing the actual teardown
// sequence (runShutdownSequence) executes AT MOST ONCE no matter how many
// times shutdown itself is called (directly, via Close, or via a caught OS
// signal), via a.shutdownOnce. Every call -- not just the first -- returns
// the FINAL a.shutdownErr the first (real) run computed; callers racing
// Close against a live signal, or calling Close twice, all observe the same
// outcome.
//
// Structured as sync.Once.Do wrapping a SEPARATE runShutdownSequence method
// (rather than inlining the sequence directly in the Do closure) so the
// closure itself stays trivial -- the actual multi-step sequencing logic
// lives in one un-nested, independently readable/testable method.
func (a *App) shutdown(ctx context.Context, signal string) error {
	a.shutdownOnce.Do(func() {
		a.shutdownErr = a.runShutdownSequence(ctx, signal)
		// a.shutdownDone is closed HERE, AFTER a.shutdownErr has been
		// assigned -- not via a defer inside runShutdownSequence itself
		// (which would close it before returning control to this closure,
		// i.e. before the assignment above ran). A caller elsewhere
		// blocked on <-a.shutdownDone (e.g. Listen) must never observe the
		// channel closed before a.shutdownErr holds its final value --
		// closing it here establishes that happens-before relationship
		// explicitly, rather than relying on defer-ordering inside a
		// different function.
		close(a.shutdownDone)
	})
	return a.shutdownErr
}

// runShutdownSequence is shutdown's actual teardown sequence, run exactly
// once via a.shutdownOnce.Do:
//
//  1. a.adapter.Shutdown(ctx) ALWAYS runs first, regardless of
//     a.shutdownHooksEnabled -- draining the underlying HTTP engine is not a
//     lifecycle-hooks concern, it is the minimum any Close/shutdown caller
//     expects to happen (mirrors Nest's own app.close() always tearing down
//     the underlying HTTP server, whether or not enableShutdownHooks was
//     ever called). If this returns a non-nil error, runShutdownSequence
//     returns it immediately WITHOUT running any of the 3 hook phases below
//     -- an HTTP engine that failed to drain cleanly means the application
//     is in an unknown state, not one lifecycle hooks should run against.
//  2. If a.shutdownHooksEnabled is true, the 3 destroy phases run in strict
//     sequence -- runModuleDestroyPhase, then runBeforeApplicationShutdownPhase,
//     then runApplicationShutdownPhase(signal) -- each against a.modules. An
//     error from ANY one of the 3 aborts the REMAINING phases too (returned
//     immediately, matching every phase function's own individual fail-fast
//     contract, now composed across phases as well). If shutdownHooksEnabled
//     is false, none of the 3 phases run at all.
//  3. a.shutdownDone is closed by shutdown (the caller of this method), NOT
//     by this method itself, AFTER a.shutdownErr has been assigned this
//     method's return value -- see shutdown's own doc comment for why: a
//     caller blocked on a.shutdownDone (e.g. App.Listen, wired by a later
//     task) must never be able to observe the channel closed before
//     a.shutdownErr holds its final value.
func (a *App) runShutdownSequence(ctx context.Context, signal string) error {
	if err := a.adapter.Shutdown(ctx); err != nil {
		return err
	}

	if !a.shutdownHooksEnabled {
		return nil
	}

	if err := runModuleDestroyPhase(ctx, a.modules); err != nil {
		return err
	}
	if err := runBeforeApplicationShutdownPhase(ctx, a.modules, signal); err != nil {
		return err
	}
	if err := runApplicationShutdownPhase(ctx, a.modules, signal); err != nil {
		return err
	}

	return nil
}
