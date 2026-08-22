package main

import (
	"gonest.dev/gonest"
)

// ConfigController exposes the resolved *DatabaseConfig as JSON so a real
// HTTP request can PROVE the .env -> struct flow ran (not just compiled) --
// see main.go's Try section.
var ConfigController = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/config")

	config := gonest.MustInject[*DatabaseConfig](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Handler(func(c *gonest.HttpContext) {
			res := c.Response()
			res.Json(config)
		})
	})
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(DatabaseConfigProvider)
	module.Controllers(ConfigController)
})
