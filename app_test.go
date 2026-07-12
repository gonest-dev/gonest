package gonest

import (
	"errors"
	"strings"
	"testing"
)

// UserProperties/UserEntity/UserService/UserProvider/UserModule/AppModule
// below adapt the "exemplo mais simples" from INSIGHT.md into a real Go
// test -- stripped of anything HTTP/Controller-related (no Fiber, no
// routes), keeping just the DI shape: a UserService struct + UserProvider
// (a Provider with a Constructor), wired into a Module, resolved via NewApp
// + MustResolve[*UserService].

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

var UserProvider = NewProvider(func(provider *Provider) {
	provider.Scope(ScopeSingleton)
	provider.Constructor(func() *UserService {
		return &UserService{index: 0, list: make([]*UserEntity, 0)}
	})
})

var exampleUserModule = NewModule(func(module *Module) {
	module.Providers(UserProvider)
	module.Exports(UserProvider)
})

func TestNewApp_UserProviderExample_ResolvesUsableUserService(t *testing.T) {
	appModule := NewModule(func(module *Module) {
		module.Imports(exampleUserModule)
	})

	var userService *UserService
	consumer := NewProvider(func(provider *Provider) {
		userService = MustResolve[*UserService](provider)
		provider.Constructor(func() *consumerMarker {
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
		t.Fatalf("MustResolve[*UserService] placeholder is nil after NewApp returned")
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
	badProvider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *undeclaredService {
			return &undeclaredService{}
		})
	})

	root := NewModule(func(module *Module) {
		// Exports a provider it never declared via Providers -- Stage 1
		// validation error (design.md's Error Handling Strategy table).
		module.Exports(badProvider)
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
	p := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *simpleService {
			return &simpleService{Ready: true}
		})
	})
	root := NewModule(func(module *Module) {
		module.Providers(p)
	})

	app := MustNewApp(root)
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

func TestNewApp_CircularDependency_ReturnsError(t *testing.T) {
	type cycleA struct{}
	type cycleB struct{}

	var pa, pb *Provider
	pa = NewProvider(func(provider *Provider) {
		MustResolve[*cycleB](pa)
		provider.Constructor(func() *cycleA { return &cycleA{} })
	})
	pb = NewProvider(func(provider *Provider) {
		MustResolve[*cycleA](pb)
		provider.Constructor(func() *cycleB { return &cycleB{} })
	})

	root := NewModule(func(module *Module) {
		module.Providers(pa, pb)
	})

	_, err := NewApp(root)
	if err == nil {
		t.Fatalf("NewApp() error = nil, want circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("NewApp() error = %q, want it to mention 'circular dependency'", err.Error())
	}
}

func TestNewApp_ConstructorError_ReturnsError(t *testing.T) {
	type failingService struct{}
	p := NewProvider(func(provider *Provider) {
		provider.Constructor(func() (*failingService, error) {
			return nil, errors.New("boom")
		})
	})
	root := NewModule(func(module *Module) {
		module.Providers(p)
	})

	_, err := NewApp(root)
	if err == nil {
		t.Fatalf("NewApp() error = nil, want the Constructor's returned error surfaced")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("NewApp() error = %q, want it to contain the Constructor's error message %q", err.Error(), "boom")
	}
}
