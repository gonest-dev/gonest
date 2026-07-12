package gonest

import (
	"context"
	"time"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolve"
	"github.com/gonest-dev/gonest/internal/resolver"
)

// bootstrapTimeout bounds Stage 3 (Parallel Resolution): every Provider's
// Constructor -- including ones that don't accept a context.Context
// themselves -- runs within this deadline collectively, since it is the ctx
// passed to errgroup.WithContext. Not yet configurable via AppOptions
// (that arrives with the "App Bootstrap & Listen" feature); a fixed
// generous default keeps NewApp usable today without hanging forever on a
// Constructor that never returns.
const bootstrapTimeout = 30 * time.Second

// App is the minimal handle returned once NewApp has finished bootstrapping
// the whole dependency graph. It is intentionally opaque at this stage --
// Listen/AppOptions/UseLogger etc. belong to the future "App Bootstrap &
// Listen" feature. It exists now only so NewApp/MustNewApp have a return
// type to hand back.
type App struct {
	root *Module
}

// NewApp bootstraps root through all 3 DI stages, in order:
//
//  1. Structural Assembly (root.Assemble): walks Imports, wires
//     OwnerModule on every registered provider/controller, validates
//     Exports.
//  2. Builder Execution: runs Declare on every provider and controller
//     across the assembled module tree, so MustResolve calls inside their
//     builder fn run against a fully-known module tree and record pending
//     edges.
//  3. Cycle detection, then Parallel Resolution: resolves every registered
//     Provider concurrently (respecting dependency edges recorded in Stage
//     2), copying each resolved instance in place into every placeholder
//     MustResolve returned for it.
//
// It returns once the whole graph is resolved, or an error from Stage 1
// (structural/export validation), cycle detection, or Stage 3 (a
// Constructor's returned error or recovered panic).
//
// NewApp calls resolve.Reset() at the very start, before Stage 2, to clear
// internal/resolve's process-global pending-edge bookkeeping left over from
// any previous NewApp call -- see resolve.Reset's doc comment for the "one
// bootstrap at a time per process" contract this establishes. Calling
// NewApp concurrently from multiple goroutines in the same process is not
// supported; NewApp is meant to run once, synchronously, at process
// startup.
func NewApp(root *Module) (*App, error) {
	resolve.Reset()

	modules, err := root.Assemble()
	if err != nil {
		return nil, err
	}

	declareAll(modules)

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := resolver.Resolve(ctx, modules); err != nil {
		return nil, err
	}

	return &App{root: root}, nil
}

// MustNewApp calls NewApp and panics if it returns an error. Convenience
// for callers (typically main) that treat bootstrap failure as fatal.
func MustNewApp(root *Module) *App {
	app, err := NewApp(root)
	if err != nil {
		panic(err)
	}
	return app
}

// declarable is satisfied by both *provider.Provider and
// *controller.Controller (both expose Declare). Declared locally so this
// file can invoke Declare on module.ProviderRef/module.ControllerRef values
// without either of those interfaces needing to expose it -- Declare is a
// bootstrap-orchestration concern, not part of what Module needs from a
// registered participant.
type declarable interface {
	Declare()
}

// declareAll runs Declare (Stage 2 builder execution) on every provider and
// controller registered across modules, exactly once each. Order does not
// matter: MustResolve calls made during Declare only ever look up an
// already-assembled module tree (Stage 1 has already fully run by the time
// this is called from NewApp), never another provider/controller's
// Declare-time state.
func declareAll(modules []*module.Module) {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			if d, ok := p.(declarable); ok {
				d.Declare()
			}
		}
		for _, c := range m.OwnControllers() {
			if d, ok := c.(declarable); ok {
				d.Declare()
			}
		}
	}
}
