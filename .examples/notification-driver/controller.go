package main

import (
	"net/http"

	"notification-driver/notifier"

	"gonest.dev/gonest"
)

type sendBody struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

var sendBodySchema = gonest.NewSchema(func(t *sendBody, s *gonest.Schema) {
	s.Property(&t.To).String().Required()
	s.Property(&t.Message).String().Required()
})

// NotificationController_ depends on notifier.Notifier ONLY -- it has no
// idea whether NOTIFICATION_DRIVER resolved to "email" or "sms" (see
// module.go). Same handler code, 2 different real backends, decided once
// at bootstrap from an env var.
var NotificationController_ = gonest.NewController(func(controller *gonest.Controller) {
	controller.Path("/notifications")

	notify := gonest.MustInject[notifier.Port](controller)

	controller.Route(gonest.HttpPost, "/", func(r *gonest.Route) {
		r.Summary("Send a notification through whichever driver is active")
		r.RequestBody(sendBodySchema)

		r.Handler(func(req *gonest.Request, res *gonest.Response) {
			body := gonest.MustParse[sendBody](req.Body().Json(), sendBodySchema)

			if err := notify.Send(body.To, body.Message); err != nil {
				res.Status(http.StatusInternalServerError).Json(map[string]any{"error": err.Error()})
				return
			}
			res.Status(http.StatusAccepted).Json(map[string]any{"driver": config.Driver, "sent": true})
		})
	})
})
