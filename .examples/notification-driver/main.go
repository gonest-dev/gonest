// Command notification-driver demonstrates gonest.Module's main real-world
// value: injection across a DIFFUSE scope decided by config, not by source
// code. NOTIFICATION_DRIVER (loaded via gonest.Dotenv, validated+defaulted
// via a Schema with Enum+Default, see config.go) picks, at bootstrap,
// whether notifier/module.go wires notifier/email.Module_ or
// notifier/sms.Module_ into AppModule_. NotificationController_
// (controller.go) injects notifier.Notifier -- the INTERFACE only -- so
// its handler code is identical no matter which driver actually answers.
//
// Run:
//
//	cd .examples/notification-driver && go run .
//
// (its own go.mod, replace-directed at the repo root -- keeps this
// example's dependencies isolated from the library's own go.mod/go.sum)
//
// Try (.env ships with NOTIFICATION_DRIVER=sms):
//
//	curl -X POST localhost:3000/notifications -d '{"to":"+15551234","message":"hi"}'
//
// Expected: server log prints "[sms] to=+15551234 message=\"hi\"", response
// body is {"driver":"sms","sent":true}. Comment out NOTIFICATION_DRIVER in
// .env (or delete the line) and re-run -- Default("email") kicks in, same
// endpoint now logs "[email] ..." and returns {"driver":"email",...} with
// ZERO changes to controller.go or notifier's consumer-facing code.
package main

import (
	"gonest.dev/gonest"
)

func main() {
	// .env is already loaded -- module.go's package-level `var config =
	// LoadNotificationConfig()` (config.go) ran before main() even
	// started, and it loads ./.env itself for exactly that reason.
	app, err := gonest.NewApp[gonest.FiberApp](AppModule_)
	if err != nil {
		panic(err)
	}

	err = app.Listen(":3000")
	if err != nil {
		panic(err)
	}
}
