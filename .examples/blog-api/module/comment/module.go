package comment

import (
	"database/sql"

	"gonest.dev/gonest"

	"blog-api/module/post"
	"blog-api/module/user"
	"blog-api/shared"
)

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := provider.Inject[*sql.DB]()
	userService := provider.Inject[*user.Service]()
	postService := provider.Inject[*post.Service]()
	provider.Constructor(func() *Service { return &Service{db: db, userService: userService, postService: postService} })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DatabaseModule, user.Module, post.Module)
	module.Providers(Provider)
	module.Controllers(Controller)
})
