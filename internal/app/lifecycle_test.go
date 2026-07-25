package app

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
)

// resolveProvider drives a *provider.Provider through exactly the steps
// runModuleInitPhase/runApplicationBootstrapPhase's real callers (NewApp's
// full bootstrap) would have already performed by the time either phase
// runs: Declare (so Constructor/OnModuleInit/OnApplicationBootstrap
// registrations actually happen -- see Provider.Declare's doc comment),
// then invoking the stored Constructor directly and feeding its result into
// SetResolvedValue (simulating internal/resolver's Stage 3, without running
// an entire resolver.Resolve). Only Singleton-scoped providers with a set
// ResolvedValue have their hooks actually invoked by runHook (see
// internal/provider/lifecycle.go), so skipping either step would make the
// hook silently no-op instead of exercising it.
func resolveProvider(t *testing.T, p *provider.Provider) {
	t.Helper()
	p.Declare()

	fn := p.ConstructorFunc()
	var args []reflect.Value
	if fn.Type().NumIn() == 1 {
		args = []reflect.Value{reflect.ValueOf(context.Background())}
	}
	out := fn.Call(args)
	p.SetResolvedValue(out[0])
}

type liRootType struct{}
type liLeafType struct{}

// TestRunModuleInitPhase_LeafFirstOrder proves runModuleInitPhase visits
// modules in the REVERSE of Assemble's root-first order: a provider owned
// by an imported (leaf) module must run its OnModuleInit hook before a
// provider owned by the root module that imports it, since importing
// modules are the ones more likely to depend on what a leaf already set up.
func TestRunModuleInitPhase_LeafFirstOrder(t *testing.T) {
	var log []string

	leafProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *liLeafType { return &liLeafType{} })
		p.OnModuleInit(func(*liLeafType) {
			log = append(log, "leaf")
		})
	})
	rootProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *liRootType { return &liRootType{} })
		p.OnModuleInit(func(*liRootType) {
			log = append(log, "root")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(leafProvider)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(rootProvider)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	// Sanity: Assemble returns root-first.
	if modules[0] != rootModule {
		t.Fatalf("Assemble() = %v, want root first", modules)
	}

	resolveProvider(t, rootProvider)
	resolveProvider(t, leafProvider)

	if err := runModuleInitPhase(context.Background(), modules); err != nil {
		t.Fatalf("runModuleInitPhase() error = %v", err)
	}

	if len(log) != 2 || log[0] != "leaf" || log[1] != "root" {
		t.Fatalf("log = %v, want [leaf root] (leaf-first)", log)
	}
}

type fiP1Type struct{}
type fiP2Type struct{}
type fiP3Type struct{}

// TestRunModuleInitPhase_FailFast proves the first non-nil error returned
// by a provider's OnModuleInit hook stops the entire phase immediately: no
// later provider -- whether in the same module or a subsequent one in the
// leaf-first walk -- runs afterward. p1 and p2 live in the leaf module
// (visited first, leaf-first); p2 errors; p3 lives in the root module
// (visited second) and must never run.
func TestRunModuleInitPhase_FailFast(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *fiP1Type { return &fiP1Type{} })
		p.OnModuleInit(func(*fiP1Type) {
			log = append(log, "p1")
		})
	})
	p2 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *fiP2Type { return &fiP2Type{} })
		p.OnModuleInit(func(*fiP2Type) error {
			log = append(log, "p2")
			return wantErr
		})
	})
	p3 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *fiP3Type { return &fiP3Type{} })
		p.OnModuleInit(func(*fiP3Type) {
			log = append(log, "p3")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(p1, p2)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(p3)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, p1)
	resolveProvider(t, p2)
	resolveProvider(t, p3)

	err = runModuleInitPhase(context.Background(), modules)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runModuleInitPhase() error = %v, want %v", err, wantErr)
	}

	if len(log) != 2 || log[0] != "p1" || log[1] != "p2" {
		t.Fatalf("log = %v, want exactly [p1 p2] -- p3 must never run after p2's error", log)
	}
}

type mxHookedType struct{}
type mxUnhookedType struct{}

