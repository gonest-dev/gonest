# Schema Generation from Metadata Context

User decisions captured via INSIGHT.md metacode iteration (2026-07-14, same pattern as "Param, Query & Custom Validation" -- user writes/edits a draft directly in INSIGHT.md, orchestrator asks follow-up questions inline in the file). See INSIGHT.md's "# dúvida: Schema Generation from Metadata" section (now answered) for the raw exchange.

## Decision 1: `@nestjs/swagger` decorator → gonest builder method mapping (confirmed)

| NestJS decorator | gonest equivalent |
| --- | --- |
| `@ApiTags(...)` | `Controller.Tags(...)` (inherited by every route) / `Route.Tags(...)` (override) |
| `@ApiOperation({summary, description, operationId})` | `Route.Summary(s)` / `Route.Description(s)` / `Route.OperationId(s)` |
| `@ApiBody({type})` | `Route.RequestBody(m *metadata.Metadata)` |
| `@ApiResponse`/`@ApiOkResponse`/`@ApiCreatedResponse`/etc | `Route.Response(status int, m ...*metadata.Metadata)` |
| `@ApiParam(...)` | `Route.PathParams(m *metadata.Metadata)` |
| `@ApiQuery(...)` | `Route.QueryParams(m *metadata.Metadata)` |
| `@ApiBearerAuth()`/`@ApiBasicAuth()` | `Controller.BearerAuth()` (inherited) / `Route.BearerAuth()` (override) |
| `@ApiExcludeEndpoint()` | `Route.ExcludeFromDocs()` |
| `@ApiDeprecated()` | `Route.Deprecated()` |
| `@ApiProperty(...)` | Already covered -- `Property()`/`Description()`/`Examples()`/`Required()` on `Metadata`, no new API |

**Override semantics (user confirmed)**: Route-level `Tags`/`BearerAuth` REPLACE the controller-level value entirely when set (route wins, does NOT merge/append) -- matches NestJS's own override behavior.

## Decision 2: `Response(status, metadata ...*Metadata)` -- variadic, optional

**Chosen**: `metadata` is variadic (`...*metadata.Metadata`), so `Response(status)` alone documents a status with NO body (equivalent to what the earlier draft called `Response(status, nil)`), and `Response(status, m)` documents a status WITH a body schema. No separate `ResponseNoBody(status)` method needed.

## Decision 3: Schema naming in `components.schemas`

**Chosen**: default name is the Go struct's own type name (`reflect.Type.Name()`, e.g. `"UserRequest"`). Overridable via a NEW `Metadata.Title(s string)`/`TitleText() string` method pair -- whole-type level (same tier as `Metadata.Description`/`DescriptionText`, NOT per-property), using OpenAPI's own vocabulary (`title` is a real OpenAPI Schema Object field) rather than inventing a gonest-specific name. When `TitleText() != ""`, it wins over the Go type name for the `components.schemas` key AND the schema's own `"title"` field.

**Rejected**: a struct tag-based override (`openapi:"id:..."`) -- user mentioned this as a possibility but the `Title()` method achieves the same result using a name that already means something in OpenAPI's own spec, consistent with how every other document-level string (`Description`) is already a method, not a tag.

## Decision 4: Undocumented routes still appear in the generated `paths`

**Chosen**: a route with NO `Summary`/`RequestBody`/`Response`/`PathParams`/`QueryParams` declared is NOT excluded from the generated OpenAPI document -- it appears using whatever CAN be inferred from what's already known regardless (HTTP method, path, default status code via `Route.Code()`). Only `Route.ExcludeFromDocs()` explicitly removes a route from the output.

## Discovery: `App` needs a NEW accessor to expose the module tree (prerequisite, blocking)

**Problem found while designing** (not from INSIGHT.md, a pure code-investigation finding): `Module.OwnControllers()` and `Controller.OwnRoutes()` already retain the full, builder-set object graph (`*route.Route` with every field this feature needs) for the app's entire lifetime -- but `App` (`internal/app/app.go`) has no public way to reach the ROOT `*module.Module` it stored internally (`root` field, unexported). `internal/app`'s own route-registration stage (`registerRoutes`) walks the tree, extracts only 3 primitives per route (method, path, handler closure) for the HTTP adapter, and discards its own references immediately after -- nothing durable survives that specific walk. The rich object graph DOES survive (via `Module`/`Controller`'s own `Own*()` accessors), just isn't reachable from outside `internal/app` today.

**Resolution**: `App` gains a new accessor (name TBD in design.md, likely `Root() *module.Module` or similar) so a schema generator (or anything else needing to walk the full app) can start from there and recurse via `Module.ImportedModules()` (already exists) + `Module.OwnControllers()` + `Controller.OwnRoutes()` (both already exist).

## Scope boundary

"Schema Generation from Metadata" (this feature) builds the WALKING + SCHEMA-BUILDING mechanism and the new `Route`/`Controller`/`Metadata` documentation-builder methods, producing the `paths`/`components.schemas` portions of an `*OpenApiDocument` (which "OpenAPI Document Builder", already shipped, only ever populated with document-LEVEL fields -- Title/Version/Contact/etc). Actually SERVING the document (`SetupSwagger`, Swagger UI HTML) is the separate "Swagger UI Setup" ROADMAP feature, built after this one.
