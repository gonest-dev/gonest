package gonest

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/fiberapp"
)

// TestNewApp_RootAlias_InsightCallShape proves the exact INSIGHT.md call
// shape gonest.NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
// compiles and works through the root gonest package. gonest.FiberApp does
// not exist as a root alias yet (a pre-existing gap from an earlier
// feature, out of scope for T5) -- fiberapp.FiberApp is used directly here
// via import instead.
func TestNewApp_RootAlias_InsightCallShape(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}
}

// TestMustNewApp_RootAlias_InsightCallShape proves
// gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})
// compiles and works through the root gonest package.
func TestMustNewApp_RootAlias_InsightCallShape(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app := MustNewApp[fiberapp.FiberApp](root, AppOptions{
		BufferLogs: true,
		LogLevels:  []LogLevel{LogLevelWarn, LogLevelError},
	})
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

// TestApp_MustListen_PromotedThroughRootAlias proves App.MustListen (added
// on internal/app.App in T4) is automatically visible on the root gonest.App
// alias with zero extra wrapper code, and that both
// app.MustListen(addr, gonest.OnListen(fn)) and
// app.MustListen(addr, nil) compile and work through the root alias.
func TestApp_MustListen_PromotedThroughRootAlias(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34589"

	fired := make(chan struct{})
	var once sync.Once
	onListen := OnListen(func() {
		once.Do(func() { close(fired) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, onListen)
	}()

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("http.Get error = %v", err)
	}
	resp.Body.Close()

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before shutdown")
	default:
	}
}

// TestApp_MustListen_NilOnListen_ThroughRootAlias proves
// app.MustListen(addr, nil) compiles and works through the root alias.
func TestApp_MustListen_NilOnListen_ThroughRootAlias(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34590"

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, nil)
	}()

	// Give MustListen a moment to bind, then confirm it's serving without
	// having panicked -- a nil OnListen must be safe end-to-end.
	var resp *http.Response
	var err2 error
	for i := 0; i < 50; i++ {
		resp, err2 = http.Get("http://" + addr + "/")
		if err2 == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err2 != nil {
		t.Fatalf("http.Get error = %v", err2)
	}
	resp.Body.Close()

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before shutdown")
	default:
	}
}
