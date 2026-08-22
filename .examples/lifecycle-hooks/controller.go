package main

import (
	"gonest.dev/gonest"
)

// StatusController exposes DbService.Ready() as JSON -- a real HTTP request
// proves OnApplicationBootstrap already ran by the time NewApp returned and
// Listen started serving (Ready is true on the very first request).
var StatusController = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/status")

	db := gonest.MustInject[*DbService](controller)

	controller.Route(gonest.HttpGet, "/", func(r *gonest.Route) {
		r.Handler(func(c *gonest.HttpContext) {
			res := c.Response()
			res.Json(map[string]bool{"ready": db.Ready()})
		})
	})
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(DbServiceProvider)
	module.Controllers(StatusController)
})
