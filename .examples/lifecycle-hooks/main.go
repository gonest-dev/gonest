// Command lifecycle-hooks demonstrates the lifecycle-hooks feature end to
// end: DbServiceProvider (service.go) registers all 5 hooks
// (OnModuleInit/OnApplicationBootstrap/OnModuleDestroy/
// BeforeApplicationShutdown/OnApplicationShutdown, mirroring NestJS's real
// lifecycle contract). OnModuleInit/OnApplicationBootstrap run automatically
// during NewApp, before Listen ever starts serving -- GET /status/ proves
// this via a real HTTP response. EnableShutdownHooks() opts this app into
// OS-signal-triggered shutdown (SIGINT/SIGTERM), so pressing Ctrl+C runs the
// 3 destroy-phase hooks (OnModuleDestroy -> BeforeApplicationShutdown ->
// OnApplicationShutdown) before the process exits, instead of the OS just
// killing it.
//
// Run:
//
//	cd .examples/lifecycle-hooks && go run .
//
// (its own go.mod, replace-directed at the repo root -- keeps this
// example's dependencies isolated from the library's own go.mod/go.sum)
//
// Try:
//
//	curl localhost:3000/status/
//
// Expected JSON: {"ready":true} -- proving OnApplicationBootstrap already
// ran by the time this app started serving requests.
//
// Then press Ctrl+C in the terminal running `go run .` and watch the
// console: it prints Drain (OnModuleDestroy), Flush
// (BeforeApplicationShutdown), then Close (OnApplicationShutdown,
// signal="SIGINT") in that exact order, before the process exits.
package main

import (
	"gonest.dev/gonest"
)

func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule)

	app.EnableShutdownHooks()

	app.MustListen(":3000")
}
