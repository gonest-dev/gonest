package module

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fakeMiddleware/fakeFilter are minimal stand-ins for the real
// *middleware.Middleware/*filter.Filter types, mirroring fakeProvider/
// fakeController below -- this package cannot import internal/middleware or
// internal/filter directly (they import internal/module for
// Declare(scope []*Module), so importing them back here would cycle). They
// only need to satisfy MiddlewareRef/FilterRef.
type fakeMiddleware struct{}

func (*fakeMiddleware) IsMiddleware() {}
func (*fakeMiddleware) IsToken()      {}

type fakeFilter struct{}

func (*fakeFilter) IsFilter() {}
func (*fakeFilter) IsToken()  {}

// fakeProvider is a minimal stand-in for the real *provider.Provider type.
// It only needs to satisfy ProviderRef.
type fakeProvider struct {
	name        string
	resolved    reflect.Type
	ownerModule *Module
}

func (*fakeProvider) IsProvider() {}

func (*fakeProvider) IsToken() {}

func (p *fakeProvider) ResolvedType() reflect.Type {
	return p.resolved
}

func (p *fakeProvider) SetOwnerModule(m *Module) {
	p.ownerModule = m
}

// fakeController is a minimal stand-in for the real *controller.Controller
// type. It only needs to satisfy ControllerRef.
type fakeController struct {
	ownerModule *Module
}

func (*fakeController) IsController() {}
func (*fakeController) IsToken()      {}

func (c *fakeController) SetOwnerModule(m *Module) {
	c.ownerModule = m
}

type fakeResolver struct {
	ownerModule *Module
}

func (*fakeResolver) IsResolver() {}
func (*fakeResolver) IsToken()    {}

func (r *fakeResolver) SetOwnerModule(m *Module) {
	r.ownerModule = m
}

// fakeListener is a minimal stand-in for the real *emitter.Listener[T]
// type. It only needs to satisfy ListenerRef.
type fakeListener struct {
	ownerModule *Module
}

func (*fakeListener) IsListener() {}
func (*fakeListener) IsToken()    {}

func (l *fakeListener) SetOwnerModule(m *Module) {
	l.ownerModule = m
}

// fakeScheduler is a minimal stand-in for the real *scheduler.Scheduler
// type. It only needs to satisfy SchedulerRef.
type fakeScheduler struct {
	ownerModule *Module
}

func (*fakeScheduler) IsScheduler() {}
func (*fakeScheduler) IsToken()     {}

func (s *fakeScheduler) SetOwnerModule(m *Module) {
	s.ownerModule = m
}

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	executed := false

	m := New(func(m *Module) {
		executed = true
	})

	if executed {
		t.Fatalf("New(fn) executed fn synchronously, want deferred until assemble runs")
	}
	if m == nil {
		t.Fatalf("New(fn) returned nil *Module")
	}
}

