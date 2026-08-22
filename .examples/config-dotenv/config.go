package main

import (
	"fmt"

	"gonest.dev/gonest"
)

// DatabaseConfig is the struct built from real process env vars (populated
// by .env, loaded via gonest.Dotenv().MustLoad in main -- see main.go) --
// env-schema-binding feature (gonest.MustParse[T](gonest.Dotenv(), schema)).
type DatabaseConfig struct {
	// Host is REQUIRED and always present in .env (DB_HOST) -- proves a
	// real .env value reaches the struct.
	Host string `json:"host" env:"DB_HOST"`
	// Port is deliberately ABSENT from .env -- proves PropertyBuilder.
	// Default(5432) resolves the field when the env var isn't set.
	Port int64 `json:"port" env:"DB_PORT"`
	// Url is built via dotenvx-style ${DB_HOST} interpolation inside .env
	// itself (DB_URL="postgres://${DB_HOST}/app") -- proves interpolation
	// syntax works end-to-end, not just Default/Required.
	Url string `json:"url" env:"DB_URL"`
}

var databaseConfigSchema = gonest.NewSchema(func(t *DatabaseConfig, s *gonest.Schema) {
	s.Property(&t.Host).String().Required()
	s.Property(&t.Port).Integer().Default(int64(5432))
	s.Property(&t.Url).String().Required()
})

// DatabaseConfigProvider builds *DatabaseConfig once, at bootstrap, from
// real process env vars -- the Constructor calls gonest.MustParse against
// gonest.Dotenv() (already loaded by main() BEFORE NewApp ran, see main.go's
// doc comment), exactly the end-to-end flow this example demonstrates.
var DatabaseConfigProvider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *DatabaseConfig {
		config := gonest.MustParse[DatabaseConfig](gonest.Dotenv(), databaseConfigSchema)
		fmt.Printf("loaded DatabaseConfig: %+v\n", config)
		return &config
	})
})
