package gonest

import "github.com/gonest-dev/gonest/internal/module"

// Module represents a declarative unit of the DI graph: its own providers
// and controllers, the modules it imports, and the subset of its own
// providers it exports to importers.
type Module = module.Module

// NewModule creates a Module that defers fn until Stage 1 assembly runs
// during bootstrap.
var NewModule = module.New