func TestNew_ExecutesFnOnlyAfterAssemble(t *testing.T) {
	executed := false

	m := New(func(m *Module) {
		executed = true
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	if !executed {
		t.Fatalf("assemble(m) did not execute fn, want fn executed exactly once")
	}
}

func TestAssemble_SimpleBFS_VisitsImportedModule(t *testing.T) {
	child := New(func(m *Module) {})
	root := New(func(m *Module) {
		m.Imports(child)
	})

	visited, err := assemble(root)
	if err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	if len(visited) != 2 {
		t.Fatalf("assemble visited %d modules, want 2 (root + child)", len(visited))
	}
}

func TestAssemble_DiamondImport_VisitsSharedModuleOnce(t *testing.T) {
	visitCount := 0
	d := New(func(m *Module) {
		visitCount++
	})
	b := New(func(m *Module) {
		m.Imports(d)
	})
	c := New(func(m *Module) {
		m.Imports(d)
	})
	a := New(func(m *Module) {
		m.Imports(b, c)
	})

	visited, err := assemble(a)
	if err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	if visitCount != 1 {
		t.Fatalf("shared module D's fn executed %d times, want exactly 1", visitCount)
	}
	if len(visited) != 4 {
		t.Fatalf("assemble visited %d modules, want 4 (A, B, C, D deduped)", len(visited))
	}
}

func TestAssemble_ExportNotDeclaredInProviders_ReturnsError(t *testing.T) {
	p := &fakeProvider{name: "Y"}
	m := New(func(m *Module) {
		m.Exports(p)
	})

	_, err := assemble(m)
	if err == nil {
		t.Fatalf("assemble returned nil error, want error for exporting undeclared provider")
	}
}

func TestAssemble_ExportDeclaredInProviders_NoError(t *testing.T) {
	p := &fakeProvider{name: "Y"}
	m := New(func(m *Module) {
		m.Providers(p)
		m.Exports(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}
}

func TestModule_Controllers_RegistersControllers(t *testing.T) {
	ctrl := &fakeController{}
	m := New(func(m *Module) {
		m.Controllers(ctrl)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}
}

type dummyType struct{}

func TestModule_OwnProviders_ReturnsRegisteredProviders(t *testing.T) {
	p := &fakeProvider{name: "X"}
	m := New(func(m *Module) {
		m.Providers(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnProviders()
	if len(got) != 1 || got[0] != ProviderRef(p) {
		t.Fatalf("OwnProviders() = %v, want [p]", got)
	}
}

func TestModule_OwnControllers_ReturnsRegisteredControllers(t *testing.T) {
	c := &fakeController{}
	m := New(func(m *Module) {
		m.Controllers(c)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnControllers()
	if len(got) != 1 || got[0] != ControllerRef(c) {
		t.Fatalf("OwnControllers() = %v, want [c]", got)
	}
}

func TestModule_OwnControllers_ReturnsCopyNotInternalSlice(t *testing.T) {
	c := &fakeController{}
	m := New(func(m *Module) {
		m.Controllers(c)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnControllers()
	got[0] = &fakeController{}

	got2 := m.OwnControllers()
	if got2[0] != ControllerRef(c) {
		t.Fatalf("OwnControllers() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestModule_Resolvers_RegistersResolvers(t *testing.T) {
	r := &fakeResolver{}
	m := New(func(m *Module) {
		m.Resolvers(r)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}
}

func TestModule_OwnResolvers_ReturnsRegisteredResolvers(t *testing.T) {
	r := &fakeResolver{}
	m := New(func(m *Module) {
		m.Resolvers(r)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnResolvers()
	if len(got) != 1 || got[0] != ResolverRef(r) {
		t.Fatalf("OwnResolvers() = %v, want [r]", got)
	}
}

func TestModule_OwnResolvers_ReturnsCopyNotInternalSlice(t *testing.T) {
	r := &fakeResolver{}
	m := New(func(m *Module) {
		m.Resolvers(r)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnResolvers()
	got[0] = &fakeResolver{}

	got2 := m.OwnResolvers()
	if got2[0] != ResolverRef(r) {
		t.Fatalf("OwnResolvers() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestModule_Imports_ReturnsImportedModules(t *testing.T) {
	child := New(func(m *Module) {})
	root := New(func(m *Module) {
		m.Imports(child)
	})

	if _, err := assemble(root); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := root.ImportedModules()
	if len(got) != 1 || got[0] != child {
		t.Fatalf("ImportedModules() = %v, want [child]", got)
	}
}

func TestModule_Exports_ReturnsExportedProviders(t *testing.T) {
	p := &fakeProvider{name: "X"}
	m := New(func(m *Module) {
		m.Providers(p)
		m.Exports(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.ExportedProviders()
	if len(got) != 1 || got[0] != ProviderRef(p) {
		t.Fatalf("ExportedProviders() = %v, want [p]", got)
	}
}

func TestModule_OwnProviders_ReturnsCopyNotInternalSlice(t *testing.T) {
	p := &fakeProvider{name: "X"}
	m := New(func(m *Module) {
		m.Providers(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnProviders()
	got[0] = &fakeProvider{name: "mutated"}

	got2 := m.OwnProviders()
	if got2[0] != ProviderRef(p) {
		t.Fatalf("OwnProviders() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestProviderRef_ResolvedType_ExposesUnderlyingType(t *testing.T) {
	p := &fakeProvider{name: "X", resolved: reflect.TypeOf(&dummyType{})}

	var ref ProviderRef = p
	if ref.ResolvedType() != reflect.TypeOf(&dummyType{}) {
		t.Fatalf("ResolvedType() = %v, want %v", ref.ResolvedType(), reflect.TypeOf(&dummyType{}))
	}
}

func TestAssemble_AutoWiresOwnerModuleOnProviders(t *testing.T) {
	p := &fakeProvider{name: "X"}
	m := New(func(m *Module) {
		m.Providers(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	if p.ownerModule != m {
		t.Fatalf("assemble did not auto-wire OwnerModule on provider: got %v, want %v", p.ownerModule, m)
	}
}

func TestModule_Name_MakesModuleNameReturnGivenName(t *testing.T) {
	m := New(func(m *Module) {})
	m.Name("Foo")

	if got := ModuleName(m); got != "Foo" {
		t.Fatalf("ModuleName(m) = %q, want %q", got, "Foo")
	}
}

func TestModule_Name_NeverCalled_FallsBackToPointerAddress(t *testing.T) {
	m := New(func(m *Module) {})

	want := fmt.Sprintf("%p", m)
	if got := ModuleName(m); got != want {
		t.Fatalf("ModuleName(m) = %q, want fallback %q", got, want)
	}
}

func TestModule_Name_EmptyString_FallsBackToPointerAddress(t *testing.T) {
	m := New(func(m *Module) {})
	m.Name("")

	want := fmt.Sprintf("%p", m)
	if got := ModuleName(m); got != want {
		t.Fatalf("ModuleName(m) = %q, want fallback %q", got, want)
	}
}

func TestModule_Name_CalledTwice_LastCallWins(t *testing.T) {
	m := New(func(m *Module) {})
	m.Name("First")
	m.Name("Second")

	if got := ModuleName(m); got != "Second" {
		t.Fatalf("ModuleName(m) = %q, want %q", got, "Second")
	}
}

func TestModule_Use_StoresMiddlewareInRegistrationOrder(t *testing.T) {
	first := &fakeMiddleware{}
	second := &fakeMiddleware{}
	m := New(func(m *Module) {
		m.Use(first, second)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnMiddleware()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("OwnMiddleware() = %v, want [first, second]", got)
	}
}

func TestModule_OwnMiddleware_ReturnsCopyNotInternalSlice(t *testing.T) {
	mw := &fakeMiddleware{}
	m := New(func(m *Module) {
		m.Use(mw)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnMiddleware()
	got[0] = &fakeMiddleware{}

	got2 := m.OwnMiddleware()
	if got2[0] != mw {
		t.Fatalf("OwnMiddleware() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestModule_Filters_StoresFiltersInRegistrationOrder(t *testing.T) {
	first := &fakeFilter{}
	second := &fakeFilter{}
	m := New(func(m *Module) {
		m.Filters(first, second)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnFilters()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("OwnFilters() = %v, want [first, second]", got)
	}
}

func TestModule_OwnFilters_ReturnsCopyNotInternalSlice(t *testing.T) {
	f := &fakeFilter{}
	m := New(func(m *Module) {
		m.Filters(f)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.OwnFilters()
	got[0] = &fakeFilter{}

	got2 := m.OwnFilters()
	if got2[0] != f {
		t.Fatalf("OwnFilters() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestAssemble_AutoWiresOwnerModuleOnControllers(t *testing.T) {
	c := &fakeController{}
	m := New(func(m *Module) {
		m.Controllers(c)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	if c.ownerModule != m {
		t.Fatalf("assemble did not auto-wire OwnerModule on controller: got %v, want %v", c.ownerModule, m)
	}
}

// --- TokenRef panic path: each of Module's 9 builder methods panics
// immediately when handed a TokenRef whose concrete kind it does not
// expect, naming both the method and the received type (unified-token-ref
// feature, TOKEN-04). Follows the same recover()+message-check pattern
// TestProviderAs_NonInterfaceT_Panics (provider_as_test.go) already uses.

func TestModule_Imports_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Imports to panic when given a non-*Module TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Imports") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Imports and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Imports(&fakeProvider{})
}

func TestModule_Providers_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Providers to panic when given a non-ProviderRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Providers") || !strings.Contains(msg, "*module.fakeController") {
			t.Fatalf("panic message = %q, want it to name Module.Providers and *module.fakeController", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Providers(&fakeController{})
}

func TestModule_Controllers_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Controllers to panic when given a non-ControllerRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Controllers") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Controllers and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Controllers(&fakeProvider{})
}

func TestModule_Resolvers_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Resolvers to panic when given a non-ResolverRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Resolvers") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Resolvers and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Resolvers(&fakeProvider{})
}

func TestModule_Use_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Use to panic when given a non-MiddlewareRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Use") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Use and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Use(&fakeProvider{})
}

func TestModule_Filters_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Filters to panic when given a non-FilterRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Filters") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Filters and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Filters(&fakeProvider{})
}

func TestModule_Listeners_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Listeners to panic when given a non-ListenerRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Listeners") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Listeners and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Listeners(&fakeProvider{})
}

func TestModule_Schedulers_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Schedulers to panic when given a non-SchedulerRef TokenRef")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Schedulers") || !strings.Contains(msg, "*module.fakeProvider") {
			t.Fatalf("panic message = %q, want it to name Module.Schedulers and *module.fakeProvider", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Schedulers(&fakeProvider{})
}

func TestModule_Exports_WrongTokenKind_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Module.Exports to panic when given a TokenRef that is neither a ProviderRef nor a *Module")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "Module.Exports") || !strings.Contains(msg, "*module.fakeMiddleware") {
			t.Fatalf("panic message = %q, want it to name Module.Exports and *module.fakeMiddleware", msg)
		}
	}()

	m := New(func(m *Module) {})
	m.Exports(&fakeMiddleware{})
}
