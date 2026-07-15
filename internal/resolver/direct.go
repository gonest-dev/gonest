package resolver

import (
	"reflect"

	"github.com/gonest-dev/gonest/internal/module"
)

// resolvedGetter is satisfied by *provider.Provider (its ResolvedValue
// getter, added alongside Stage 3's SetResolvedValue -- see stage3.go's own
// resolvedSetter). Declared here for the same reason constructable/scoped/
// resolvedSetter are: module.ProviderRef only needs identity/type
// information, not the ability to read a fully-resolved value -- that is
// this file's own concern, used strictly AFTER Stage 3 (Resolve) has
// completed.
type resolvedGetter interface {
	ResolvedValue() (reflect.Value, bool)
}

// candidateProviders collects every module.ProviderRef visible from scope:
// for each module in scope, its own providers PLUS its imports' exported
// providers -- deduplicated by identity, since two different scope modules
// may share a common imported/exported provider (e.g. a Guard referenced by
// controllers in two modules that both import the same shared module).
// Mirrors Find's own-then-imports search, but across every module in scope
// rather than a single owner module + its own imports.
func candidateProviders(scope []*module.Module) []module.ProviderRef {
	seen := make(map[module.ProviderRef]bool)
	var out []module.ProviderRef

	add := func(p module.ProviderRef) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, m := range scope {
		for _, p := range m.OwnProviders() {
			add(p)
		}
		for _, imported := range m.ImportedModules() {
			for _, p := range imported.ExportedProviders() {
				add(p)
			}
		}
	}

	return out
}

// findDirectMatches returns every candidate in scope whose resolved value
// satisfies t: for a pointer t, every EXACT ResolvedType() match; for an
// interface t, an EXACT match if one exists (returned ALONE, never combined
// with Implements() matches -- an override registered AS the interface type
// itself takes precedence, per spec.md's own edge case), otherwise every
// provider whose ResolvedType() satisfies reflect.Type.Implements(t).
// Candidates with no resolved value yet (should not happen once phase 1 has
// completed, per this feature's bootstrap ordering) are silently skipped
// rather than erroring -- an implementation invariant, not a user-facing
// scenario this feature needs its own error message for.
func findDirectMatches(scope []*module.Module, t reflect.Type) []reflect.Value {
	candidates := candidateProviders(scope)

	var exact []reflect.Value
	for _, p := range candidates {
		if p.ResolvedType() != t {
			continue
		}
		if v, ok := resolvedValueOf(p); ok {
			exact = append(exact, v)
		}
	}
	if len(exact) > 0 {
		return exact
	}

	if t.Kind() != reflect.Interface {
		return nil
	}

	var implementing []reflect.Value
	for _, p := range candidates {
		rt := p.ResolvedType()
		if rt == nil || !rt.Implements(t) {
			continue
		}
		if v, ok := resolvedValueOf(p); ok {
			implementing = append(implementing, v)
		}
	}
	return implementing
}

// FindDirect returns the single value in scope resolving t, and whether
// exactly one was found. Ambiguity (2+ matches) is reported as "not found"
// here -- internal/inject's MustInject[T] uses FindDirectAll (not this
// function) for interface T specifically so it can distinguish "zero
// matches" from "ambiguous" in its own panic message; FindDirect is used
// as-is for pointer T, where 2+ exact matches is not a scenario this
// feature's spec anticipates (a well-formed DI graph never registers the
// same concrete pointer type twice in one scope).
func FindDirect(scope []*module.Module, t reflect.Type) (reflect.Value, bool) {
	matches := findDirectMatches(scope, t)
	if len(matches) != 1 {
		return reflect.Value{}, false
	}
	return matches[0], true
}

// FindDirectAll returns every value in scope resolving t (t must be an
// interface kind -- caller's responsibility to check, per spec.md TB-03's
// panic requirement living in internal/inject, not here).
func FindDirectAll(scope []*module.Module, t reflect.Type) []reflect.Value {
	return findDirectMatches(scope, t)
}

func resolvedValueOf(p module.ProviderRef) (reflect.Value, bool) {
	rg, ok := p.(resolvedGetter)
	if !ok {
		return reflect.Value{}, false
	}
	return rg.ResolvedValue()
}
