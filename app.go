package gonest

import "github.com/gonest-dev/gonest/internal/app"

// App is the minimal handle returned once NewApp has finished bootstrapping
// the whole dependency graph. Because this is a true Go type alias (not a
// defined type), every method on app.App -- including MustListen -- is
// automatically visible on gonest.App with zero extra wrapper code.
type App = app.App

// HttpAdapter is the constraint NewApp[T]/MustNewApp[T] require of their
// type argument -- e.g. FiberApp. See internal/app.HttpAdapter's doc
// comment for the full contract (RegisterRoute, Listen, Init).
type HttpAdapter = app.HttpAdapter

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels). See internal/app.AppOptions's doc comment for the
// full contract. LogLevel/OnListen's own aliases live in options.go.
type AppOptions = app.AppOptions

// NewApp bootstraps root through all 3 DI stages (Structural Assembly,
// Builder Execution, Cycle Detection + Parallel Resolution) plus Stage 2.5
// (route collection/registration onto a T-typed HttpAdapter, e.g.
// gonest.NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})). Go cannot
// re-export a generic function via var, so this is a real wrapper calling
// the internal one (same pattern as MustInject/MustParam, see AD-004 in
// STATE.md).
//
// T is passed BY VALUE at the call site (e.g. FiberApp, not *FiberApp) per
// INSIGHT.md's ground-truth call-site contract; the second type parameter
// enforcing "*T implements HttpAdapter" is inferred automatically -- see
// internal/app's httpAdapterPtr for why that extra parameter exists. See
// internal/app.NewApp's doc comment for the full bootstrap contract.
//
// opts is threaded straight through to internal/app.NewApp -- this wrapper
// does not interpret it.
func NewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts AppOptions) (*App, error) {
	return app.NewApp[T, PT](root, opts)
}

// MustNewApp calls NewApp and panics if it returns an error.
func MustNewApp[T any, PT interface {
	*T
	HttpAdapter
}](root *Module, opts AppOptions) *App {
	return app.MustNewApp[T, PT](root, opts)
}
