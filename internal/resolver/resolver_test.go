package resolver

import (
	"reflect"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
)

// fakeProvider is a minimal stand-in for the real *provider.Provider type,
// mirroring internal/module's own fakeProvider (this package cannot import
// that unexported test helper, so it defines its own).
type fakeProvider struct {
	resolved    reflect.Type
	ownerModule *module.Module
}

func (*fakeProvider) IsProvider() {}

func (p *fakeProvider) ResolvedType() reflect.Type {
	return p.resolved
}

func (p *fakeProvider) SetOwnerModule(m *module.Module) {
	p.ownerModule = m
}

type fooService struct{}
type barService struct{}
type bazService struct{}

func fooType() reflect.Type { return reflect.TypeOf(&fooService{}) }
func barType() reflect.Type { return reflect.TypeOf(&barService{}) }
func bazType() reflect.Type { return reflect.TypeOf(&bazService{}) }

func TestFind_OwnModuleProvider_ResolvesDirectly(t *testing.T) {
	p := &fakeProvider{resolved: fooType()}
	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	m.Assemble()

	got := Find(m, fooType())
	if got != module.ProviderRef(p) {
		t.Fatalf("Find() = %v, want %v", got, p)
	}
}

func TestFind_OwnModuleHasPriorityOverImports(t *testing.T) {
	ownP := &fakeProvider{resolved: fooType()}
	importedP := &fakeProvider{resolved: fooType()}

	child := module.New(func(m *module.Module) {
		m.Providers(importedP)
		m.Exports(importedP)
	})
	root := module.New(func(m *module.Module) {
		m.Imports(child)
		m.Providers(ownP)
	})
	root.Assemble()

	got := Find(root, fooType())
	if got != module.ProviderRef(ownP) {
		t.Fatalf("Find() = %v, want own module's provider %v (priority over imports)", got, ownP)
	}
}

func TestFind_ImportedExportedProvider_ResolvesCorrectly(t *testing.T) {
	p := &fakeProvider{resolved: barType()}
	child := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Exports(p)
	})
	root := module.New(func(m *module.Module) {
		m.Imports(child)
	})
	root.Assemble()

	got := Find(root, barType())
	if got != module.ProviderRef(p) {
		t.Fatalf("Find() = %v, want imported+exported provider %v", got, p)
	}
}

func TestFind_ImportedButNotExported_Panics(t *testing.T) {
	p := &fakeProvider{resolved: bazType()}
	child := module.New(func(m *module.Module) {
		m.Providers(p)
		// not exported
	})
	root := module.New(func(m *module.Module) {
		m.Imports(child)
	})
	root.Assemble()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Find() did not panic for type in imported module but not exported")
		}
		msg, ok := r.(string)
		want := "gonest: type *resolver.bazService exists in module " + ModuleName(child) + " but is not exported"
		if !ok || msg != want {
			t.Fatalf("panic value = %v, want %q", r, want)
		}
	}()

	Find(root, bazType())
}

func TestFind_TypeNowhere_Panics(t *testing.T) {
	root := module.New(func(m *module.Module) {})
	root.Assemble()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Find() did not panic for type registered nowhere")
		}
		msg, ok := r.(string)
		want := "gonest: no provider registered for type *resolver.fooService"
		if !ok || msg != want {
			t.Fatalf("panic value = %v, want %q", r, want)
		}
	}()

	Find(root, fooType())
}

// TestFind_DiamondImport_ResolvesDirectImportOnly confirms resolution scope
// is exactly "own module -> directly imported modules' exports", not a
// transitive search across the whole import tree (context.md: "Não busca a
// partir da raiz (AppModule) pra baixo"). D's provider is shared by two
// modules (B and C) that both import it -- a diamond -- but A only
// directly imports B and C, not D, so a type only exported by D is not
// visible to A even though D is reachable by BFS during Stage 1 assembly.
func TestFind_DiamondImport_ResolvesDirectImportOnly(t *testing.T) {
	p := &fakeProvider{resolved: fooType()}
	d := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Exports(p)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(d)
	})
	c := module.New(func(m *module.Module) {
		m.Imports(d)
	})
	a := module.New(func(m *module.Module) {
		m.Imports(b, c)
	})
	a.Assemble()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Find() did not panic: type exported by D (not a direct import of A) should not be visible to A")
		}
		msg, ok := r.(string)
		want := "gonest: no provider registered for type *resolver.fooService"
		if !ok || msg != want {
			t.Fatalf("panic value = %v, want %q", r, want)
		}
	}()

	Find(a, fooType())
}

// TestFind_DiamondImport_DirectImporterResolvesSharedProvider confirms
// that B, which directly imports D, resolves D's exported provider -- the
// counterpart to the negative case above, proving the diamond shape itself
// (D visited once during Stage 1 assembly, see internal/module's own
// diamond test) does not interfere with per-module scoped resolution.
func TestFind_DiamondImport_DirectImporterResolvesSharedProvider(t *testing.T) {
	p := &fakeProvider{resolved: fooType()}
	d := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Exports(p)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(d)
	})
	c := module.New(func(m *module.Module) {
		m.Imports(d)
	})
	a := module.New(func(m *module.Module) {
		m.Imports(b, c)
	})
	a.Assemble()

	got := Find(b, fooType())
	if got != module.ProviderRef(p) {
		t.Fatalf("Find() = %v, want %v (B directly imports D)", got, p)
	}
}
