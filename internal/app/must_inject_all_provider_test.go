package app

// This file covers T7-T10 of the provider-must-inject-all feature
// (.specs/features/provider-must-inject-all/{spec,design,tasks}.md):
// integration tests exercising gonest.MustInjectAll[T](p) (a
// *provider.Provider owner) through a REAL New/MustNewApp bootstrap, not
// just internal/inject's own unit tests (T1-T6, already passing). All 4
// tasks live in this single file, added in T7->T8->T9->T10 order per
// tasks.md's own execution note (same-file sequential, not parallel
// agents).
//
// internal/app cannot import the root gonest package (gonest.go itself
// imports internal/app to build its App/AppOptions aliases -- the reverse
// import would cycle), so every test below reaches the feature through
// internal/inject.MustAll[T](p) directly (the exact function
// gonest.MustInjectAll[T] forwards to) plus internal/module,
// internal/provider and internal/scope -- mirroring the same "public API
// but at the internal/app package's own reachable layer" pattern already
// used throughout this package's other test files (e.g.
// TestNewApp_UserProviderExample_ResolvesUsableUserService in app_test.go,
// which resolves *UserService via inject.Must[*UserService](p) inside a
// Constructor closure the exact same way).

import (
	"strings"
	"sync"
	"testing"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/scope"
)

// mustInjectAllPingable is the shared target interface every fixture below
// implements (or, for T8, deliberately does not) -- mirrors
// INSIGHT-MUST-INJECT-ALL.md's port.Pingable example.
type mustInjectAllPingable interface {
	Ping() string
}

// ---------------------------------------------------------------------------
// T7 -- happy path: 2+ concrete Providers in different imported modules,
// each exported via module.ProviderAs[mustInjectAllPingable], plus one
// Provider whose Constructor closure calls inject.MustAll[mustInjectAllPingable](p)
// and captures the result.
// ---------------------------------------------------------------------------

type mustInjectAllSQLAdapter struct{}

func (a *mustInjectAllSQLAdapter) Ping() string { return "sql" }

type mustInjectAllCacheAdapter struct{}

func (a *mustInjectAllCacheAdapter) Ping() string { return "cache" }

// TestMustInjectAll_HappyPath_TwoModulesRealBootstrap proves REQ-001/
// REQ-002 end to end: a Provider-owned MustInjectAll[T] call, made inside a
// real New() bootstrap, returns a slice containing every Singleton provider
// (from the owner's own module AND its imports) that was exported via
// module.ProviderAs[mustInjectAllPingable] -- with the concrete values
// genuinely populated once New() returns (not just correctly-sized).
func TestMustInjectAll_HappyPath_TwoModulesRealBootstrap(t *testing.T) {
	sqlAdapter := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllSQLAdapter { return &mustInjectAllSQLAdapter{} })
	})
	sqlAsPingable := module.ProviderAs[mustInjectAllPingable](sqlAdapter)
	sqlModule := module.New(func(m *module.Module) {
		m.Providers(sqlAdapter, sqlAsPingable)
		m.Exports(sqlAsPingable)
	})

	cacheAdapter := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllCacheAdapter { return &mustInjectAllCacheAdapter{} })
	})
	cacheAsPingable := module.ProviderAs[mustInjectAllPingable](cacheAdapter)
	cacheModule := module.New(func(m *module.Module) {
		m.Providers(cacheAdapter, cacheAsPingable)
		m.Exports(cacheAsPingable)
	})

	var pingables []mustInjectAllPingable
	type healthUsecase struct{}
	healthProvider := provider.New(func(p *provider.Provider) {
		pingables = inject.MustAll[mustInjectAllPingable](p)
		p.Constructor(func() *healthUsecase { return &healthUsecase{} })
	})

	root := module.New(func(m *module.Module) {
		m.Imports(sqlModule, cacheModule)
		m.Providers(healthProvider)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil *App")
	}

	if len(pingables) != 2 {
		t.Fatalf("MustInjectAll captured slice len = %d, want 2 -- got %+v", len(pingables), pingables)
	}

	var haveSQL, haveCache bool
	for _, p := range pingables {
		switch p.(type) {
		case *mustInjectAllSQLAdapter:
			haveSQL = true
		case *mustInjectAllCacheAdapter:
			haveCache = true
		}
	}
	if !haveSQL {
		t.Errorf("MustInjectAll slice = %+v, want it to contain a *mustInjectAllSQLAdapter", pingables)
	}
	if !haveCache {
		t.Errorf("MustInjectAll slice = %+v, want it to contain a *mustInjectAllCacheAdapter", pingables)
	}
}

