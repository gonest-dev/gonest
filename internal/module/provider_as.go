package module

import (
	"fmt"
	"reflect"
)

// hasResolvedValue is a locally-declared duck-typed interface matching
// *provider.Provider's ResolvedValue() shape (internal/resolver's own
// resolvedGetter, structurally identical). Declared here -- not imported --
// for the same cross-package reason every other duck-typed marker in this
// codebase is declared locally: module.ProviderRef itself does not expose
// ResolvedValue, so providerAsRef needs its own local check to decide
// whether the ref it wraps has one to delegate to.
type hasResolvedValue interface {
	ResolvedValue() (reflect.Value, bool)
}

// providerAsRef wraps an existing ProviderRef, reporting a caller-chosen
// interface type as its own ResolvedType() while delegating actual value
// resolution to the wrapped ref. It is a thin, read-only VIEW: it never
// drives Stage 3 construction itself (see internal/resolver/stage3.go's
// allProviders, which excludes any IsProviderAsView from the construction
// pass) -- only the wrapped ref's own registration does that. See this
// feature's (provider-interface-export) design.md Components section for
// the full rationale.
type providerAsRef struct {
	ref ProviderRef
	as  reflect.Type

	ownerModule *Module
}

// ProviderAs wraps ref, returning a ProviderRef that resolves as TInterface
// wherever registered (Providers and/or Exports) instead of ref's own
// concrete type. TInterface must be an interface kind -- panics immediately
// otherwise, since that check only inspects T itself and is safe to run at
// call time (typically package var-init). Whether ref's ACTUAL resolved
// type implements TInterface cannot be checked here (ref's ResolvedType()
// is not reliable until Stage 2's Declare has run) -- see
// internal/app/provider_as_validate.go for the deferred check.
//
// Chaining (wrapping a ProviderRef that is itself the result of another
// ProviderAs call) is rejected outright: a wrapper's own ResolvedType() is
// the interface it was constructed for, not the original concrete type, so
// "does the ORIGINAL concrete type implement this new interface" could not
// be answered later -- wrap the original concrete ProviderRef separately
// for each interface instead.
func ProviderAs[T any](ref ProviderRef) ProviderRef {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Interface {
		panic(fmt.Sprintf("gonest: ProviderAs[T] requires T to be an interface type, got %s", t))
	}
	if _, ok := ref.(isProviderAsView); ok {
		panic("gonest: ProviderAs cannot wrap another ProviderAs view -- wrap the original concrete ProviderRef for each interface instead")
	}

	return &providerAsRef{ref: ref, as: t}
}

// IsProviderAsView is the marker internal/app's validateProviderAsRefs and
// internal/resolver/stage3.go's allProviders type-assert against (their own
// local copies of this exact shape). EXPORTED -- unlike most of this
// codebase's marker methods (IsProvider, IsExportable, etc, which are
// exported for the same reason) -- because Go ties UNEXPORTED interface
// methods to the declaring package: an unexported isProviderAsView()
// defined here could never satisfy an unexported isProviderAsView()
// declared in internal/app or internal/resolver, even with an identical
// signature (see ProviderRef's own doc comment for the same rule, already
// established for this codebase's other cross-package marker interfaces).
// Not part of ProviderRef itself -- ordinary ProviderRef implementations
// (e.g. *provider.Provider) have no reason to implement it.
func (p *providerAsRef) IsProviderAsView() {}

// InnerRef returns the wrapped ProviderRef -- the accessor
// internal/app's validateProviderAsRefs needs to reach the wrapped ref's
// (post-Stage-2) ResolvedType() for the deferred implements-check.
// EXPORTED for the same cross-package reason IsProviderAsView is.
func (p *providerAsRef) InnerRef() ProviderRef {
	return p.ref
}

// IsProvider satisfies ProviderRef.
func (p *providerAsRef) IsProvider() {}

// IsExportable satisfies ExportableRef (via ProviderRef), so a
// providerAsRef can be passed to Module.Exports alongside plain providers
// and *Module re-export arguments.
func (p *providerAsRef) IsExportable() {}

// ResolvedType returns the interface type this view was constructed for
// (captured at ProviderAs[T] call time), NOT computed from the wrapped
// ref -- so it is valid immediately, unlike the wrapped ref's own
// ResolvedType() (nil until Stage 2's Declare has run for it).
func (p *providerAsRef) ResolvedType() reflect.Type {
	return p.as
}

// SetOwnerModule associates this view with the module that owns it. Own
// storage, independent of the wrapped ref's own ownership -- called by
// Stage 1 assembly like any other registered ProviderRef.
func (p *providerAsRef) SetOwnerModule(m *Module) {
	p.ownerModule = m
}

// OwnerModule returns the module this view was registered under.
func (p *providerAsRef) OwnerModule() *Module {
	return p.ownerModule
}

// ResolvedValue delegates to the wrapped ref if it exposes one (duck-typed
// against hasResolvedValue, same shape internal/resolver's own
// resolvedGetter expects) -- so internal/resolver's FindDirect/
// FindDirectAll pick up a providerAsRef with zero changes beyond deleting
// the Implements() fallback (see direct.go). Returns (zero Value, false)
// if the wrapped ref has no resolved value (yet, or ever).
func (p *providerAsRef) ResolvedValue() (reflect.Value, bool) {
	rv, ok := p.ref.(hasResolvedValue)
	if !ok {
		return reflect.Value{}, false
	}
	return rv.ResolvedValue()
}

// isProviderAsView is the local marker interface ProviderAs itself checks
// ref against, to reject chaining (ProviderAs wrapping another ProviderAs
// view). Structurally identical to -- but a distinct declaration from --
// internal/app's and internal/resolver's own local copies, per this
// codebase's established cross-package duck-typing convention. Matches
// against IsProviderAsView (exported, see its own doc comment for why).
type isProviderAsView interface {
	IsProviderAsView()
}
