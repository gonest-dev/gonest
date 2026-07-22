// Package resolver implements the DI Resolver's search step: given a
// *module.Module (the module a MustInject caller belongs to) and a target
// reflect.Type, it finds the module.ProviderRef that resolves that type,
// searching the owner module's own providers first and then its imported
// modules' exported providers.
package resolver

import (
	"fmt"

	"reflect"

	"gonest.dev/gonest/internal/module"
)

// ModuleName returns a debug-friendly identifier for m, reusing
// internal/module's own naming so error messages here are consistent with
// errors produced during Stage 1 assembly (e.g. the "exports provider it
// does not declare" error).
func ModuleName(m *module.Module) string {
	return module.ModuleName(m)
}

// Find searches for the module.ProviderRef that resolves target, starting
// at owner:
//
//  1. owner's own providers (module-scoped priority: local always wins).
//  2. owner's imported modules' EXPORTED providers only -- an imported
//     module's un-exported providers are invisible to importers, matching
//     Nest's exports: [] semantics (see context.md).
//
// If target exists in an imported module but that module did not export
// it, Find panics distinguishing that case from "not registered anywhere"
// (see design.md's Error Handling Strategy table).
func Find(owner *module.Module, target reflect.Type) module.ProviderRef {
	if ref := findOwn(owner, target); ref != nil {
		return ref
	}

	for _, imported := range owner.ImportedModules() {
		if ref := findExported(imported, target); ref != nil {
			return ref
		}
		if hasOwnUnexported(imported, target) {
			panic(fmt.Sprintf("gonest: type %s exists in module %s but is not exported", target.String(), ModuleName(imported)))
		}
	}

	panic(fmt.Sprintf("gonest: no provider registered for type %s", target.String()))
}

// findOwn searches m's own providers for one resolving target.
func findOwn(m *module.Module, target reflect.Type) module.ProviderRef {
	for _, p := range m.OwnProviders() {
		if p.ResolvedType() == target {
			return p
		}
	}
	return nil
}

// findExported searches m's EFFECTIVE exports (its own exports plus every
// provider re-exported transitively via Exports's *Module arguments) for
// one resolving
// target -- an imported module's re-exports are visible to importers exactly
// as if the re-exporting module had exported the provider itself.
func findExported(m *module.Module, target reflect.Type) module.ProviderRef {
	for _, p := range m.EffectiveExports() {
		if p.ResolvedType() == target {
			return p
		}
	}
	return nil
}

// hasOwnUnexported reports whether m declares (via Providers) a provider
// resolving target, regardless of whether it is exported. Used to produce
// the "exists but is not exported" panic instead of a generic "not
// registered" one when the type is one BFS hop away but not visible.
func hasOwnUnexported(m *module.Module, target reflect.Type) bool {
	return findOwn(m, target) != nil
}