// TestRunModuleInitPhase_MixedHookedAndUnhooked proves a provider that
// never registered OnModuleInit is silently skipped (no panic, no error)
// alongside one that did, within the same phase call.
func TestRunModuleInitPhase_MixedHookedAndUnhooked(t *testing.T) {
	var log []string

	hooked := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mxHookedType { return &mxHookedType{} })
		p.OnModuleInit(func(*mxHookedType) {
			log = append(log, "hooked")
		})
	})
	unhooked := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mxUnhookedType { return &mxUnhookedType{} })
		// No OnModuleInit call at all.
	})

	root := module.New(func(m *module.Module) {
		m.Providers(hooked, unhooked)
	})

	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, hooked)
	resolveProvider(t, unhooked)

	if err := runModuleInitPhase(context.Background(), modules); err != nil {
		t.Fatalf("runModuleInitPhase() error = %v", err)
	}

	if len(log) != 1 || log[0] != "hooked" {
		t.Fatalf("log = %v, want exactly [hooked]", log)
	}
}

type abRootType struct{}
type abLeafType struct{}

// TestRunApplicationBootstrapPhase_LeafFirstOrder mirrors
// TestRunModuleInitPhase_LeafFirstOrder exactly, but for the
// OnApplicationBootstrap hook/runApplicationBootstrapPhase pair -- proving
// the same leaf-first traversal shape holds for both phase functions
// independently.
func TestRunApplicationBootstrapPhase_LeafFirstOrder(t *testing.T) {
	var log []string

	leafProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abLeafType { return &abLeafType{} })
		p.OnApplicationBootstrap(func(*abLeafType) {
			log = append(log, "leaf")
		})
	})
	rootProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abRootType { return &abRootType{} })
		p.OnApplicationBootstrap(func(*abRootType) {
			log = append(log, "root")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(leafProvider)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(rootProvider)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, rootProvider)
	resolveProvider(t, leafProvider)

	if err := runApplicationBootstrapPhase(context.Background(), modules); err != nil {
		t.Fatalf("runApplicationBootstrapPhase() error = %v", err)
	}

	if len(log) != 2 || log[0] != "leaf" || log[1] != "root" {
		t.Fatalf("log = %v, want [leaf root] (leaf-first)", log)
	}
}

type abP1Type struct{}
type abP2Type struct{}
type abP3Type struct{}

// TestRunApplicationBootstrapPhase_FailFast mirrors
// TestRunModuleInitPhase_FailFast for runApplicationBootstrapPhase.
func TestRunApplicationBootstrapPhase_FailFast(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abP1Type { return &abP1Type{} })
		p.OnApplicationBootstrap(func(*abP1Type) {
			log = append(log, "p1")
		})
	})
	p2 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abP2Type { return &abP2Type{} })
		p.OnApplicationBootstrap(func(*abP2Type) error {
			log = append(log, "p2")
			return wantErr
		})
	})
	p3 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abP3Type { return &abP3Type{} })
		p.OnApplicationBootstrap(func(*abP3Type) {
			log = append(log, "p3")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(p1, p2)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(p3)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, p1)
	resolveProvider(t, p2)
	resolveProvider(t, p3)

	err = runApplicationBootstrapPhase(context.Background(), modules)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runApplicationBootstrapPhase() error = %v, want %v", err, wantErr)
	}

	if len(log) != 2 || log[0] != "p1" || log[1] != "p2" {
		t.Fatalf("log = %v, want exactly [p1 p2] -- p3 must never run after p2's error", log)
	}
}

type abMxHookedType struct{}
type abMxUnhookedType struct{}

// TestRunApplicationBootstrapPhase_MixedHookedAndUnhooked mirrors
// TestRunModuleInitPhase_MixedHookedAndUnhooked for
// runApplicationBootstrapPhase.
func TestRunApplicationBootstrapPhase_MixedHookedAndUnhooked(t *testing.T) {
	var log []string

	hooked := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abMxHookedType { return &abMxHookedType{} })
		p.OnApplicationBootstrap(func(*abMxHookedType) {
			log = append(log, "hooked")
		})
	})
	unhooked := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *abMxUnhookedType { return &abMxUnhookedType{} })
		// No OnApplicationBootstrap call at all.
	})

	root := module.New(func(m *module.Module) {
		m.Providers(hooked, unhooked)
	})

	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, hooked)
	resolveProvider(t, unhooked)

	if err := runApplicationBootstrapPhase(context.Background(), modules); err != nil {
		t.Fatalf("runApplicationBootstrapPhase() error = %v", err)
	}

	if len(log) != 1 || log[0] != "hooked" {
		t.Fatalf("log = %v, want exactly [hooked]", log)
	}
}

type mdRootType struct{}
type mdLeafType struct{}

