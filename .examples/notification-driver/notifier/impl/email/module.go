package email

import "gonest.dev/gonest"

// Module owns+exports Provider. Never imported directly by AppModule --
// only notifier.ModuleForRoot (../module.go) decides whether it enters the
// graph at all.
var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider)
	module.Exports(Provider)
})
