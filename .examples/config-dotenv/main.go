// Command config-dotenv demonstrates the dotenv-loading feature end to end:
// gonest.Dotenv().MustLoad loads a real ./.env file into the process
// environment as the FIRST line of main() (before any Module/Provider/
// NewApp exists), then DatabaseConfigProvider's Constructor (config.go)
// calls gonest.MustParse[DatabaseConfig](gonest.Dotenv(), databaseConfigSchema)
// to build a real *DatabaseConfig from those env vars -- DB_HOST comes
// straight from .env, DB_PORT is absent from .env and resolves via
// PropertyBuilder.Default(5432), and DB_URL proves ${DB_HOST}
// interpolation ran.
//
// Run:
//
//	cd .examples/config-dotenv && go run .
//
// (its own go.mod, replace-directed at the repo root -- keeps this
// example's dependencies isolated from the library's own go.mod/go.sum)
//
// Try:
//
//	curl localhost:3000/config
//
// Expected JSON: {"host":"db.internal","port":5432,"url":"postgres://db.internal/app"}
package main

import (
	"gonest.dev/gonest"
)

func main() {
	gonest.Dotenv().MustLoad("./.env")

	app, err := gonest.NewApp[gonest.FiberApp](AppModule)
	if err != nil {
		panic(err)
	}

	err = app.Listen(":3000")
	if err != nil {
		panic(err)
	}
}
