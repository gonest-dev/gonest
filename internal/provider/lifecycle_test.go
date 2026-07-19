package provider

import (
	"context"
	"errors"
	"testing"

	"gonest.dev/gonest/internal/scope"
)

type lifecycleWidget struct{ name string }

// registerFuncs is a table-driven helper covering the 3 hooks' identical
// registration surface: a *Provider method taking `any` and panicking on
// invalid shapes.
func registerFuncs(p *Provider) map[string]func(any) {
	return map[string]func(any){
		"OnModuleInit":           p.OnModuleInit,
		"OnApplicationBootstrap": p.OnApplicationBootstrap,
		"OnModuleDestroy":        p.OnModuleDestroy,
	}
}

func TestHookRegistration_ValidShapes(t *testing.T) {
	shapes := map[string]any{
		"func(T)":                        func(*lifecycleWidget) {},
		"func(T) error":                  func(*lifecycleWidget) error { return nil },
		"func(T, context.Context)":       func(*lifecycleWidget, context.Context) {},
		"func(T, context.Context) error": func(*lifecycleWidget, context.Context) error { return nil },
	}

	for hookName := range registerFuncs(New(nil)) {
		for shapeName, fn := range shapes {
			t.Run(hookName+"/"+shapeName, func(t *testing.T) {
				p := New(nil)
				reg := registerFuncs(p)[hookName]
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("unexpected panic registering valid shape: %v", r)
					}
				}()
				reg(fn)
			})
		}
	}
}

func TestHookRegistration_InvalidShapes(t *testing.T) {
	invalid := map[string]any{
		"no args":                     func() {},
		"wrong second arg type":       func(*lifecycleWidget, string) {},
		"too many args":               func(*lifecycleWidget, context.Context, int) {},
		"two returns, second not err": func(*lifecycleWidget) (int, int) { return 0, 0 },
		"not a func":                  42,
	}

	wantMsg := map[string]string{
		"OnModuleInit":           "gonest: invalid OnModuleInit signature",
		"OnApplicationBootstrap": "gonest: invalid OnApplicationBootstrap signature",
		"OnModuleDestroy":        "gonest: invalid OnModuleDestroy signature",
	}

	for hookName, want := range wantMsg {
		for shapeName, fn := range invalid {
			t.Run(hookName+"/"+shapeName, func(t *testing.T) {
				p := New(nil)
				reg := registerFuncs(p)[hookName]
				defer func() {
					r := recover()
					if r == nil {
						t.Fatalf("expected panic, got none")
					}
					if r != want {
						t.Fatalf("panic message = %v, want %v", r, want)
					}
				}()
				reg(fn)
			})
		}
	}
}

func newResolvedProvider(t *testing.T, s scope.Scope) (*Provider, *lifecycleWidget) {
	t.Helper()
	p := New(nil)
	p.Scope(s)
	p.Constructor(func() *lifecycleWidget { return &lifecycleWidget{name: "w"} })
	v, err := callAndStore(p)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	return p, v
}

// callAndStore mimics what internal/resolver's Stage 3 does: call the
// Constructor and store the result via SetResolvedValue.
func callAndStore(p *Provider) (*lifecycleWidget, error) {
	fn := p.ConstructorFunc()
	out := fn.Call(nil)
	v := out[0]
	p.SetResolvedValue(v)
	return v.Interface().(*lifecycleWidget), nil
}

func TestRunModuleInit_InvokesWithResolvedValue(t *testing.T) {
	p, want := newResolvedProvider(t, scope.Singleton)

	var got *lifecycleWidget
	calls := 0
	p.OnModuleInit(func(w *lifecycleWidget) {
		calls++
		got = w
	})

	if err := p.RunModuleInit(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("hook called %d times, want 1", calls)
	}
	if got != want {
		t.Fatalf("hook received %v, want %v", got, want)
	}
}

func TestRunApplicationBootstrap_WithContextArg(t *testing.T) {
	p, want := newResolvedProvider(t, scope.Singleton)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "value")

	var gotCtx context.Context
	var gotW *lifecycleWidget
	p.OnApplicationBootstrap(func(w *lifecycleWidget, c context.Context) error {
		gotW = w
		gotCtx = c
		return nil
	})

	if err := p.RunApplicationBootstrap(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotW != want {
		t.Fatalf("hook received %v, want %v", gotW, want)
	}
	if gotCtx != ctx {
		t.Fatalf("hook received wrong context")
	}
}

