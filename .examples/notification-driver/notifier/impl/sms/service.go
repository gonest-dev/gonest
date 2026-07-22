// Package sms is a Notifier adapter. It does NOT import
// notification-driver/notifier -- its Provider returns the concrete
// *Service type, and gonest's resolver matches MustInject[notifier.Notifier]
// against it structurally (reflect.Type.Implements()), see
// notifier/module.go's doc comment for why that matters here.
package sms

import (
	"fmt"
	"notification-driver/notifier/port"

	"gonest.dev/gonest"
)

// Service is a stand-in for a real SMS gateway client -- prints instead of
// sending, enough to prove which adapter actually answered a request.
type Service struct{}

var _ port.Notifier = (*Service)(nil)

func (s *Service) Send(to, message string) error {
	fmt.Printf("[sms] to=%s message=%q\n", to, message)
	return nil
}

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *Service { return &Service{} })
})
