package email

import (
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

// AsNotifier wraps Provider, explicitly registering it as port.Notifier --
// MustInject[port.Notifier] only resolves providers wrapped this way now
// (gonest's ProviderAs, see notifier/module.go's updated doc comment: the
// old reflect.Type.Implements() structural fallback no longer exists).
// Provider's own concrete registration (below) stays required too -- the
// wrapper never drives construction itself.
var AsNotifier = gonest.ProviderAs[port.Notifier](Provider)

// Module owns+exports Provider (its own concrete registration) AND
// AsNotifier (the explicit port.Notifier view of it). Never imported
// directly by AppModule -- only notifier.ModuleForRoot (../module.go)
// decides whether it enters the graph at all.
var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider, AsNotifier)
	module.Exports(AsNotifier)
})
