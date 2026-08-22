package user

import (
	"blog-api/shared"
	"database/sql"

	"gonest.dev/gonest"
)

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := gonest.MustInject[*sql.DB](provider)
	provider.Constructor(func() *Service { return &Service{db: db} })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DatabaseModule)
	module.Providers(Provider)
	module.Controllers(Controller)
	module.Exports(Provider)
})
