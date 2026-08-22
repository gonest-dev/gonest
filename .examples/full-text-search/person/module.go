package person

import (
	"gonest.dev/gonest"
)

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
