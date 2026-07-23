package app

import (
	"fmt"

	"gonest.dev/gonest/internal/module"
)

// isProviderAsView is satisfied by *module.providerAsRef (unexported,
// internal/module/provider_as.go) via its exported IsProviderAsView()/
// InnerRef() methods (exported so cross-package type assertion works at
// all -- Go ties UNEXPORTED interface methods to the declaring package, see
// module.providerAsRef's own doc comment). Declared here -- not imported --
// for the same cross-package reason this file's own declarable interface
// (and internal/resolver/stage3.go's own copy of this exact marker) are:
// each package needs its own local copy of the shape to type-assert
// against it.
type isProviderAsView interface {
	IsProviderAsView()
	InnerRef() module.ProviderRef
}

// validateProviderAsRefs walks every module's OwnProviders(), checking
// each ProviderAs view (see isProviderAsView above) against the ref it
// wraps: the wrapped ref's ResolvedType() must be non-nil (it must have
// been separately registered via Providers on SOME module -- and Declared,
// Stage 2 -- see this feature's (provider-interface-export) design.md
// "Validation timing" Tech Decision for why this check cannot run inside
// ProviderAs itself) and must implement the view's own target interface.
//
// Called from NewApp between declareProviders (Stage 2, so every
// registered provider's ResolvedType() is reliable) and resolver.Resolve
// (Stage 3) -- fails at NewApp/MustNewApp call time, before any request.
func validateProviderAsRefs(modules []*module.Module) error {
	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			view, ok := p.(isProviderAsView)
			if !ok {
				continue
			}

			target := p.ResolvedType()
			inner := view.InnerRef()
			innerType := inner.ResolvedType()

			if innerType == nil {
				return fmt.Errorf("gonest: ProviderAs[%s] wraps a provider that was never registered via its own Providers(...) call -- the wrapped concrete ProviderRef needs its own Providers(...) registration, in addition to being wrapped", target)
			}
			if !innerType.Implements(target) {
				return fmt.Errorf("gonest: ProviderAs[%s] wraps a provider of type %s, which does not implement %s", target, innerType, target)
			}
		}
	}

	return nil
}
