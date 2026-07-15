package module

import "fmt"

// Assemble performs Stage 1 (Structural Assembly) on root: it walks the
// Imports graph via BFS, executing each module's fn exactly once even when
// the import graph contains diamonds (multiple modules importing the same
// shared module), wires each registered provider/controller's OwnerModule
// back to its owning Module, and validates that each module's Exports is a
// subset of its own Providers.
//
// It returns the list of visited modules (root first) or an error if any
// module exports a provider it did not declare. Callers outside this
// package (e.g. internal/resolver, or a future bootstrap entry point) use
// this to obtain a fully-wired module tree before searching it.
func (m *Module) Assemble() ([]*Module, error) {
	return assemble(m)
}

// assemble performs Stage 1 (Structural Assembly): it walks the Imports
// graph starting at root via BFS, executing each module's fn exactly once
// even when the import graph contains diamonds (multiple modules importing
// the same shared module). After every module's fn has run, it validates
// that each module's Exports is a subset of its own Providers.
//
// It returns the list of visited modules (root first) or an error if any
// module exports a provider it did not declare.
func assemble(root *Module) ([]*Module, error) {
	visited := make(map[*Module]bool)
	order := make([]*Module, 0)
	queue := []*Module{root}

	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]

		if visited[m] {
			continue
		}
		visited[m] = true
		order = append(order, m)

		if m.fn != nil {
			m.fn(m)
		}

		for _, p := range m.providers {
			p.SetOwnerModule(m)
		}
		for _, c := range m.controllers {
			c.SetOwnerModule(m)
		}
		for _, l := range m.listeners {
			l.SetOwnerModule(m)
		}

		queue = append(queue, m.imports...)
	}

	for _, m := range order {
		if err := validateExports(m); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// validateExports ensures every provider in m.Exports was also registered
// via m.Providers on the same module.
func validateExports(m *Module) error {
	declared := make(map[ProviderRef]bool, len(m.providers))
	for _, p := range m.providers {
		declared[p] = true
	}

	for _, p := range m.exports {
		if !declared[p] {
			return fmt.Errorf("module %s exports provider %v it does not declare", moduleName(m), p)
		}
	}

	return nil
}

// moduleName returns a debug-friendly identifier for a module: the name set
// via Module.Name, if any, or a pointer-address fallback otherwise (also
// used when Name("") was called with an empty string).
func moduleName(m *Module) string {
	if m.name != "" {
		return m.name
	}
	return fmt.Sprintf("%p", m)
}

// ModuleName returns a debug-friendly identifier for m, for use in error
// messages produced by other internal/* packages (e.g. internal/resolver)
// that need to name a module but have no access to its private fields.
func ModuleName(m *Module) string {
	return moduleName(m)
}