// TestRunModuleDestroyPhase_RootFirstOrder proves runModuleDestroyPhase
// visits modules in Assemble's OWN root-first order (unreversed) -- the
// opposite of runModuleInitPhase's leaf-first walk, since teardown is
// expected to run in the reverse order of startup.
func TestRunModuleDestroyPhase_RootFirstOrder(t *testing.T) {
	var log []string

	leafProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdLeafType { return &mdLeafType{} })
		p.OnModuleDestroy(func(*mdLeafType) {
			log = append(log, "leaf")
		})
	})
	rootProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdRootType { return &mdRootType{} })
		p.OnModuleDestroy(func(*mdRootType) {
			log = append(log, "root")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(leafProvider)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(rootProvider)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if modules[0] != rootModule {
		t.Fatalf("Assemble() = %v, want root first", modules)
	}

	resolveProvider(t, rootProvider)
	resolveProvider(t, leafProvider)

	if err := runModuleDestroyPhase(context.Background(), modules); err != nil {
		t.Fatalf("runModuleDestroyPhase() error = %v", err)
	}

	if len(log) != 2 || log[0] != "root" || log[1] != "leaf" {
		t.Fatalf("log = %v, want [root leaf] (root-first)", log)
	}
}

type mdP1Type struct{}
type mdP2Type struct{}
type mdP3Type struct{}

// TestRunModuleDestroyPhase_FailFast mirrors TestRunModuleInitPhase_FailFast
// for runModuleDestroyPhase: p1 lives in the root module (visited first,
// root-first), p1 errors, and p2/p3 (leaf module, visited second) must
// never run.
func TestRunModuleDestroyPhase_FailFast(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP1Type { return &mdP1Type{} })
		p.OnModuleDestroy(func(*mdP1Type) error {
			log = append(log, "p1")
			return wantErr
		})
	})
	p2 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP2Type { return &mdP2Type{} })
		p.OnModuleDestroy(func(*mdP2Type) {
			log = append(log, "p2")
		})
	})
	p3 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP3Type { return &mdP3Type{} })
		p.OnModuleDestroy(func(*mdP3Type) {
			log = append(log, "p3")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(p2, p3)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(p1)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, p1)
	resolveProvider(t, p2)
	resolveProvider(t, p3)

	err = runModuleDestroyPhase(context.Background(), modules)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runModuleDestroyPhase() error = %v, want %v", err, wantErr)
	}

	if len(log) != 1 || log[0] != "p1" {
		t.Fatalf("log = %v, want exactly [p1] -- p2/p3 must never run after p1's error", log)
	}
}

type basRootType struct{}
type basLeafType struct{}

// TestRunBeforeApplicationShutdownPhase_RootFirstOrderAndSignal proves
// runBeforeApplicationShutdownPhase visits modules root-first (same
// traversal as runModuleDestroyPhase) and forwards signal unchanged to
// every hook invocation.
func TestRunBeforeApplicationShutdownPhase_RootFirstOrderAndSignal(t *testing.T) {
	var log []string
	var gotSignals []string

	leafProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *basLeafType { return &basLeafType{} })
		p.BeforeApplicationShutdown(func(_ *basLeafType, signal string) {
			log = append(log, "leaf")
			gotSignals = append(gotSignals, signal)
		})
	})
	rootProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *basRootType { return &basRootType{} })
		p.BeforeApplicationShutdown(func(_ *basRootType, signal string) {
			log = append(log, "root")
			gotSignals = append(gotSignals, signal)
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(leafProvider)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(rootProvider)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, rootProvider)
	resolveProvider(t, leafProvider)

	if err := runBeforeApplicationShutdownPhase(context.Background(), modules, "SIGTERM"); err != nil {
		t.Fatalf("runBeforeApplicationShutdownPhase() error = %v", err)
	}

	if len(log) != 2 || log[0] != "root" || log[1] != "leaf" {
		t.Fatalf("log = %v, want [root leaf] (root-first)", log)
	}
	if len(gotSignals) != 2 || gotSignals[0] != "SIGTERM" || gotSignals[1] != "SIGTERM" {
		t.Fatalf("gotSignals = %v, want [SIGTERM SIGTERM]", gotSignals)
	}
}

