package comment

import (
	"database/sql"

	"github.com/gonest-dev/gonest"

	"blog-api/post"
	"blog-api/shared"
	"blog-api/user"
)

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	db := gonest.MustInject[*sql.DB](provider)
	userService := gonest.MustInject[*user.Service](provider)
	postService := gonest.MustInject[*post.Service](provider)
	provider.Constructor(func() *Service { return &Service{db: db, userService: userService, postService: postService} })
})

var Module = gonest.NewModule(func(module *gonest.Module) {
	module.Imports(shared.DatabaseModule, user.Module, post.Module)
	module.Providers(Provider)
	module.Controllers(Controller)
})
