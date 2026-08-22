package main

import (
	"fmt"

	"gonest.dev/gonest"
)

// DbService is a stand-in for a real resource (a DB pool, a broker
// connection) that needs setup AFTER its own dependencies are ready
// (OnModuleInit) and BEFORE the app is considered fully bootstrapped
// (OnApplicationBootstrap), plus teardown on shutdown (OnModuleDestroy/
// BeforeApplicationShutdown/OnApplicationShutdown) -- exactly INSIGHT-ON.md's
// original sketch, now with the real 5-hook set (Milestone 20).
type DbService struct {
	ready bool
}

func (s *DbService) Connect() error {
	fmt.Println("[DbService] Connect (OnModuleInit): opening pool...")
	return nil
}

func (s *DbService) Ping() error {
	fmt.Println("[DbService] Ping (OnApplicationBootstrap): pool healthy, marking ready")
	s.ready = true
	return nil
}

func (s *DbService) Drain() error {
	fmt.Println("[DbService] Drain (OnModuleDestroy): stopping new queries")
	s.ready = false
	return nil
}

func (s *DbService) Flush() error {
	fmt.Println("[DbService] Flush (BeforeApplicationShutdown): flushing in-flight writes")
	return nil
}

func (s *DbService) Close(signal string) error {
	fmt.Printf("[DbService] Close (OnApplicationShutdown, signal=%q): pool closed\n", signal)
	return nil
}

// Ready reports whether OnApplicationBootstrap has run -- used by
// StatusController to prove, via a real HTTP response, that the hook
// actually fired during NewApp (not just "compiled").
func (s *DbService) Ready() bool {
	return s.ready
}

// DbServiceProvider registers all 5 lifecycle hooks against the same
// *DbService instance Constructor builds. Every hook's first parameter is
// the provider's own resolved instance -- gonest feeds the SAME *DbService
// back in, so a hook can mutate/read state other hooks (and route handlers,
// via MustInject) observe.
var DbServiceProvider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *DbService { return &DbService{} })

	provider.OnModuleInit(func(s *DbService) error { return s.Connect() })
	provider.OnApplicationBootstrap(func(s *DbService) error { return s.Ping() })
	provider.OnModuleDestroy(func(s *DbService) error { return s.Drain() })
	provider.BeforeApplicationShutdown(func(s *DbService, _ string) error { return s.Flush() })
	provider.OnApplicationShutdown(func(s *DbService, signal string) error { return s.Close(signal) })
})