// ---------------------------------------------------------------------------
// T8 -- zero matches: no provider in scope implements the target interface.
// ---------------------------------------------------------------------------

// mustInjectAllUnrelated implements no methods of mustInjectAllPingable --
// used only to prove a module with NO matching provider still bootstraps
// cleanly and yields an empty (non-panicking) MustInjectAll slice.
type mustInjectAllUnrelated struct{}

// TestMustInjectAll_ZeroMatches_ReturnsEmptySliceNoPanic proves REQ-003: a
// Provider-owned MustInjectAll[T] call with zero candidates in scope does
// NOT panic New()/MustNewApp(), and the captured slice has length 0.
func TestMustInjectAll_ZeroMatches_ReturnsEmptySliceNoPanic(t *testing.T) {
	unrelated := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllUnrelated { return &mustInjectAllUnrelated{} })
	})

	var pingables []mustInjectAllPingable
	type healthUsecase struct{}
	healthProvider := provider.New(func(p *provider.Provider) {
		pingables = inject.MustAll[mustInjectAllPingable](p)
		p.Constructor(func() *healthUsecase { return &healthUsecase{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(unrelated, healthProvider)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("New() error = %v, want nil (zero matches must not fail bootstrap)", err)
	}
	if app == nil {
		t.Fatal("New() returned nil *App")
	}

	if pingables == nil {
		t.Fatal("MustInjectAll captured slice is nil, want a non-nil (possibly empty) slice per REQ-002/mustAllProvider's reflect.MakeSlice")
	}
	if len(pingables) != 0 {
		t.Fatalf("MustInjectAll captured slice len = %d, want 0 (no provider in scope implements mustInjectAllPingable)", len(pingables))
	}
}

// ---------------------------------------------------------------------------
// T9 -- a scope.Transient member panics NewApp/MustNewApp fail-fast, per
// REQ-005 (T3's mustAllProvider check).
// ---------------------------------------------------------------------------

type mustInjectAllTransientAdapter struct{}

func (a *mustInjectAllTransientAdapter) Ping() string { return "transient" }

// TestMustInjectAll_TransientMember_PanicsNewApp proves REQ-005: a
// scope.Transient provider that implements the target interface and sits in
// the caller's scope makes New()/MustNewApp() panic with T3's exact
// mustAllProvider message -- fail-fast, no partial app returned.
func TestMustInjectAll_TransientMember_PanicsNewApp(t *testing.T) {
	transientAdapter := provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		p.Constructor(func() *mustInjectAllTransientAdapter { return &mustInjectAllTransientAdapter{} })
	})
	transientAsPingable := module.ProviderAs[mustInjectAllPingable](transientAdapter)

	type healthUsecase struct{}
	healthProvider := provider.New(func(p *provider.Provider) {
		_ = inject.MustAll[mustInjectAllPingable](p)
		p.Constructor(func() *healthUsecase { return &healthUsecase{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(transientAdapter, transientAsPingable, healthProvider)
	})

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("New() did not panic, want a panic naming the Transient member (REQ-005)")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("recover() = %v (%T), want a string panic message", rec, rec)
		}
		const want = "only ScopeSingleton providers can be members of a Provider-owned MustInjectAll slice"
		if !strings.Contains(msg, want) {
			t.Fatalf("panic message = %q, want it to contain %q", msg, want)
		}
	}()

	app, err := New[recordingFakeAdapter](root, Options{})
	if err == nil && app != nil {
		t.Fatal("New() returned a usable *App, want a panic (Transient member in a MustInjectAll scope)")
	}
}

// TestMustNewApp_TransientMember_Panics proves the same REQ-005 fail-fast
// through MustNewApp's own panic-on-error wrapper -- both entry points must
// refuse to hand back a partial app.
func TestMustNewApp_TransientMember_Panics(t *testing.T) {
	transientAdapter := provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		p.Constructor(func() *mustInjectAllTransientAdapter { return &mustInjectAllTransientAdapter{} })
	})
	transientAsPingable := module.ProviderAs[mustInjectAllPingable](transientAdapter)

	type healthUsecase struct{}
	healthProvider := provider.New(func(p *provider.Provider) {
		_ = inject.MustAll[mustInjectAllPingable](p)
		p.Constructor(func() *healthUsecase { return &healthUsecase{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(transientAdapter, transientAsPingable, healthProvider)
	})

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("MustNewApp() did not panic, want a panic naming the Transient member (REQ-005)")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("recover() = %v (%T), want a string panic message", rec, rec)
		}
		const want = "only ScopeSingleton providers can be members of a Provider-owned MustInjectAll slice"
		if !strings.Contains(msg, want) {
			t.Fatalf("panic message = %q, want it to contain %q", msg, want)
		}
	}()

	MustNewApp[recordingFakeAdapter](root, Options{})
}

// ---------------------------------------------------------------------------
// T10 -- ordering guarantee: waitDeps blocks the owner Provider's Constructor
// until every matched provider's Constructor has finished, proven by a
// shared, mutex-guarded log where "owner" must always be the last entry.
// ---------------------------------------------------------------------------

// mustInjectAllOrderingLog is a small mutex-guarded []string helper, reset
// per test run, appended to by every matched provider's Constructor plus
// the owner's own Constructor.
type mustInjectAllOrderingLog struct {
	mu  sync.Mutex
	log []string
}

func (l *mustInjectAllOrderingLog) append(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.log = append(l.log, name)
}

func (l *mustInjectAllOrderingLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.log...)
}

