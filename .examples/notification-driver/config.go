package main

import (
	"fmt"

	"gonest.dev/gonest"
)

// NotificationConfig is built from real process env vars (populated by
// .env, loaded via gonest.Dotenv().MustLoad in main -- see main.go) before
// any Module exists, so the resolved Driver can decide WHICH module gets
// wired into AppModule below (see module.go).
type NotificationConfig struct {
	// Driver picks the notification backend. Enum restricts it to the 2
	// modules this example actually wires (email.go/sms.go) -- an invalid
	// value fails MustParse at bootstrap, not at first send. Default makes
	// NOTIFICATION_DRIVER optional: unset .env falls back to "email".
	Driver string `json:"driver" env:"NOTIFICATION_DRIVER"`
}

var notificationConfigSchema = gonest.NewSchema(func(t *NotificationConfig, s *gonest.Schema) {
	s.Property(&t.Driver).String().Enum("email", "sms").Default("email")
})

// LoadNotificationConfig loads ./.env itself (package-level var
// initializers run BEFORE main(), so main() calling Dotenv().MustLoad
// would be too late for module.go's own package-level `var config =
// LoadNotificationConfig()`) and returns the resolved config for module
// composition to read synchronously -- plain Go control flow, no DI needed
// (module builder functions have no MustInject of their own).
func LoadNotificationConfig() *NotificationConfig {
	gonest.Dotenv().MustLoad("./.env")
	config := gonest.MustParse[NotificationConfig](gonest.Dotenv(), notificationConfigSchema)
	fmt.Printf("loaded NotificationConfig: %+v\n", config)
	return &config
}
