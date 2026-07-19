package app

import (
	"context"
	"sync"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// e2eLifecycleType is the resolved type of the single Provider both tests in
// this file register -- distinct per test function (see leType2 below) so
// the two tests' independent module trees never share any DI state, even
// though both live in the same package and run in the same test binary.
type e2eLifecycleType struct{}

// e2eLifecycleRecorder centralizes the "ordered []string slice, mutex
// protected" pattern this task's spec calls for: every one of the 5
// lifecycle hooks appends its own distinct label via record, and the test
// reads back a stable snapshot via snapshot() rather than reading the
// backing slice directly -- avoiding a data race between a hook goroutine
// (there is none here, NewApp/Close both run these hooks synchronously on
// the calling goroutine, but the recorder is written this way regardless so
// it stays correct if that ever changes) and the test's own assertions.
//
// gotSignal/sawSignal capture what OnApplicationShutdown actually received
// as its trailing `signal string` argument -- scenario (c) needs to assert
// on the exact value ("" for a Close-triggered shutdown), not just that the
// hook ran at all.
type e2eLifecycleRecorder struct {
	mu        sync.Mutex
	log       []string
	sawSignal bool
	gotSignal string
}

func (r *e2eLifecycleRecorder) record(label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, label)
}

func (r *e2eLifecycleRecorder) recordSignal(label, signal string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, label)
	r.sawSignal = true
	r.gotSignal = signal
}

func (r *e2eLifecycleRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.log))
	copy(out, r.log)
	return out
}

// newE2ELifecycleProvider builds a *provider.Provider whose Constructor
// resolves to a fresh *e2eLifecycleType and whose 5 lifecycle hooks each
// append their own distinct label to rec -- one Provider exercising all 5
// hook kinds this feature added (OnModuleInit/OnApplicationBootstrap at
// bootstrap time, OnModuleDestroy/BeforeApplicationShutdown/
// OnApplicationShutdown at shutdown time), reused by both tests in this
// file with their own independent recorder.
func newE2ELifecycleProvider(rec *e2eLifecycleRecorder) *provider.Provider {
	return provider.New(func(p *provider.Provider) {
		p.Constructor(func() *e2eLifecycleType { return &e2eLifecycleType{} })
		p.OnModuleInit(func(*e2eLifecycleType) {
			rec.record("OnModuleInit")
		})
		p.OnApplicationBootstrap(func(*e2eLifecycleType) {
			rec.record("OnApplicationBootstrap")
		})
		p.OnModuleDestroy(func(*e2eLifecycleType) {
			rec.record("OnModuleDestroy")
		})
		p.BeforeApplicationShutdown(func(_ *e2eLifecycleType, signal string) {
			rec.record("BeforeApplicationShutdown")
		})
		p.OnApplicationShutdown(func(_ *e2eLifecycleType, signal string) {
			rec.recordSignal("OnApplicationShutdown", signal)
		})
	})
}

// assertLog fails t with both the got and want slices rendered if got does
// not match want exactly, element by element (order matters -- that is the
// entire point of this test file).
func assertLog(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("log = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("log = %v, want %v", got, want)
		}
	}
}

// TestLifecycleE2E_FullSequenceViaNewAppAndClose proves the whole 5-hook
// lifecycle fires in the exact documented order across a real NewApp[T]
// bootstrap followed by EnableShutdownHooks + Close: scenarios (a), (b) and
// (c) from this task's spec, all against the SAME *App instance (the destroy
// phases are sync.Once-guarded, so (a) must observe the log BEFORE (b)
// triggers the shutdown-time hooks, not after).
func TestLifecycleE2E_FullSequenceViaNewAppAndClose(t *testing.T) {
	rec := &e2eLifecycleRecorder{}
	p := newE2ELifecycleProvider(rec)
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	app, err := NewApp[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	// Scenario (a): immediately after NewApp returns, only the 2
	// bootstrap-time hooks have run, in order -- nothing destroy-phase yet.
	assertLog(t, rec.snapshot(), []string{"OnModuleInit", "OnApplicationBootstrap"})

	// Scenario (b): EnableShutdownHooks + Close appends the 3 destroy-phase
	// labels strictly after the 2 already recorded.
	app.EnableShutdownHooks()
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertLog(t, rec.snapshot(), []string{
		"OnModuleInit",
		"OnApplicationBootstrap",
		"OnModuleDestroy",
		"BeforeApplicationShutdown",
		"OnApplicationShutdown",
	})

	// Scenario (c): OnApplicationShutdown must have received the documented
	// manual-Close sentinel signal, the empty string.
	rec.mu.Lock()
	sawSignal, gotSignal := rec.sawSignal, rec.gotSignal
	rec.mu.Unlock()
	if !sawSignal {
		t.Fatalf("OnApplicationShutdown hook never ran")
	}
	if gotSignal != "" {
		t.Fatalf("OnApplicationShutdown signal = %q, want empty string for a Close()-triggered shutdown", gotSignal)
	}
}

// e2eLifecycleTypeNoHooks is a SEPARATE resolved type from
// e2eLifecycleType, used only by TestLifecycleE2E_CloseWithoutShutdownHooksEnabled
// below -- scenario (d) requires a completely fresh *App/Provider/module
// tree (Close/shutdown are sync.Once-guarded and only ever run once per
// *App), so this test must not share any state, including its DI types,
// with TestLifecycleE2E_FullSequenceViaNewAppAndClose above.
type e2eLifecycleTypeNoHooks struct{}

// TestLifecycleE2E_CloseWithoutShutdownHooksEnabled proves scenario (d):
// calling Close on a fresh *App that never called EnableShutdownHooks still
// returns nil (the adapter itself always drains, unconditionally -- see
// runShutdownSequence's own doc comment), but none of the 3 destroy-phase
// hooks fire -- the log stays exactly what NewApp's own bootstrap already
// produced.
func TestLifecycleE2E_CloseWithoutShutdownHooksEnabled(t *testing.T) {
	rec := &e2eLifecycleRecorder{}
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *e2eLifecycleTypeNoHooks { return &e2eLifecycleTypeNoHooks{} })
		p.OnModuleInit(func(*e2eLifecycleTypeNoHooks) {
			rec.record("OnModuleInit")
		})
		p.OnApplicationBootstrap(func(*e2eLifecycleTypeNoHooks) {
			rec.record("OnApplicationBootstrap")
		})
		p.OnModuleDestroy(func(*e2eLifecycleTypeNoHooks) {
			rec.record("OnModuleDestroy")
		})
		p.BeforeApplicationShutdown(func(_ *e2eLifecycleTypeNoHooks, signal string) {
			rec.record("BeforeApplicationShutdown")
		})
		p.OnApplicationShutdown(func(_ *e2eLifecycleTypeNoHooks, signal string) {
			rec.recordSignal("OnApplicationShutdown", signal)
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	app, err := NewApp[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	assertLog(t, rec.snapshot(), []string{"OnModuleInit", "OnApplicationBootstrap"})

	// EnableShutdownHooks deliberately never called.
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v, want nil even without EnableShutdownHooks", err)
	}

	assertLog(t, rec.snapshot(), []string{"OnModuleInit", "OnApplicationBootstrap"})
}
