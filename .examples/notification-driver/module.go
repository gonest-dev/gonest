package main

import (
	"notification-driver/notifier"

	"gonest.dev/gonest"
)

// AppModule_ imports notifier.Module_ -- which decides, via its own
// Module.Lazy callback (notifier/module.go), whether email.Module_ or
// sms.Module_ actually answers for Notifier. main.go no longer reads
// NOTIFICATION_DRIVER or picks a module itself (module-lazy-loading
// feature, Milestone 24 -- replaces the old ModuleForRoot(driver) free
// function called from here).
var AppModule_ = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(notifier.Module_)
	module.Controllers(NotificationController_)
})
