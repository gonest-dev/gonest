package notifier

import (
	"fmt"

	"gonest.dev/gonest"
)

// Config is built from real process env vars (populated by .env, loaded
// via gonest.Dotenv().MustLoad in main.go -- see main.go's own doc
// comment) -- same env-schema-binding pattern .examples/config-dotenv's
// own DatabaseConfig uses.
type Config struct {
	// Driver picks the notification backend. Enum restricts it to the 2
	// modules this example actually wires (email/sms) -- an invalid value
	// fails MustParse at bootstrap, not at first send. Default makes
	// NOTIFICATION_DRIVER optional: unset .env falls back to "email".
	Driver string `json:"driver" env:"NOTIFICATION_DRIVER"`
}

var configSchema = gonest.NewSchema(func(t *Config, s *gonest.Schema) {
	s.Property(&t.Driver).String().Enum("email", "sms").Default("email")
})

// Config_ builds *Config once, at bootstrap, from real process env vars --
// its Constructor is self-contained (no MustInject of its own), the hard
// requirement Module.Lazy's eager MustInject[T](l) resolution places on any
// provider it targets (module-lazy-loading feature's Out of Scope table).
// Registered via Module_'s own Providers (module.go) AND read inside its
// Lazy callback -- Stage 3's skip-if-already-resolved check (LAZY-03)
// guarantees this Constructor still only runs once for the whole bootstrap.
var Config_ = gonest.NewProvider(func(p *gonest.Provider) {
	p.Constructor(func() *Config {
		config := gonest.MustParse[Config](gonest.Dotenv(), configSchema)
		fmt.Printf("loaded notifier.Config: %+v\n", config)
		return &config
	})
})
