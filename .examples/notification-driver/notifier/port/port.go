// Package notifier is the port: a single Notifier interface (this file)
// plus a module picker (module.go) that wires ONE real adapter --
// notifier/email or notifier/sms -- based on a config value the caller
// passes in. Consumers (controller.go in the parent package) inject
// *only* Notifier -- they never know or care which adapter actually
// answers.
package port

// Type is the ONLY type consumer code ever depends on. email.Provider_
// and sms.Provider_ both satisfy it structurally -- neither adapter package
// imports this one (see module.go's doc comment for why that matters).
type Notifier interface {
	Send(to, message string) error
}