type mustInjectAllOrderingAdapterA struct{}

func (a *mustInjectAllOrderingAdapterA) Ping() string { return "a" }

type mustInjectAllOrderingAdapterB struct{}

func (a *mustInjectAllOrderingAdapterB) Ping() string { return "b" }

type mustInjectAllOrderingAdapterC struct{}

func (a *mustInjectAllOrderingAdapterC) Ping() string { return "c" }

// TestMustInjectAll_OrderingGuarantee_OwnerConstructorRunsLast proves
// REQ-007: the dependency graph built from PendingAllEdge (owner -> every
// matched provider) really does make waitDeps hold the owner's own
// Constructor until ALL matched providers' Constructors have finished --
// not merely that the final slice ends up correctly populated. Each matched
// provider's Constructor appends its own name to a shared, mutex-guarded
// log BEFORE returning; the owner's Constructor appends "owner" to the same
// log. "owner" must always be the LAST entry. Run with -race -count=5 in
// the gate (see tasks.md's T10) to catch an ordering race that would only
// show up under repetition.
func TestMustInjectAll_OrderingGuarantee_OwnerConstructorRunsLast(t *testing.T) {
	log := &mustInjectAllOrderingLog{}

	adapterA := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllOrderingAdapterA {
			log.append("a")
			return &mustInjectAllOrderingAdapterA{}
		})
	})
	adapterAAsPingable := module.ProviderAs[mustInjectAllPingable](adapterA)

	adapterB := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllOrderingAdapterB {
			log.append("b")
			return &mustInjectAllOrderingAdapterB{}
		})
	})
	adapterBAsPingable := module.ProviderAs[mustInjectAllPingable](adapterB)

	adapterC := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectAllOrderingAdapterC {
			log.append("c")
			return &mustInjectAllOrderingAdapterC{}
		})
	})
	adapterCAsPingable := module.ProviderAs[mustInjectAllPingable](adapterC)

	type healthUsecase struct{}
	var pingables []mustInjectAllPingable
	healthProvider := provider.New(func(p *provider.Provider) {
		pingables = inject.MustAll[mustInjectAllPingable](p)
		p.Constructor(func() *healthUsecase {
			log.append("owner")
			return &healthUsecase{}
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(adapterA, adapterAAsPingable, adapterB, adapterBAsPingable, adapterC, adapterCAsPingable, healthProvider)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil *App")
	}

	if len(pingables) != 3 {
		t.Fatalf("MustInjectAll captured slice len = %d, want 3", len(pingables))
	}

	got := log.snapshot()
	if len(got) != 4 {
		t.Fatalf("ordering log = %v, want 4 entries (a, b, c, owner in some order with owner last)", got)
	}
	if got[len(got)-1] != "owner" {
		t.Fatalf("ordering log = %v, want \"owner\" to be the LAST entry -- proves waitDeps blocked the owner's Constructor until every matched provider finished", got)
	}
	seen := map[string]bool{}
	for _, name := range got[:len(got)-1] {
		seen[name] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !seen[want] {
			t.Fatalf("ordering log = %v, want it to contain %q before \"owner\"", got, want)
		}
	}
}