// TestRunBeforeApplicationShutdownPhase_FailFast mirrors
// TestRunModuleDestroyPhase_FailFast for runBeforeApplicationShutdownPhase.
func TestRunBeforeApplicationShutdownPhase_FailFast(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP1Type { return &mdP1Type{} })
		p.BeforeApplicationShutdown(func(_ *mdP1Type, signal string) error {
			log = append(log, "p1")
			return wantErr
		})
	})
	p2 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP2Type { return &mdP2Type{} })
		p.BeforeApplicationShutdown(func(_ *mdP2Type, signal string) {
			log = append(log, "p2")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(p2)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(p1)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, p1)
	resolveProvider(t, p2)

	err = runBeforeApplicationShutdownPhase(context.Background(), modules, "SIGTERM")
	if !errors.Is(err, wantErr) {
		t.Fatalf("runBeforeApplicationShutdownPhase() error = %v, want %v", err, wantErr)
	}

	if len(log) != 1 || log[0] != "p1" {
		t.Fatalf("log = %v, want exactly [p1] -- p2 must never run after p1's error", log)
	}
}

type asRootType struct{}
type asLeafType struct{}

// TestRunApplicationShutdownPhase_RootFirstOrderAndSignal mirrors
// TestRunBeforeApplicationShutdownPhase_RootFirstOrderAndSignal for
// runApplicationShutdownPhase.
func TestRunApplicationShutdownPhase_RootFirstOrderAndSignal(t *testing.T) {
	var log []string
	var gotSignals []string

	leafProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *asLeafType { return &asLeafType{} })
		p.OnApplicationShutdown(func(_ *asLeafType, signal string) {
			log = append(log, "leaf")
			gotSignals = append(gotSignals, signal)
		})
	})
	rootProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *asRootType { return &asRootType{} })
		p.OnApplicationShutdown(func(_ *asRootType, signal string) {
			log = append(log, "root")
			gotSignals = append(gotSignals, signal)
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(leafProvider)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(rootProvider)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, rootProvider)
	resolveProvider(t, leafProvider)

	if err := runApplicationShutdownPhase(context.Background(), modules, "SIGINT"); err != nil {
		t.Fatalf("runApplicationShutdownPhase() error = %v", err)
	}

	if len(log) != 2 || log[0] != "root" || log[1] != "leaf" {
		t.Fatalf("log = %v, want [root leaf] (root-first)", log)
	}
	if len(gotSignals) != 2 || gotSignals[0] != "SIGINT" || gotSignals[1] != "SIGINT" {
		t.Fatalf("gotSignals = %v, want [SIGINT SIGINT]", gotSignals)
	}
}

// TestRunApplicationShutdownPhase_FailFast mirrors
// TestRunModuleDestroyPhase_FailFast for runApplicationShutdownPhase.
func TestRunApplicationShutdownPhase_FailFast(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP1Type { return &mdP1Type{} })
		p.OnApplicationShutdown(func(_ *mdP1Type, signal string) error {
			log = append(log, "p1")
			return wantErr
		})
	})
	p2 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP2Type { return &mdP2Type{} })
		p.OnApplicationShutdown(func(_ *mdP2Type, signal string) {
			log = append(log, "p2")
		})
	})

	leafModule := module.New(func(m *module.Module) {
		m.Providers(p2)
	})
	rootModule := module.New(func(m *module.Module) {
		m.Providers(p1)
		m.Imports(leafModule)
	})

	modules, err := rootModule.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	resolveProvider(t, p1)
	resolveProvider(t, p2)

	err = runApplicationShutdownPhase(context.Background(), modules, "SIGTERM")
	if !errors.Is(err, wantErr) {
		t.Fatalf("runApplicationShutdownPhase() error = %v, want %v", err, wantErr)
	}

	if len(log) != 1 || log[0] != "p1" {
		t.Fatalf("log = %v, want exactly [p1] -- p2 must never run after p1's error", log)
	}
}

// fakeShutdownAdapter is a minimal HttpAdapter stub used only to drive
// App.shutdown/Close's orchestration in isolation -- shutdownCalls counts
// invocations (proving shutdown is called exactly once even if shutdown()
// itself is invoked multiple times), and shutdownErr, if non-nil, is
// returned verbatim to prove a failing adapter.Shutdown aborts the 3 hook
// phases entirely.
type fakeShutdownAdapter struct {
	shutdownCalls int
	shutdownErr   error
}

