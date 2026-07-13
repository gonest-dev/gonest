package gonest

import "github.com/gonest-dev/gonest/internal/app"

// App is the minimal handle returned once NewApp has finished bootstrapping
// the whole dependency graph.
type App = app.App

// NewApp bootstraps root through all 3 DI stages (Structural Assembly,
// Builder Execution, Cycle Detection + Parallel Resolution). See
// internal/app.NewApp's doc comment for the full contract.
func NewApp(root *Module) (*App, error) {
	return app.NewApp(root)
}

// MustNewApp calls NewApp and panics if it returns an error.
func MustNewApp(root *Module) *App {
	return app.MustNewApp(root)
}