func TestRunModuleDestroy_ErrorPropagated(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	wantErr := errors.New("boom")
	p.OnModuleDestroy(func(w *lifecycleWidget) error {
		return wantErr
	})

	err := p.RunModuleDestroy(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRunModuleInit_PanicRecovered(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	p.OnModuleInit(func(w *lifecycleWidget) {
		panic("kaboom")
	})

	err := p.RunModuleInit(context.Background())
	if err == nil {
		t.Fatalf("expected error from recovered panic, got nil")
	}
	wantSubstr := "gonest: provider for type *provider.lifecycleWidget panicked during OnModuleInit: kaboom"
	if err.Error() != wantSubstr {
		t.Fatalf("error = %q, want %q", err.Error(), wantSubstr)
	}
}

func TestRunXxx_NoopWhenNeverRegistered(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	if err := p.RunModuleInit(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.RunApplicationBootstrap(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.RunModuleDestroy(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunXxx_NoopForNonSingletonScope(t *testing.T) {
	for _, s := range []scope.Scope{scope.Transient, scope.Request} {
		p, _ := newResolvedProvider(t, s)

		calls := 0
		p.OnModuleInit(func(w *lifecycleWidget) { calls++ })
		p.OnApplicationBootstrap(func(w *lifecycleWidget) { calls++ })
		p.OnModuleDestroy(func(w *lifecycleWidget) { calls++ })

		if err := p.RunModuleInit(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := p.RunApplicationBootstrap(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := p.RunModuleDestroy(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 0 {
			t.Fatalf("scope %v: hook(s) invoked %d times, want 0", s, calls)
		}
	}
}

func TestRunModuleInit_NoopWhenResolvedValueNotSet(t *testing.T) {
	p := New(nil)
	p.Constructor(func() *lifecycleWidget { return &lifecycleWidget{} })

	calls := 0
	p.OnModuleInit(func(w *lifecycleWidget) { calls++ })

	if err := p.RunModuleInit(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("hook invoked %d times, want 0 (no resolved value set)", calls)
	}
}

// registerSignalFuncs is registerFuncs' counterpart for the 2
// signal-carrying hooks added in T2.
func registerSignalFuncs(p *Provider) map[string]func(any) {
	return map[string]func(any){
		"BeforeApplicationShutdown": p.BeforeApplicationShutdown,
		"OnApplicationShutdown":     p.OnApplicationShutdown,
	}
}

func TestSignalHookRegistration_ValidShapes(t *testing.T) {
	shapes := map[string]any{
		"func(T, string)":                        func(*lifecycleWidget, string) {},
		"func(T, string) error":                  func(*lifecycleWidget, string) error { return nil },
		"func(T, context.Context, string)":       func(*lifecycleWidget, context.Context, string) {},
		"func(T, context.Context, string) error": func(*lifecycleWidget, context.Context, string) error { return nil },
	}

	for hookName := range registerSignalFuncs(New(nil)) {
		for shapeName, fn := range shapes {
			t.Run(hookName+"/"+shapeName, func(t *testing.T) {
				p := New(nil)
				reg := registerSignalFuncs(p)[hookName]
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("unexpected panic registering valid shape: %v", r)
					}
				}()
				reg(fn)
			})
		}
	}
}

func TestSignalHookRegistration_InvalidShapes(t *testing.T) {
	invalid := map[string]any{
		"no args":                     func() {},
		"missing trailing string":     func(*lifecycleWidget) {},
		"missing trailing string+ctx": func(*lifecycleWidget, context.Context) {},
		"wrong second arg type":       func(*lifecycleWidget, int) {},
		"wrong trailing arg type":     func(*lifecycleWidget, context.Context, int) {},
		"too many args":               func(*lifecycleWidget, context.Context, string, int) {},
		"two returns, second not err": func(*lifecycleWidget, string) (int, int) { return 0, 0 },
		"not a func":                  42,
	}

	wantMsg := map[string]string{
		"BeforeApplicationShutdown": "gonest: invalid BeforeApplicationShutdown signature",
		"OnApplicationShutdown":     "gonest: invalid OnApplicationShutdown signature",
	}

	for hookName, want := range wantMsg {
		for shapeName, fn := range invalid {
			t.Run(hookName+"/"+shapeName, func(t *testing.T) {
				p := New(nil)
				reg := registerSignalFuncs(p)[hookName]
				defer func() {
					r := recover()
					if r == nil {
						t.Fatalf("expected panic, got none")
					}
					if r != want {
						t.Fatalf("panic message = %v, want %v", r, want)
					}
				}()
				reg(fn)
			})
		}
	}
}

func TestRunBeforeApplicationShutdown_InvokesWithSignalNoContext(t *testing.T) {
	p, want := newResolvedProvider(t, scope.Singleton)

	var gotW *lifecycleWidget
	var gotSignal string
	calls := 0
	p.BeforeApplicationShutdown(func(w *lifecycleWidget, signal string) {
		calls++
		gotW = w
		gotSignal = signal
	})

	if err := p.RunBeforeApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("hook called %d times, want 1", calls)
	}
	if gotW != want {
		t.Fatalf("hook received %v, want %v", gotW, want)
	}
	if gotSignal != "SIGTERM" {
		t.Fatalf("hook received signal %q, want %q", gotSignal, "SIGTERM")
	}
}

func TestRunApplicationShutdown_WithContextAndSignal(t *testing.T) {
	p, want := newResolvedProvider(t, scope.Singleton)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "value")

	var gotCtx context.Context
	var gotW *lifecycleWidget
	var gotSignal string
	p.OnApplicationShutdown(func(w *lifecycleWidget, c context.Context, signal string) error {
		gotW = w
		gotCtx = c
		gotSignal = signal
		return nil
	})

	if err := p.RunApplicationShutdown(ctx, "SIGINT"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotW != want {
		t.Fatalf("hook received %v, want %v", gotW, want)
	}
	if gotCtx != ctx {
		t.Fatalf("hook received wrong context")
	}
	if gotSignal != "SIGINT" {
		t.Fatalf("hook received signal %q, want %q", gotSignal, "SIGINT")
	}
}

func TestRunApplicationShutdown_ErrorPropagated(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	wantErr := errors.New("boom")
	p.OnApplicationShutdown(func(w *lifecycleWidget, signal string) error {
		return wantErr
	})

	err := p.RunApplicationShutdown(context.Background(), "SIGTERM")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRunBeforeApplicationShutdown_PanicRecovered(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	p.BeforeApplicationShutdown(func(w *lifecycleWidget, signal string) {
		panic("kaboom")
	})

	err := p.RunBeforeApplicationShutdown(context.Background(), "SIGTERM")
	if err == nil {
		t.Fatalf("expected error from recovered panic, got nil")
	}
	wantSubstr := "gonest: provider for type *provider.lifecycleWidget panicked during BeforeApplicationShutdown: kaboom"
	if err.Error() != wantSubstr {
		t.Fatalf("error = %q, want %q", err.Error(), wantSubstr)
	}
}

func TestRunSignalXxx_NoopWhenNeverRegistered(t *testing.T) {
	p, _ := newResolvedProvider(t, scope.Singleton)

	if err := p.RunBeforeApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.RunApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSignalXxx_NoopForNonSingletonScope(t *testing.T) {
	for _, s := range []scope.Scope{scope.Transient, scope.Request} {
		p, _ := newResolvedProvider(t, s)

		calls := 0
		p.BeforeApplicationShutdown(func(w *lifecycleWidget, signal string) { calls++ })
		p.OnApplicationShutdown(func(w *lifecycleWidget, signal string) { calls++ })

		if err := p.RunBeforeApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := p.RunApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 0 {
			t.Fatalf("scope %v: hook(s) invoked %d times, want 0", s, calls)
		}
	}
}

func TestRunSignalXxx_NoopWhenResolvedValueNotSet(t *testing.T) {
	p := New(nil)
	p.Constructor(func() *lifecycleWidget { return &lifecycleWidget{} })

	calls := 0
	p.BeforeApplicationShutdown(func(w *lifecycleWidget, signal string) { calls++ })

	if err := p.RunBeforeApplicationShutdown(context.Background(), "SIGTERM"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("hook invoked %d times, want 0 (no resolved value set)", calls)
	}
}
