package module

import "testing"

func TestModule_Lazy_RunsFnSynchronously(t *testing.T) {
	executed := false

	m := &Module{}
	m.Lazy(func(l *LazyModule) {
		executed = true
	})

	if !executed {
		t.Fatalf("Module.Lazy(fn) did not run fn synchronously")
	}
}

func TestModule_Lazy_NilFnIsNoOp(t *testing.T) {
	m := &Module{}

	// Must not panic.
	m.Lazy(nil)
}

func TestLazyModule_Imports_LandsOnOwnerModule(t *testing.T) {
	owner := &Module{}
	imported := &Module{}

	owner.Lazy(func(l *LazyModule) {
		l.Imports(imported)
	})

	got := owner.ImportedModules()
	if len(got) != 1 || got[0] != imported {
		t.Fatalf("LazyModule.Imports did not land on owner's own Imports storage, got %v", got)
	}
}

func TestLazyModule_Exports_LandsOnOwnerModule(t *testing.T) {
	owner := &Module{}
	p := &fakeProvider{name: "p"}
	owner.Providers(p)

	owner.Lazy(func(l *LazyModule) {
		l.Exports(p)
	})

	got := owner.ExportedProviders()
	if len(got) != 1 || got[0] != p {
		t.Fatalf("LazyModule.Exports did not land on owner's own Exports storage, got %v", got)
	}
}

func TestLazyModule_Exports_ModuleRef_LandsOnOwnerModule(t *testing.T) {
	owner := &Module{}
	imported := &Module{}
	owner.Imports(imported)

	owner.Lazy(func(l *LazyModule) {
		l.Exports(imported)
	})

	got := owner.OwnExportedModules()
	if len(got) != 1 || got[0] != imported {
		t.Fatalf("LazyModule.Exports(*Module) did not land on owner's own re-export storage, got %v", got)
	}
}

func TestLazyModule_OwnProviders_DelegatesToOwner(t *testing.T) {
	owner := &Module{}
	p1 := &fakeProvider{name: "p1"}
	p2 := &fakeProvider{name: "p2"}
	owner.Providers(p1, p2)

	var seen []ProviderRef
	owner.Lazy(func(l *LazyModule) {
		seen = l.OwnProviders()
	})

	if len(seen) != 2 || seen[0] != p1 || seen[1] != p2 {
		t.Fatalf("LazyModule.OwnProviders did not delegate to owner.OwnProviders, got %v", seen)
	}
}

func TestLazyModule_OwnProviders_IsDefensiveCopy(t *testing.T) {
	owner := &Module{}
	p1 := &fakeProvider{name: "p1"}
	owner.Providers(p1)

	var seen []ProviderRef
	owner.Lazy(func(l *LazyModule) {
		seen = l.OwnProviders()
		seen[0] = nil
	})

	if owner.providers[0] != p1 {
		t.Fatalf("mutating LazyModule.OwnProviders' returned slice affected owner's internal state")
	}
}

func TestModule_Lazy_RunsBeforeAssembleReadsImportsExports(t *testing.T) {
	imported := &Module{}
	p := &fakeProvider{name: "p"}

	root := New(func(m *Module) {
		m.Providers(p)
		m.Lazy(func(l *LazyModule) {
			l.Imports(imported)
			l.Exports(p)
		})
	})

	order, err := root.Assemble()
	if err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
	if len(order) != 2 || order[0] != root || order[1] != imported {
		t.Fatalf("Assemble did not walk the module wired via Lazy's Imports call, got %v", order)
	}
	if len(root.exports) != 1 || root.exports[0] != p {
		t.Fatalf("Assemble did not see the export wired via Lazy's Exports call")
	}
}
