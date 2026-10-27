package main

import "gonest.dev/gonest"

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	emitter := provider.Inject[*gonest.Emitter]()
	provider.Constructor(func() *Service { return &Service{emitter: emitter} })
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(Provider)
	module.Resolvers(PostResolver)
})
