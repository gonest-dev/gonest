// Package email is a Notifier adapter. It does NOT import
// notification-driver/notifier -- its Provider_ returns the concrete
// *Service type; module.go's AsNotifier_ explicitly registers it as
// port.Notifier via gonest.ProviderAs, see notifier/module.go's doc
// comment for why that matters here.
package email

import (
	"fmt"
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

// Service is a stand-in for a real SMTP/API client -- prints instead of
// sending, enough to prove which adapter actually answered a request.
type Service struct{}

var _ port.Notifier = (*Service)(nil)

func (s *Service) Send(to, message string) error {
	fmt.Printf("[email] to=%s message=%q\n", to, message)
	return nil
}

var Provider_ = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *Service { return &Service{} })
})
