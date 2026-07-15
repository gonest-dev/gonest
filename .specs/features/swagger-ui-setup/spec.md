# Swagger UI Setup Specification

## Problem Statement

Last piece of Milestone 7: `gonest.SetupSwagger(app, uiPath, doc, options)` (INSIGHT.md's own bootstrap example already specifies the full call shape) serves the already-generated `*OpenAPI` (via `doc.Document()`, already `json.Marshal`-able from "Schema Generation from Metadata") over HTTP -- a JSON endpoint plus a Swagger UI HTML page. This feature is purely SERVING/WIRING, no new generation logic.

## Goals

- [x] `Context` gains a way to send a raw HTML response (infra gap, same class as `Body()`/`Queries()` additions in prior features) -- `HTML(s string) error`
- [x] `gonest.SwaggerOptions{JsonDocumentUrl string, PersistAuth bool, DocExpansion string}` -- INSIGHT.md's exact field set, nothing more
- [x] `gonest.SetupSwagger(app *App, uiPath string, doc *OpenAPI, options SwaggerOptions)` -- registers TWO routes directly on the app's adapter (post-bootstrap, no DI/Module involvement -- same `app.Adapter().RegisterRoute(...)` mechanism `internal/app`'s own bootstrap already uses internally):
  - `GET {options.JsonDocumentUrl}` -- serves `doc.Document()` as JSON
  - `GET {uiPath}` -- serves an HTML page loading Swagger UI (via CDN, no vendored assets) configured to fetch the JSON from `options.JsonDocumentUrl`, with `persistAuthorization: options.PersistAuth` and `docExpansion: options.DocExpansion` baked into the UI's own JS initializer

## Out of Scope

| Feature                                                                                                                   | Reason                                                                                                                                                                 |
| ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Vendoring Swagger UI's static assets into the binary                                                                      | No requirement asks for offline/vendored assets; CDN-loaded HTML is simpler and matches how most Go OpenAPI libraries (e.g. `swaggo`) offer a CDN-based minimal option |
| Any `SwaggerOptions` field beyond `JsonDocumentUrl`/`PersistAuth`/`DocExpansion`                                          | INSIGHT.md's own bootstrap example is the only concrete requirement source                                                                                             |
| OAuth2/API-key-specific Swagger UI configuration beyond bearer auth (already covered by `Document()`'s `securitySchemes`) | Nothing asks for it                                                                                                                                                    |

---

## User Stories

### P1: `SetupSwagger`, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: As a gonest user, I want `gonest.SetupSwagger(app, config.OpenApi.UiPath, doc, gonest.SwaggerOptions{JsonDocumentUrl: config.OpenApi.JsonPath, PersistAuth: true, DocExpansion: "none"})` (INSIGHT.md's own bootstrap example) to serve a working Swagger UI page and JSON document.

**Acceptance Criteria**:

1. WHEN `SetupSwagger` is called THEN a `GET` request to `options.JsonDocumentUrl` SHALL return `doc.Document()` as JSON (status 200, `Content-Type: application/json`)
2. WHEN `SetupSwagger` is called THEN a `GET` request to `uiPath` SHALL return an HTML page (status 200, `Content-Type: text/html`) whose content references `options.JsonDocumentUrl` as the spec URL, and embeds `options.PersistAuth`/`options.DocExpansion` into the Swagger UI JS initializer
3. WHEN `SetupSwagger` is called BEFORE `app.MustListen`/`Listen` (INSIGHT.md's own bootstrap ordering) THEN both routes SHALL be reachable once the server starts (routes registered directly on the adapter, no dependency on `NewApp`'s own DI-driven route registration having "room" left)

**Independent Test**: real HTTP dispatch (`app.Test`) to both routes after `SetupSwagger`; assert JSON route returns valid, `Document()`-shaped JSON; assert HTML route returns non-empty HTML containing `options.JsonDocumentUrl` and the configured `docExpansion` value somewhere in the response body.

---

## Edge Cases

- WHEN `doc` has zero routes/schemas generated (e.g. `Generate` was never called, or the app has no controllers) THEN `SetupSwagger`'s JSON endpoint SHALL still return a valid (if mostly-empty) OpenAPI document shape -- no panic, no special-casing
- WHEN `uiPath` and `options.JsonDocumentUrl` collide with an already-registered application route THEN behavior is whatever the underlying `RegisterRoute`/adapter already does for a path collision (not defended against specially here -- same "trust the caller" stance as elsewhere)

---

## Requirement Traceability

| Requirement ID | Story                                                                     | Phase | Status |
| -------------- | ------------------------------------------------------------------------- | ----- | ------ |
| SW-01          | P1: Context.HTML sends raw HTML response                                  | Done  | Done   |
| SW-02          | P1: SetupSwagger registers JSON endpoint serving doc.Document()           | Done  | Done   |
| SW-03          | P1: SetupSwagger registers UI endpoint serving configured Swagger UI HTML | Done  | Done   |

**ID format:** `SW-[NUMBER]`

**Coverage:** 3 total, 3 mapped.

---

## Success Criteria

- [x] INSIGHT.md's `SetupSwagger` bootstrap call reproduced end-to-end via real HTTP dispatch, both routes working
- [x] Zero regressions in existing test suite
