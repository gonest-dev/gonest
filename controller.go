package gonest

import "github.com/gonest-dev/gonest/internal/controller"

// Controller represents a declarative unit that consumes providers via
// MustInject. It does not participate in the provider resolution graph --
// it only consumes placeholders it requests.
type Controller = controller.Controller

// NewController creates a Controller that defers fn until bootstrap runs
// it.
var NewController = controller.New
