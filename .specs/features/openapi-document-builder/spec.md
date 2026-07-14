# OpenAPI Document Builder Specification

## Problem Statement

Milestone 7's first, self-contained piece: `gonest.NewOpenApiDocument(version, fn)` builds a document-LEVEL metadata object (title, description, version, contact, license, bearer auth) with zero dependency on `internal/metadata`/`internal/route` -- it's a standalone builder, same shape INSIGHT.md's own bootstrap example (`# exemplo de bootstrap completo`) already fully specifies. `SetupSwagger` (serving it) and actual schema generation from routes/metadata are SEPARATE features (this milestone's other 2 ROADMAP items) -- this feature is ONLY the document builder itself.

## Goals

- [ ] `gonest.NewOpenApiDocument(version string, fn func(b *OpenApiDocument)) *OpenApiDocument` -- INSIGHT.md's own call shape (`gonest.NewOpenApiDocument("3.1.0", func (b *gonest.OpenApiDocument) {...})`)
- [ ] `OpenApiDocument.Title(s string)`, `Description(s string)`, `Version(s string)` -- basic metadata setters
- [ ] `OpenApiDocument.Contact(name, url, email string)` -- INSIGHT.md's exact call shape (3 positional strings, not a struct)
- [ ] `OpenApiDocument.License(name, url string)` -- INSIGHT.md's exact call shape (2 positional strings)
- [ ] `OpenApiDocument.BearerAuth()` -- zero-arg, marks the document as using bearer token auth (OpenAPI's `securitySchemes`)
- [ ] Getters for every setter (`TitleText()`, `DescriptionText()`, `VersionText()`, `ContactInfo()`, `LicenseInfo()`, `HasBearerAuth()`) -- same "setter/getter split, no method overloading" convention already established throughout `internal/metadata`

## Out of Scope

| Feature | Reason |
| --- | --- |
| `SetupSwagger`, `SwaggerOptions` (serving the document, UI setup) | Separate ROADMAP feature ("Swagger UI Setup"), depends on this one existing first |
| Generating `paths`/`schemas` from registered routes/`Metadata` | Separate ROADMAP feature ("Schema Generation from Metadata") -- requires deciding how routes link to request/response schemas, an open design question not yet discussed with the user |
| Actual OpenAPI 3.1 JSON/YAML SERIALIZATION of the document | This feature only builds the in-memory `*OpenApiDocument` -- turning it into the wire format is "Schema Generation from Metadata"'s concern (it needs paths/schemas to produce a useful document anyway, no reason to serialize an empty shell here) |
| Additional OpenAPI document fields beyond what INSIGHT.md shows (e.g. `servers`, `tags`, multiple security schemes) | INSIGHT.md's own bootstrap example is the only concrete requirement source; adding fields nothing asks for is speculative scope creep |

---

## User Stories

### P1: Document builder, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewOpenApiDocument("3.1.0", func (b *gonest.OpenApiDocument) { b.Title(...); b.Description(...); b.Version(...); b.Contact(...); b.License(...); b.BearerAuth() })` (INSIGHT.md's own bootstrap example) to compile and store every field correctly.

**Acceptance Criteria**:

1. WHEN `NewOpenApiDocument(version, fn)` is called THEN system SHALL construct a `*OpenApiDocument`, store `version` (the OpenAPI SPEC version, e.g. `"3.1.0"` -- distinct from `Version(s)`, which sets the API's OWN version per INSIGHT.md's `b.Version(config.OpenApi.Version)`), run `fn(doc)`, and return `doc`
2. WHEN `Title`/`Description`/`Version` are called THEN system SHALL store each string, retrievable via its own getter
3. WHEN `Contact(name, url, email)` is called THEN system SHALL store all 3 values, retrievable together via `ContactInfo()`
4. WHEN `License(name, url)` is called THEN system SHALL store both values, retrievable via `LicenseInfo()`
5. WHEN `BearerAuth()` is called THEN system SHALL mark `HasBearerAuth() == true` (default `false` if never called)

**Independent Test**: reproduce INSIGHT.md's bootstrap example verbatim (all 6 builder calls), assert every getter returns exactly what was set, plus the OpenAPI spec version string passed to `NewOpenApiDocument` itself.

---

## Edge Cases

- WHEN a setter (`Title`/`Contact`/etc) is called MORE THAN ONCE THEN system SHALL overwrite (last-write-wins, same precedent as every branch method throughout `internal/metadata`)
- WHEN `BearerAuth()` is never called THEN `HasBearerAuth()` SHALL report `false` -- no auth scheme by default

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| OD-01 | P1: NewOpenApiDocument constructs, runs fn, returns doc | Done | Done |
| OD-02 | P1: Title/Description/Version store+retrieve | Done | Done |
| OD-03 | P1: Contact(name,url,email) store+retrieve | Done | Done |
| OD-04 | P1: License(name,url) store+retrieve | Done | Done |
| OD-05 | P1: BearerAuth() sets flag, false by default | Done | Done |

**ID format:** `OD-[NUMBER]`

**Coverage:** 5 total, 5 mapped.

---

## Success Criteria

- [x] INSIGHT.md's bootstrap example (`NewOpenApiDocument` block) compiles and stores every field correctly
- [x] Zero regressions in existing test suite (commit `9b08afd`, evaluator PASS)
