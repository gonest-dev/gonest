package main

import (
	"notification-driver/notifier"

	"gonest.dev/gonest"
)

// config is resolved ONCE, before AppModule_ is even built -- its Driver
// field decides which real notifier.Module_ enters the graph.
var config = LoadNotificationConfig()

// NotifierModule_ is the RESULT of the choice, a real *gonest.Module value
// (either notifier/email.Module_ or notifier/sms.Module_, see
// notifier/module.go). AppModule_ imports this variable, never the driver
// packages directly -- the entire notifier/ package could be swapped for a
// 3rd driver without AppModule_'s own source changing at all.
var NotifierModule_ = notifier.ModuleForRoot(config.Driver)

var AppModule_ = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(NotifierModule_)
	module.Controllers(NotificationController_)
})
