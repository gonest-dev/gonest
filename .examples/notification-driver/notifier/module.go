package notifier

import (
	"notification-driver/notifier/impl/email"
	"notification-driver/notifier/impl/sms"
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

type Port = port.Notifier

// ModuleForRoot is a dynamic module (see /docs/core-concepts/modules --
// "Dynamic modules"): a plain function closing over a config value, run
// ONCE at package-init time, that picks which real adapter module answers
// for Notifier. AppModule_ (../module.go) imports ONLY this function's
// result -- it never imports email.Module_ or sms.Module_ directly, so
// swapping the driver never touches AppModule_'s own wiring.
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
//
// driver must already be validated (../config.go's Enum("email", "sms"))
// -- this function trusts its input, it does not re-validate.
func ModuleForRoot(driver string) *gonest.Module {
	switch driver {
	case "sms":
		return sms.Module_
	default:
		return email.Module_
	}
}
