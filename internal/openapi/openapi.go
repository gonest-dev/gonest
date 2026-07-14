// Package openapi is Milestone 7's first, self-contained piece: a
// standalone document-LEVEL builder (title, description, version, contact,
// license, bearer auth) with zero dependency on internal/metadata or
// internal/route -- see .specs/features/openapi-document-builder/spec.md's
// Problem Statement. It reproduces INSIGHT.md's own bootstrap example
// verbatim (the "# exemplo de bootstrap completo" section's
// gonest.NewOpenApiDocument("3.1.0", func (b *gonest.OpenApiDocument) {...})
// call shape).
//
// This package only builds the in-memory *OpenApiDocument -- serving it
// (SetupSwagger/SwaggerOptions) and generating paths/schemas from registered
// routes/Metadata are separate ROADMAP features (spec.md's Out of Scope).
package openapi

import "github.com/gonest-dev/gonest/internal/metadata"

// OpenApiDocument holds every document-level field INSIGHT.md's bootstrap
// example sets. specVersion is the OpenAPI SPEC version passed to New (e.g.
// "3.1.0") -- distinct from version, which is the API's OWN version set via
// Version(s) (spec.md AC1).
type OpenApiDocument struct {
	specVersion string

	title       string
	description string
	version     string

	contactName  string
	contactUrl   string
	contactEmail string

	licenseName string
	licenseUrl  string

	bearerAuth bool

	// paths/schemas/schemaNames (schema-generation feature, P2) are
	// populated ONLY by Generate, never by hand-written builder calls --
	// Title/Contact/etc above stay exactly as they are. paths: outer key is
	// the full path string (Controller.PathPrefix()+Route.Path()), inner key
	// is the lowercase HTTP method, value is one OpenAPI Path Item Object.
	// schemas: components.schemas, keyed by schema name. schemaNames is the
	// dedup cache (design.md's Data Models) -- pointer-keyed so the SAME
	// *metadata.Metadata, referenced from multiple places, is walked and
	// registered exactly once.
	paths       map[string]map[string]any
	schemas     map[string]any
	schemaNames map[*metadata.Metadata]string
}

// New constructs a zero-value *OpenApiDocument, stores specVersion, runs fn
// against it (if non-nil), and returns it -- spec.md OD-01. fn receives the
// SAME pointer New returns (pointer identity), mirroring the deferred-fn
// builder shape used throughout this codebase (e.g. internal/module.New).
func New(specVersion string, fn func(b *OpenApiDocument)) *OpenApiDocument {
	b := &OpenApiDocument{specVersion: specVersion}
	if fn != nil {
		fn(b)
	}
	return b
}

// SpecVersion returns the OpenAPI spec version passed to New (e.g.
// "3.1.0") -- distinct from VersionText, which returns the API's OWN
// version set via Version(s).
func (b *OpenApiDocument) SpecVersion() string {
	return b.specVersion
}

// Title sets the document's title and returns b so calls can chain.
// Last-write-wins on repeat calls, same precedent as every branch method
// throughout internal/metadata.
func (b *OpenApiDocument) Title(s string) *OpenApiDocument {
	b.title = s
	return b
}

// TitleText returns the title set via Title, or "" if it was never called.
// Named differently from the setter because Go has no method overloading --
// same setter/getter split established by internal/metadata.Metadata's
// Description/DescriptionText.
func (b *OpenApiDocument) TitleText() string {
	return b.title
}

// Description sets the document's description and returns b so calls can
// chain. Last-write-wins on repeat calls.
func (b *OpenApiDocument) Description(s string) *OpenApiDocument {
	b.description = s
	return b
}

// DescriptionText returns the description set via Description, or "" if it
// was never called.
func (b *OpenApiDocument) DescriptionText() string {
	return b.description
}

// Version sets the API's OWN version (distinct from the OpenAPI spec
// version passed to New, see SpecVersion) and returns b so calls can chain.
// Last-write-wins on repeat calls.
func (b *OpenApiDocument) Version(s string) *OpenApiDocument {
	b.version = s
	return b
}

// VersionText returns the version set via Version, or "" if it was never
// called.
func (b *OpenApiDocument) VersionText() string {
	return b.version
}

// Contact sets the document's contact name/url/email (INSIGHT.md's exact
// call shape: 3 positional strings, not a struct) and returns b so calls
// can chain. Last-write-wins on repeat calls -- all 3 values overwritten
// together.
func (b *OpenApiDocument) Contact(name, url, email string) *OpenApiDocument {
	b.contactName = name
	b.contactUrl = url
	b.contactEmail = email
	return b
}

// ContactInfo returns the name/url/email set via Contact, all "" if it was
// never called.
func (b *OpenApiDocument) ContactInfo() (name, url, email string) {
	return b.contactName, b.contactUrl, b.contactEmail
}

// License sets the document's license name/url (INSIGHT.md's exact call
// shape: 2 positional strings) and returns b so calls can chain.
// Last-write-wins on repeat calls -- both values overwritten together.
func (b *OpenApiDocument) License(name, url string) *OpenApiDocument {
	b.licenseName = name
	b.licenseUrl = url
	return b
}

// LicenseInfo returns the name/url set via License, both "" if it was never
// called.
func (b *OpenApiDocument) LicenseInfo() (name, url string) {
	return b.licenseName, b.licenseUrl
}

// BearerAuth marks the document as using bearer token auth (OpenAPI's
// securitySchemes) and returns b so calls can chain. Zero-arg, per spec.md
// OD-05.
func (b *OpenApiDocument) BearerAuth() *OpenApiDocument {
	b.bearerAuth = true
	return b
}

// HasBearerAuth reports whether BearerAuth was ever called -- false by
// default (spec.md's Edge Cases: "no auth scheme by default").
func (b *OpenApiDocument) HasBearerAuth() bool {
	return b.bearerAuth
}
