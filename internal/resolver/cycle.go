package resolver

import (
	"errors"
	"strings"

	"github.com/gonest-dev/gonest/internal/module"
)

// color tracks DFS visitation state for 3-color cycle detection: white
// (unvisited) nodes are simply absent from the map, gray nodes are on the
// current DFS path (ancestors of the node being explored), and black nodes
// are fully explored with no cycle found through them.
type color int

const (
	white color = iota
	gray
	black
)

// DetectCycle walks graph (as built by BuildGraph) with a standard 3-color
// DFS. It returns nil if the graph is acyclic. On finding a back-edge (an
// edge into a node currently gray, i.e. an ancestor on the current DFS
// path), it reconstructs the full cycle chain from the DFS path and returns
// an error of the form "circular dependency: A -> B -> C -> A" using each
// node's ResolvedType().String() as its display name.
func DetectCycle(graph map[module.ProviderRef][]module.ProviderRef) error {
	colors := make(map[module.ProviderRef]color)
	var path []module.ProviderRef

	var visit func(node module.ProviderRef) error
	visit = func(node module.ProviderRef) error {
		colors[node] = gray
		path = append(path, node)

		for _, dep := range graph[node] {
			switch colors[dep] {
			case gray:
				return cycleError(path, dep)
			case black:
				continue
			default:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		colors[node] = black
		path = path[:len(path)-1]
		return nil
	}

	for node := range graph {
		if _, seen := colors[node]; seen {
			continue
		}
		if err := visit(node); err != nil {
			return err
		}
	}

	return nil
}

// cycleError builds the "circular dependency: A -> B -> ... -> A" error
// from the current DFS path and the back-edge target (the gray ancestor
// that closes the cycle). The chain starts at backEdgeTarget's position in
// path (the earliest ancestor that is part of the cycle, not the whole DFS
// path from the graph's traversal root) and repeats backEdgeTarget at the
// end to make the loop visually explicit.
func cycleError(path []module.ProviderRef, backEdgeTarget module.ProviderRef) error {
	start := 0
	for i, node := range path {
		if node == backEdgeTarget {
			start = i
			break
		}
	}

	chain := path[start:]

	names := make([]string, 0, len(chain)+1)
	for _, node := range chain {
		names = append(names, node.ResolvedType().String())
	}
	names = append(names, backEdgeTarget.ResolvedType().String())

	return errors.New("circular dependency: " + strings.Join(names, " -> "))
}
