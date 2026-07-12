package gonest

import "github.com/gonest-dev/gonest/internal/provider"

// Provider represents one resolvable type in the DI graph: it holds the
// Scope, Constructor, and (after bootstrap) the resolved instance.
type Provider = provider.Provider

// NewProvider creates a Provider that defers fn until Stage 2 builder
// execution runs during bootstrap.
var NewProvider = provider.New
