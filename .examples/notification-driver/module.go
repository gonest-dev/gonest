package main

import (
	"notification-driver/notifier"

	"gonest.dev/gonest"
)

// config is resolved ONCE, before AppModule is even built -- its Driver
// field decides which real notifier.Module enters the graph.
var config = LoadNotificationConfig()

// NotifierModule is the RESULT of the choice, a real *gonest.Module value
// (either notifier/email.Module or notifier/sms.Module, see
// notifier/module.go). AppModule imports this variable, never the driver
// packages directly -- the entire notifier/ package could be swapped for a
// 3rd driver without AppModule's own source changing at all.
var NotifierModule = notifier.ModuleForRoot(config.Driver)

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(NotifierModule)
	module.Controllers(NotificationController)
})
