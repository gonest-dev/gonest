package notifier

import (
	"notification-driver/notifier/impl/email"
	"notification-driver/notifier/impl/sms"
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

type Port = port.Notifier

// Module_ picks -- via Module.Lazy, from INSIDE the DI graph -- which real
// adapter module (email.Module_ or sms.Module_) answers for Notifier,
// mirroring NestJS's DynamicModule.forRootAsync (module-lazy-loading
// feature, Milestone 24). Config_ (config.go) is this module's OWN
// Schema-validated Provider (env:"NOTIFICATION_DRIVER"); Lazy reads it via
// gonest.MustInject[*Config](l) -- eagerly, synchronously, right here at
// Stage 1 assembly time -- and Imports/Exports the winning driver module
// before assemble.go's BFS ever looks at this module's Imports/Exports.
//
// Replaces the old ModuleForRoot(driver string) free function: AppModule_
// (../module.go) now just imports this single Module_ var, and never reads
// NOTIFICATION_DRIVER itself.
//
// email/sms's own Provider_.Constructor returns a CONCRETE type (*Service),
// not notifier.Notifier -- each impl's own module.go explicitly registers
// it as port.Notifier via gonest.ProviderAs[port.Notifier](Provider_)
// (interface resolution is exclusively explicit now, no structural
// reflect.Type.Implements() fallback -- see gonest.ProviderAs's own doc
// comment). email/sms import notifier/port (the interface's home package)
// to do that wrapping, but NOT this notifier package itself -- that is
// what avoids an import cycle (this package already imports email/sms to
// reference their Module_ vars; if they imported this package back, that
// would cycle).
var Module_ = gonest.NewModule(func(m *gonest.Module) {
	m.Providers(Config_)
	// Exported unconditionally (unlike the driver-specific email/sms
	// re-export below, decided inside Lazy) so a consumer outside this
	// module -- e.g. ../controller.go, which used to read a package-level
	// `config` var straight out of main.go -- can inject *Config directly
	// instead.
	m.Exports(Config_)
	m.Lazy(func(l *gonest.LazyModule) {
		config := gonest.MustInject[*Config](l)
		switch config.Driver {
		case "sms":
			l.Imports(sms.Module_)
			l.Exports(sms.Module_)
		default:
			l.Imports(email.Module_)
			l.Exports(email.Module_)
		}
	})
})
