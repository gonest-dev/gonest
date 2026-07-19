package app

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
)

// wiringTestTimeout bounds every channel-synchronized wait in this file --
// a deadlock guard only, never a synchronization mechanism itself (per this
// repo's no-time.Sleep-for-synchronization convention).
const wiringTestTimeout = 5 * time.Second

type wiringHookedType struct{}

// TestNewApp_InvokesModuleInitAndApplicationBootstrapHooks proves T6's core
// wiring: a Provider's OnModuleInit/OnApplicationBootstrap hooks actually
// run during NewApp, before NewApp returns, in the documented order (Init
// before Bootstrap).
func TestNewApp_InvokesModuleInitAndApplicationBootstrapHooks(t *testing.T) {
	var log []string

	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *wiringHookedType { return &wiringHookedType{} })
		p.OnModuleInit(func(*wiringHookedType) {
			log = append(log, "init")
		})
		p.OnApplicationBootstrap(func(*wiringHookedType) {
			log = append(log, "bootstrap")
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	if _, err := NewApp[recordingFakeAdapter](root, Options{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if len(log) != 2 || log[0] != "init" || log[1] != "bootstrap" {
		t.Fatalf("log = %v, want [init bootstrap]", log)
	}
}

type wiringFailingInitType struct{}

// TestNewApp_ModuleInitHookError_AbortsNewApp proves a non-nil error
// returned from an OnModuleInit hook makes NewApp itself return that exact
// error, without proceeding to declareControllers or later stages.
func TestNewApp_ModuleInitHookError_AbortsNewApp(t *testing.T) {
	wantErr := errors.New("init boom")

	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *wiringFailingInitType { return &wiringFailingInitType{} })
		p.OnModuleInit(func(*wiringFailingInitType) error {
			return wantErr
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	_, err := NewApp[recordingFakeAdapter](root, Options{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewApp() error = %v, want %v", err, wantErr)
	}
}

type wiringFailingBootstrapType struct{}

// TestNewApp_ApplicationBootstrapHookError_AbortsNewApp mirrors
// TestNewApp_ModuleInitHookError_AbortsNewApp for OnApplicationBootstrap.
func TestNewApp_ApplicationBootstrapHookError_AbortsNewApp(t *testing.T) {
	wantErr := errors.New("bootstrap boom")

	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *wiringFailingBootstrapType { return &wiringFailingBootstrapType{} })
		p.OnApplicationBootstrap(func(*wiringFailingBootstrapType) error {
			return wantErr
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	_, err := NewApp[recordingFakeAdapter](root, Options{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewApp() error = %v, want %v", err, wantErr)
	}
}

// controllableListenAdapter is a fake HttpAdapter whose Listen behavior is
// fully controlled by the test: it blocks until unblock is closed, then
// returns listenErr. Used to exercise App.Listen's post-adapter-Listen
// branching (shutdownHooksEnabled true/false, adapter error vs nil) without
// any real network I/O.
type controllableListenAdapter struct {
	unblock   chan struct{}
	listenErr error
}

func (f *controllableListenAdapter) Init(opts Options) {}
func (f *controllableListenAdapter) RegisterRoute(method route.HttpMethod, path string, h func(req *execution.Request, res *execution.Response)) error {
	return nil
}
func (f *controllableListenAdapter) Listen(addr string, onListen func()) error {
	if onListen != nil {
		onListen()
	}
	<-f.unblock
	return f.listenErr
}
func (f *controllableListenAdapter) Test(req *http.Request) (*http.Response, error) {
	return nil, nil
}
func (f *controllableListenAdapter) Shutdown(ctx context.Context) error {
	return nil
}

// TestListen_ShutdownHooksDisabled_ReturnsAsSoonAsAdapterListenReturns
// proves the opt-in-not-taken path: with shutdownHooksEnabled left false
// (NewApp's default), Listen returns immediately once adapter.Listen
// returns nil -- no new blocking on shutdownDone.
func TestListen_ShutdownHooksDisabled_ReturnsAsSoonAsAdapterListenReturns(t *testing.T) {
	adapter := &controllableListenAdapter{unblock: make(chan struct{})}
	a := &App{adapter: adapter, shutdownDone: make(chan struct{})}
	close(adapter.unblock) // adapter.Listen returns immediately

	done := make(chan error, 1)
	go func() { done <- a.Listen(":0") }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Listen() error = %v, want nil", err)
		}
	case <-time.After(wiringTestTimeout):
		t.Fatal("Listen() did not return -- shutdownHooksEnabled is false, it must not block")
	}
}

// TestListen_ShutdownHooksEnabled_BlocksUntilShutdownDoneThenReturnsShutdownErr
// proves the opt-in-taken path: with shutdownHooksEnabled true, Listen
// blocks past adapter.Listen's own nil return until a.shutdownDone closes,
// then returns a.shutdownErr -- driven here via a.Close(ctx), channel-
// synchronized (no time.Sleep), with a bounded timeout only as a deadlock
// guard.
func TestListen_ShutdownHooksEnabled_BlocksUntilShutdownDoneThenReturnsShutdownErr(t *testing.T) {
	listenAdapter := &controllableListenAdapter{unblock: make(chan struct{})}
	a := &App{adapter: listenAdapter, shutdownDone: make(chan struct{})}
	a.EnableShutdownHooks()

	listenDone := make(chan error, 1)
	go func() { listenDone <- a.Listen(":0") }()

	// Listen must NOT have returned yet -- adapter.Listen is still blocked on
	// unblock, and even once that closes, shutdownHooksEnabled means Listen
	// should keep waiting on shutdownDone.
	select {
	case err := <-listenDone:
		t.Fatalf("Listen() returned early (err=%v) before adapter.Listen was even unblocked", err)
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	close(listenAdapter.unblock) // adapter.Listen now returns nil

	// Listen still must not return yet -- it should now be blocked on
	// a.shutdownDone, not on the adapter.
	select {
	case err := <-listenDone:
		t.Fatalf("Listen() returned (err=%v) before shutdownDone closed", err)
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-listenDone:
		if err != nil {
			t.Fatalf("Listen() error = %v, want nil (a.shutdownErr after a clean Close)", err)
		}
	case <-time.After(wiringTestTimeout):
		t.Fatal("Listen() did not return after Close() closed shutdownDone")
	}
}

// TestListen_AdapterListenError_ReturnsImmediatelyWithoutWaitingOnShutdownDone
// proves a non-nil error from adapter.Listen itself (e.g. addr already in
// use) is returned immediately, even with shutdownHooksEnabled true -- Listen
// must not wait on a shutdownDone that will never close (nothing was ever
// shut down).
func TestListen_AdapterListenError_ReturnsImmediatelyWithoutWaitingOnShutdownDone(t *testing.T) {
	wantErr := errors.New("address already in use")
	adapter := &controllableListenAdapter{unblock: make(chan struct{}), listenErr: wantErr}
	a := &App{adapter: adapter, shutdownDone: make(chan struct{})}
	a.shutdownHooksEnabled = true // opted in, but must not matter for an adapter error
	close(adapter.unblock)        // adapter.Listen returns wantErr immediately

	done := make(chan error, 1)
	go func() { done <- a.Listen(":0") }()

	select {
	case err := <-done:
		if !errors.Is(err, wantErr) {
			t.Fatalf("Listen() error = %v, want %v", err, wantErr)
		}
	case <-time.After(wiringTestTimeout):
		t.Fatal("Listen() did not return -- a non-nil adapter.Listen error must not wait on shutdownDone")
	}
}