func (f *fakeShutdownAdapter) Init(opts Options) {}
func (f *fakeShutdownAdapter) RegisterRoute(method route.HttpMethod, path string, h func(c *execution.HttpContext)) error {
	return nil
}
func (f *fakeShutdownAdapter) Listen(addr string, onListen func()) error { return nil }
func (f *fakeShutdownAdapter) Test(req *http.Request) (*http.Response, error) {
	return nil, nil
}
func (f *fakeShutdownAdapter) Shutdown(ctx context.Context) error {
	f.shutdownCalls++
	return f.shutdownErr
}

// newShutdownTestApp builds a bare *App suitable for exercising
// shutdown/Close/EnableShutdownHooks/runShutdownSequence directly, without
// going through the full NewApp[T] bootstrap (DI graph, routes, etc. are
// irrelevant to these tests). shutdownDone is initialized here, mirroring
// what NewApp itself now does, since runShutdownSequence unconditionally
// closes it.
func newShutdownTestApp(adapter HttpAdapter, modules []*module.Module) *App {
	return &App{
		adapter:      adapter,
		modules:      modules,
		shutdownDone: make(chan struct{}),
	}
}

// TestShutdown_AlwaysCallsAdapterShutdown proves a.adapter.Shutdown(ctx)
// runs regardless of shutdownHooksEnabled -- HTTP draining is not gated by
// that flag.
func TestShutdown_AlwaysCallsAdapterShutdown(t *testing.T) {
	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, nil)

	if err := a.shutdown(context.Background(), ""); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	if adapter.shutdownCalls != 1 {
		t.Fatalf("adapter.shutdownCalls = %d, want 1", adapter.shutdownCalls)
	}
}

// TestShutdown_HooksDisabled_SkipsDestroyPhases proves that with
// shutdownHooksEnabled left false (the default), shutdown still calls
// adapter.Shutdown but none of the 3 destroy phases run.
func TestShutdown_HooksDisabled_SkipsDestroyPhases(t *testing.T) {
	var log []string
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mdP1Type { return &mdP1Type{} })
		p.OnModuleDestroy(func(*mdP1Type) {
			log = append(log, "destroy")
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	resolveProvider(t, p)

	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, modules)
	// shutdownHooksEnabled left false deliberately.

	if err := a.shutdown(context.Background(), ""); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	if adapter.shutdownCalls != 1 {
		t.Fatalf("adapter.shutdownCalls = %d, want 1", adapter.shutdownCalls)
	}
	if len(log) != 0 {
		t.Fatalf("log = %v, want empty -- destroy phase must not run when shutdownHooksEnabled is false", log)
	}
}

type shP1Type struct{}
type shP2Type struct{}
type shP3Type struct{}

// TestShutdown_HooksEnabled_AbortsRemainingPhasesOnError proves that when
// shutdownHooksEnabled is true and runModuleDestroyPhase's hook errors, the
// BeforeApplicationShutdown/OnApplicationShutdown phases never run
// afterward.
func TestShutdown_HooksEnabled_AbortsRemainingPhasesOnError(t *testing.T) {
	var log []string
	wantErr := errors.New("boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *shP1Type { return &shP1Type{} })
		p.OnModuleDestroy(func(*shP1Type) error {
			log = append(log, "destroy")
			return wantErr
		})
		p.BeforeApplicationShutdown(func(_ *shP1Type, signal string) {
			log = append(log, "before-shutdown")
		})
		p.OnApplicationShutdown(func(_ *shP1Type, signal string) {
			log = append(log, "shutdown")
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p1)
	})
	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	resolveProvider(t, p1)

	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, modules)
	a.shutdownHooksEnabled = true

	gotErr := a.shutdown(context.Background(), "SIGTERM")
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("shutdown() error = %v, want %v", gotErr, wantErr)
	}
	if len(log) != 1 || log[0] != "destroy" {
		t.Fatalf("log = %v, want exactly [destroy] -- BeforeApplicationShutdown/OnApplicationShutdown must never run", log)
	}
}

// TestShutdown_AdapterErrorSkipsHookPhases proves a non-nil error from
// adapter.Shutdown itself aborts before any of the 3 hook phases run at
// all, even with shutdownHooksEnabled true.
func TestShutdown_AdapterErrorSkipsHookPhases(t *testing.T) {
	var log []string
	wantErr := errors.New("adapter boom")

	p1 := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *shP2Type { return &shP2Type{} })
		p.OnModuleDestroy(func(*shP2Type) {
			log = append(log, "destroy")
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p1)
	})
	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	resolveProvider(t, p1)

	adapter := &fakeShutdownAdapter{shutdownErr: wantErr}
	a := newShutdownTestApp(adapter, modules)
	a.shutdownHooksEnabled = true

	gotErr := a.shutdown(context.Background(), "")
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("shutdown() error = %v, want %v", gotErr, wantErr)
	}
	if len(log) != 0 {
		t.Fatalf("log = %v, want empty -- hook phases must never run when adapter.Shutdown errors", log)
	}
}

