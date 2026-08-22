package email

import (
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

// AsNotifier_ wraps Provider_, explicitly registering it as port.Notifier --
// MustInject[port.Notifier] only resolves providers wrapped this way now
// (gonest's ProviderAs, see notifier/module.go's updated doc comment: the
// old reflect.Type.Implements() structural fallback no longer exists).
// Provider_'s own concrete registration (below) stays required too -- the
// wrapper never drives construction itself.
var AsNotifier_ = gonest.ProviderAs[port.Notifier](Provider_)

// Module_ owns+exports Provider_ (its own concrete registration) AND
// AsNotifier_ (the explicit port.Notifier view of it). Never imported
// directly by AppModule_ -- only notifier.ModuleForRoot (../module.go)
// decides whether it enters the graph at all.
var Module_ = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider_, AsNotifier_)
	module.Exports(AsNotifier_)
})
