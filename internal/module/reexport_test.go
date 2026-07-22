package module

import "testing"

// TestModule_ExportModules_StoresReExportedModules verifies Exports
// appends to exportedModules and OwnExportedModules reflects it -- same
// registration/getter shape as Imports/ImportedModules.
func TestModule_ExportModules_StoresReExportedModules(t *testing.T) {
	child := New(func(m *Module) {})
	root := New(func(m *Module) {
		m.Imports(child)
		m.Exports(child)
	})

	if _, err := assemble(root); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := root.OwnExportedModules()
	if len(got) != 1 || got[0] != child {
		t.Fatalf("OwnExportedModules() = %v, want [child]", got)
	}
}

// TestModule_OwnExportedModules_ReturnsCopyNotInternalSlice mirrors the
// defensive-copy tests for OwnProviders/OwnControllers/etc: mutating the
// returned slice must not affect subsequent calls.
func TestModule_OwnExportedModules_ReturnsCopyNotInternalSlice(t *testing.T) {
	child := New(func(m *Module) {})
	other := New(func(m *Module) {})
	root := New(func(m *Module) {
		m.Imports(child)
		m.Exports(child)
	})

	if _, err := assemble(root); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := root.OwnExportedModules()
	got[0] = other

	got2 := root.OwnExportedModules()
	if got2[0] != child {
		t.Fatalf("OwnExportedModules() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

// TestModule_EffectiveExports_NoReExports_MatchesExportedProviders is the
// regression-safety baseline: with no re-exported modules, EffectiveExports
// must return exactly what ExportedProviders already returns today.
func TestModule_EffectiveExports_NoReExports_MatchesExportedProviders(t *testing.T) {
	p := &fakeProvider{name: "X"}
	m := New(func(m *Module) {
		m.Providers(p)
		m.Exports(p)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := m.EffectiveExports()
	want := m.ExportedProviders()
	if len(got) != len(want) || len(got) != 1 || got[0] != ProviderRef(p) {
		t.Fatalf("EffectiveExports() = %v, want %v (== ExportedProviders())", got, want)
	}
}

// TestModule_EffectiveExports_TwoHopChain_IncludesReExportedModulesOwnExports
// covers B re-exporting C: B's EffectiveExports must include C's own
// exported providers.
func TestModule_EffectiveExports_TwoHopChain_IncludesReExportedModulesOwnExports(t *testing.T) {
	pc := &fakeProvider{name: "C"}
	c := New(func(m *Module) {
		m.Providers(pc)
		m.Exports(pc)
	})
	b := New(func(m *Module) {
		m.Imports(c)
		m.Exports(c)
	})

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	if len(got) != 1 || got[0] != ProviderRef(pc) {
		t.Fatalf("EffectiveExports() = %v, want [pc]", got)
	}
}

// TestModule_EffectiveExports_ThreeHopChain_IsTransitive proves real
// transitivity (not just one extra hop): B re-exports C, C re-exports D,
// and D's own exported provider must surface all the way up to B.
func TestModule_EffectiveExports_ThreeHopChain_IsTransitive(t *testing.T) {
	pd := &fakeProvider{name: "D"}
	d := New(func(m *Module) {
		m.Providers(pd)
		m.Exports(pd)
	})
	c := New(func(m *Module) {
		m.Imports(d)
		m.Exports(d)
	})
	b := New(func(m *Module) {
		m.Imports(c)
		m.Exports(c)
	})

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	if len(got) != 1 || got[0] != ProviderRef(pd) {
		t.Fatalf("EffectiveExports() = %v, want [pd] (transitively through C)", got)
	}
}

// TestModule_EffectiveExports_UndeclaredExportOnReExportedModule_NotIncluded
// verifies re-export only propagates what a module ALREADY chose to
// export: a provider declared via Providers on C but never passed to C's
// own Exports must not leak into B's EffectiveExports just because B
// re-exports C.
func TestModule_EffectiveExports_UndeclaredExportOnReExportedModule_NotIncluded(t *testing.T) {
	pc := &fakeProvider{name: "C-internal"}
	c := New(func(m *Module) {
		m.Providers(pc) // never exported
	})
	b := New(func(m *Module) {
		m.Imports(c)
		m.Exports(c)
	})

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	if len(got) != 0 {
		t.Fatalf("EffectiveExports() = %v, want empty (C never exported pc)", got)
	}
}

// TestModule_EffectiveExports_Diamond_DedupesSharedReExportedModule covers
// B re-exporting both C and D, where C and D both re-export a shared
// module E: E's own exported provider must appear exactly once in B's
// EffectiveExports, not twice.
func TestModule_EffectiveExports_Diamond_DedupesSharedReExportedModule(t *testing.T) {
	pe := &fakeProvider{name: "E"}
	e := New(func(m *Module) {
		m.Providers(pe)
		m.Exports(pe)
	})
	c := New(func(m *Module) {
		m.Imports(e)
		m.Exports(e)
	})
	d := New(func(m *Module) {
		m.Imports(e)
		m.Exports(e)
	})
	b := New(func(m *Module) {
		m.Imports(c, d)
		m.Exports(c, d)
	})

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	count := 0
	for _, p := range got {
		if p == ProviderRef(pe) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("EffectiveExports() contains pe %d times, want exactly 1 (diamond dedup)", count)
	}
}

// TestModule_EffectiveExports_SelfReExport_DoesNotHang proves a module
// re-exporting itself terminates instead of recursing forever.
func TestModule_EffectiveExports_SelfReExport_DoesNotHang(t *testing.T) {
	p := &fakeProvider{name: "X"}
	var b *Module
	b = New(func(m *Module) {
		m.Providers(p)
		m.Exports(p)
		m.Imports(b)
		m.Exports(b)
	})

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	if len(got) != 1 || got[0] != ProviderRef(p) {
		t.Fatalf("EffectiveExports() = %v, want [p] (self re-export terminates without duplicating)", got)
	}
}

// TestModule_EffectiveExports_MutualReExportCycle_DoesNotHang proves a
// mutual re-export cycle (B re-exports C, C re-exports B, with real mutual
// Imports backing both) terminates instead of infinitely recursing.
func TestModule_EffectiveExports_MutualReExportCycle_DoesNotHang(t *testing.T) {
	pb := &fakeProvider{name: "B"}
	pc := &fakeProvider{name: "C"}
	b := New(func(m *Module) {})
	c := New(func(m *Module) {})
	b.fn = func(m *Module) {
		m.Providers(pb)
		m.Exports(pb)
		m.Imports(c)
		m.Exports(c)
	}
	c.fn = func(m *Module) {
		m.Providers(pc)
		m.Exports(pc)
		m.Imports(b)
		m.Exports(b)
	}

	if _, err := assemble(b); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}

	got := b.EffectiveExports()
	if len(got) != 2 {
		t.Fatalf("EffectiveExports() = %v, want 2 providers (pb and pc, cycle terminated)", got)
	}
}

// TestAssemble_ExportModulesNotImported_ReturnsError verifies assemble
// rejects Exports(X) when X was never passed to Imports, symmetric to
// the existing check for Exports(p) without Providers(p).
func TestAssemble_ExportModulesNotImported_ReturnsError(t *testing.T) {
	other := New(func(m *Module) {})
	m := New(func(m *Module) {
		m.Exports(other)
	})

	_, err := assemble(m)
	if err == nil {
		t.Fatalf("assemble returned nil error, want error for re-exporting an unimported module")
	}
}

// TestAssemble_ExportModulesImported_NoError is the positive counterpart:
// when the re-exported module was also imported, assemble must succeed.
func TestAssemble_ExportModulesImported_NoError(t *testing.T) {
	other := New(func(m *Module) {})
	m := New(func(m *Module) {
		m.Imports(other)
		m.Exports(other)
	})

	if _, err := assemble(m); err != nil {
		t.Fatalf("assemble returned unexpected error: %v", err)
	}
}
