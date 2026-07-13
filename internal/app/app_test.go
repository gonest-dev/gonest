package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/provider"
	"github.com/gonest-dev/gonest/internal/scope"
)

// UserProperties/UserEntity/UserService/UserProvider/UserModule/AppModule
// below adapt the "exemplo mais simples" from INSIGHT.md into a real Go
// test -- stripped of anything HTTP/Controller-related (no Fiber, no
// routes), keeping just the DI shape: a UserService struct + UserProvider
// (a Provider with a Constructor), wired into a Module, resolved via NewApp
// + MustInject[*UserService].

type UserEntity struct {
	ID   int64
	Name string
}

type UserService struct {
	index int
	list  []*UserEntity
}

func (t *UserService) List() []*UserEntity { return t.list }

func (t *UserService) Create(name string) *UserEntity {
	t.index++
	u := &UserEntity{ID: int64(t.index), Name: name}
	t.list = append(t.list, u)
	return u
}

var UserProvider = provider.New(func(p *provider.Provider) {
	p.Scope(scope.Singleton)
	p.Constructor(func() *UserService {
		return &UserService{index: 0, list: make([]*UserEntity, 0)}
	})
})

var exampleUserModule = module.New(func(m *module.Module) {
	m.Providers(UserProvider)
	m.Exports(UserProvider)
})

func TestNewApp_UserProviderExample_ResolvesUsableUserService(t *testing.T) {
	appModule := module.New(func(m *module.Module) {
		m.Imports(exampleUserModule)
	})

	var userService *UserService
	consumer := provider.New(func(p *provider.Provider) {
		userService = inject.MustInject[*UserService](p)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	appModule.Providers(consumer)

	app, err := NewApp(appModule)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	if userService == nil {
		t.Fatalf("MustInject[*UserService] placeholder is nil after NewApp returned")
	}

	// Prove it's genuinely usable, not a zero-value placeholder: the
	// Constructor initialized list to a non-nil empty slice, and Create
	// mutates real state.
	if got := userService.List(); got == nil {
		t.Fatalf("userService.List() = nil, want non-nil empty slice from real Constructor initialization")
	}

	created := userService.Create("Ada")
	if created.ID != 1 || created.Name != "Ada" {
		t.Fatalf("userService.Create(%q) = %+v, want ID=1 Name=Ada", "Ada", created)
	}
	if len(userService.List()) != 1 {
		t.Fatalf("userService.List() len = %d after Create, want 1 -- resolved instance is not the real, usable one", len(userService.List()))
	}
}

type consumerMarker struct{}

func TestMustNewApp_PanicsOnAssembleError(t *testing.T) {
	type undeclaredService struct{}
	badProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *undeclaredService {
			return &undeclaredService{}
		})
	})

	root := module.New(func(m *module.Module) {
		// Exports a provider it never declared via Providers -- Stage 1
		// validation error (design.md's Error Handling Strategy table).
		m.Exports(badProvider)
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustNewApp() did not panic for a Stage 1 assemble error")
		}
	}()

	MustNewApp(root)
}

func TestMustNewApp_ReturnsAppOnSuccess(t *testing.T) {
	type simpleService struct{ Ready bool }
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *simpleService {
			return &simpleService{Ready: true}
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	app := MustNewApp(root)
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

func TestNewApp_CircularDependency_ReturnsError(t *testing.T) {
	type cycleA struct{}
	type cycleB struct{}

	var pa, pb *provider.Provider
	pa = provider.New(func(p *provider.Provider) {
		inject.MustInject[*cycleB](pa)
		p.Constructor(func() *cycleA { return &cycleA{} })
	})
	pb = provider.New(func(p *provider.Provider) {
		inject.MustInject[*cycleA](pb)
		p.Constructor(func() *cycleB { return &cycleB{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(pa, pb)
	})

	_, err := NewApp(root)
	if err == nil {
		t.Fatalf("NewApp() error = nil, want circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("NewApp() error = %q, want it to mention 'circular dependency'", err.Error())
	}
}

// TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges proves that
// calling NewApp twice in the same process, with two completely unrelated
// module trees, does not let the first call's MustInject bookkeeping leak
// into the second call's cycle detection or placeholder resolution.
// internal/inject's pendingEdges slice is process-global; without resetting
// it at the start of each NewApp call, the second call's Stage 3 would
// observe stale pending edges from the first call forever (unbounded growth
// across a long-running process, and a correctness risk for
// placeholdersFor's unscoped inject.PendingEdges() read).
func TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges(t *testing.T) {
	type firstTreeService struct{ Value string }
	firstProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *firstTreeService {
			return &firstTreeService{Value: "first"}
		})
	})
	firstRoot := module.New(func(m *module.Module) {
		m.Providers(firstProvider)
	})

	firstApp, err := NewApp(firstRoot)
	if err != nil {
		t.Fatalf("first NewApp() error = %v", err)
	}
	if firstApp == nil {
		t.Fatalf("first NewApp() returned nil *App")
	}

	// After the first call resolves, no pending edges should remain in
	// internal/inject's global bookkeeping -- NewApp must reset the log at
	// the start of ITS OWN call, not leave the previous call's edges parked
	// forever.
	if got := len(inject.PendingEdges()); got != 0 {
		t.Fatalf("inject.PendingEdges() len = %d after first NewApp() returned, want 0 -- pending edges must not accumulate across NewApp calls", got)
	}

	type secondTreeService struct{ Value string }
	var resolved *secondTreeService
	secondProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *secondTreeService {
			return &secondTreeService{Value: "second"}
		})
	})
	var consumer *provider.Provider
	consumer = provider.New(func(p *provider.Provider) {
		resolved = inject.MustInject[*secondTreeService](consumer)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	secondRoot := module.New(func(m *module.Module) {
		m.Providers(secondProvider, consumer)
	})

	secondApp, err := NewApp(secondRoot)
	if err != nil {
		t.Fatalf("second NewApp() error = %v, want nil -- must not be affected by the first call's leftover state", err)
	}
	if secondApp == nil {
		t.Fatalf("second NewApp() returned nil *App")
	}

	if resolved == nil {
		t.Fatalf("second tree's MustInject[*secondTreeService] placeholder is nil after second NewApp() returned")
	}
	if resolved.Value != "second" {
		t.Fatalf("resolved.Value = %q, want %q -- second call's placeholder must be filled from the SECOND tree's provider, not confused with the first tree's leftover pending edges", resolved.Value, "second")
	}
}

func TestNewApp_ConstructorError_ReturnsError(t *testing.T) {
	type failingService struct{}
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() (*failingService, error) {
			return nil, errors.New("boom")
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	_, err := NewApp(root)
	if err == nil {
		t.Fatalf("NewApp() error = nil, want the Constructor's returned error surfaced")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("NewApp() error = %q, want it to contain the Constructor's error message %q", err.Error(), "boom")
	}
}