// TestShutdown_RunsOnlyOnce proves calling shutdown twice (directly, and
// via Close) runs the destroy sequence's adapter.Shutdown call only once,
// and both calls return the exact same error the first computed.
func TestShutdown_RunsOnlyOnce(t *testing.T) {
	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, nil)

	err1 := a.shutdown(context.Background(), "")
	err2 := a.shutdown(context.Background(), "")
	err3 := a.Close(context.Background())

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("errs = %v, %v, %v, want all nil", err1, err2, err3)
	}
	if adapter.shutdownCalls != 1 {
		t.Fatalf("adapter.shutdownCalls = %d, want 1 (shutdown must run at most once)", adapter.shutdownCalls)
	}
}

// TestShutdown_RunsOnlyOnce_SameErrorOnRepeat proves the SAME error the
// first shutdown call computed is returned on every subsequent call, not a
// second (possibly different, e.g. nil) result.
func TestShutdown_RunsOnlyOnce_SameErrorOnRepeat(t *testing.T) {
	wantErr := errors.New("boom")
	adapter := &fakeShutdownAdapter{shutdownErr: wantErr}
	a := newShutdownTestApp(adapter, nil)

	err1 := a.shutdown(context.Background(), "")
	err2 := a.shutdown(context.Background(), "")

	if !errors.Is(err1, wantErr) {
		t.Fatalf("err1 = %v, want %v", err1, wantErr)
	}
	if err1 != err2 {
		t.Fatalf("err2 = %v, want the exact same error as err1 (%v)", err2, err1)
	}
	if adapter.shutdownCalls != 1 {
		t.Fatalf("adapter.shutdownCalls = %d, want 1", adapter.shutdownCalls)
	}
}

// TestShutdown_ClosesShutdownDone proves runShutdownSequence closes
// a.shutdownDone exactly once, observable via a non-blocking receive after
// shutdown returns.
func TestShutdown_ClosesShutdownDone(t *testing.T) {
	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, nil)

	if err := a.shutdown(context.Background(), ""); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	select {
	case <-a.shutdownDone:
		// closed, as expected
	default:
		t.Fatalf("a.shutdownDone was not closed after shutdown() returned")
	}
}

// TestClose_DelegatesToShutdownWithEmptySignal proves Close(ctx) calls
// shutdown(ctx, "") and returns its error directly.
func TestClose_DelegatesToShutdownWithEmptySignal(t *testing.T) {
	var gotSignal string
	sawSignal := false

	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *shP3Type { return &shP3Type{} })
		p.OnApplicationShutdown(func(_ *shP3Type, signal string) {
			sawSignal = true
			gotSignal = signal
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	modules, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	resolveProvider(t, p)

	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, modules)
	a.shutdownHooksEnabled = true

	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !sawSignal {
		t.Fatalf("OnApplicationShutdown hook never ran via Close()")
	}
	if gotSignal != "" {
		t.Fatalf("gotSignal = %q, want empty string for a Close()-triggered shutdown", gotSignal)
	}
}

// TestEnableShutdownHooks_ReturnsSameApp proves EnableShutdownHooks is
// chainable -- it returns the same *App it was called on.
func TestEnableShutdownHooks_ReturnsSameApp(t *testing.T) {
	adapter := &fakeShutdownAdapter{}
	a := newShutdownTestApp(adapter, nil)

	got := a.EnableShutdownHooks()
	if got != a {
		t.Fatalf("EnableShutdownHooks() = %p, want the same *App (%p)", got, a)
	}
	if !a.shutdownHooksEnabled {
		t.Fatalf("shutdownHooksEnabled = false, want true after EnableShutdownHooks()")
	}

	// Drain the shutdown sequence via Close so the signal-watching goroutine
	// EnableShutdownHooks started (blocked forever on signal.Notify's
	// channel, since no real signal is sent in this test) does not leak past
	// the test -- shutdown's sync.Once means this Close call is the one that
	// actually runs teardown; the goroutine's own later call would just be a
	// no-op against the same sync.Once.
	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
