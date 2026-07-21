package person

import (
	"gonest.dev/gonest"
)

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *Service { return &Service{} })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
