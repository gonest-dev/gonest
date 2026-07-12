package module

import "testing"

// fakeProvider is a minimal stand-in for the real *provider.Provider type
// (owned by a later task). It only needs to satisfy providerRef.
type fakeProvider struct {
	name string
}

func (*fakeProvider) isProvider() {}

// fakeController is a minimal stand-in for the real *controller.Controller
// type (owned by a later task). It only needs to satisfy controllerRef.
type fakeController struct{}

func (*fakeController) isController() {}

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
