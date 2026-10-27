package post

import (
	"database/sql"

	"gonest.dev/gonest"

	"blog-api/module/user"
	"blog-api/shared"
)

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := provider.Inject[*sql.DB]()
	userService := provider.Inject[*user.Service]()
	provider.Constructor(func() *Service { return &Service{db: db, userService: userService} })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DatabaseModule, user.Module)
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
