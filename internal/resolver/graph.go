package resolver

import (
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolve"
)

// BuildGraph combines the pending edges recorded by internal/resolve (every
// MustResolve[T] call made so far, across all owners) with Find (this
// package's module-scoped search) to produce the provider dependency graph:
// each key is a Provider that itself called MustResolve during its own
// builder fn, mapped to the list of providers it depends on.
//
// Pending edges whose owner is not itself a module.ProviderRef (e.g. a
// *controller.Controller) are skipped entirely -- a Controller consumes
// providers but is never a dependency of anything, so it must never become
// a node in the cycle-detection graph (design.md: "Controller não entra no
// grafo de resolução").
func BuildGraph() map[module.ProviderRef][]module.ProviderRef {
	graph := make(map[module.ProviderRef][]module.ProviderRef)

	for _, edge := range resolve.PendingEdges() {
		ownerRef, ok := edge.Owner.(module.ProviderRef)
		if !ok {
			continue
		}

		ownerModule := edge.Owner.OwnerModule()
		target := Find(ownerModule, edge.TargetType)

		graph[ownerRef] = append(graph[ownerRef], target)
	}

	return graph
}
