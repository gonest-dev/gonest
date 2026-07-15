# Schema Generation from Metadata Tasks

**Design**: `.specs/features/schema-generation/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T0-T3, todos evaluator PASS)

---

## Execution Plan

```
T0 (App.Root() accessor)
  → T1 (Metadata.Title + Controller.Tags/BearerAuth + Route documentation builder methods -- mechanical, P1)
  → T2 (internal/openapi.Generate + recursive schemaFor + Document() -- the real logic, P2)
  → T3 (root re-exports + INSIGHT.md settle + end-to-end reproduction)
```

Sequencial. T0/T1 são mecânicos e de baixo risco (getters/setters aditivos, mesmo padrão AD-012 já repetido várias vezes) -- podem rodar com evaluator mais leve. T2 é o núcleo real (walker recursivo + geração de schema) -- evaluator dedicado, mais rigoroso.

---

## Task Breakdown

### T0: `App.Root()` accessor ✅ DONE (evaluator: PASS, commit `2ef60d2`)

**What**: `internal/app/app.go`'s `App` ganha `func (a *App) Root() *module.Module { return a.root }` -- `a.root` já existe (setado no bootstrap), é só expor.

**Where**: `internal/app/app.go` (existente, estendido), `internal/app/app_test.go` (existente, estendido)

**Depends on**: nenhuma

**Requirement**: SG-00

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] `App.Root()` devolve o mesmo `*Module` que o bootstrap montou -- teste com árvore de módulos (root importando sub-módulo com controller/route próprios), confirma que `Root()` + `ImportedModules()` + `OwnControllers()` + `Controller.OwnRoutes()` (todos já existentes) juntos alcançam TODA rota registrada, sem buraco
- [x] Gate check passa
- [x] Test count: 2+

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): add App.Root() accessor exposing the assembled module tree`

---

### T1: `Metadata.Title` + `Controller.Tags`/`BearerAuth` + `Route` documentation builder methods ✅ DONE (evaluator: PASS, commit `66aef7b`)

**What**: (ver design.md's Components pra assinatura completa de cada método)

1. `internal/metadata/metadata.go`'s `Metadata` ganha campo `title string` + `Title(s string) *Metadata` / `TitleText() string` (mesmo par setter/getter de `Description`/`DescriptionText`, MESMO nível -- whole-type, não `PropertyBuilder`)
2. `internal/controller/controller.go`'s `Controller` ganha `tags []string`, `bearerAuth bool` + `Tags(...string) *Controller` / `OwnTags() []string` (cópia defensiva) + `BearerAuth() *Controller` / `HasBearerAuth() bool`
3. `internal/route/route.go`'s `Route` ganha os 10 métodos de documentação (ver design.md's Components: `Summary`/`Description`/`OperationId`/`Tags`/`BearerAuth`/`RequestBody`/`Response`/`PathParams`/`QueryParams`/`ExcludeFromDocs`/`Deprecated`), cada um setter-retorna-`*Route` + getter próprio. `Route` ganha NOVA dependência de `internal/metadata` (confirmado sem ciclo -- metadata nunca importa route)

**Where**: `internal/metadata/metadata.go` (estendido), `internal/controller/controller.go` (estendido), `internal/route/route.go` (estendido), respectivos `_test.go`

**Depends on**: T0 (sequencial por simplicidade de dispatch, sem dependência real de código)

**Requirement**: SG-01, SG-02, SG-03

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] `Metadata.Title`/`TitleText` funciona, default `""` quando nunca chamado
- [x] `Controller.Tags`/`BearerAuth` armazenam corretamente, `OwnTags()` é cópia defensiva
- [x] `Route.Tags()`/`BearerAuth()` distinguem "nunca chamado" (herda do controller, resolvido em T2) de "chamado" (override) -- teste confirma o getter reporta os dois estados corretamente (não resolve herança aqui, só armazena o suficiente pra T2 resolver depois)
- [x] `Route.Response(status, m ...*Metadata)` com zero args documenta status sem body; com 1 arg documenta com body; chamado duas vezes pra status DIFERENTES acumula ambos; chamado duas vezes pro MESMO status sobrescreve
- [x] `Route.ExcludeFromDocs()`/`Deprecated()` armazenam flag, `false` por default
- [x] Gate check passa
- [x] Test count: 15+ (cobertura de cada método novo nos 3 arquivos)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(openapi): add Metadata.Title, Controller.Tags/BearerAuth, Route documentation builder methods`

---

### T2: `internal/openapi.Generate` + núcleo recursivo de schema + `Document()` ✅ DONE (evaluator: PASS, commits `325155c`/`7d4f915` -- gap de teste de nullable `$ref`/`anyOf` fechado; SPEC_DEVIATION #3 do dev era falsa (INSIGHT.md não estava desatualizado), corrigida aqui: era só escolha estilística do fixture de teste, não incompatibilidade real)

**What**: (ver design.md's Components/Data Models pra detalhe completo)

- `OpenAPI` ganha campos novos: `paths map[string]map[string]any`, `schemas map[string]any`, `schemaNames map[*metadata.Metadata]string`
- `func Generate(doc *OpenAPI, root *module.Module)` -- percorre `root` + `root.ImportedModules()` (recursivo, `visitedModules` pra evitar ciclo) + `Module.OwnControllers()` + `Controller.OwnRoutes()`, monta `doc.paths`/`doc.schemas`
- `walkController`/`walkRoute` (não-exportados) -- resolvem herança Tags/BearerAuth (controller vs override da rota, rota SEMPRE vence quando setada), montam path item por rota NÃO excluída (`ExcludeFromDocs()`)
- `schemaFor(p *metadata.PropertyBuilder, doc *OpenAPI, visiting map[*metadata.Metadata]bool) map[string]any` -- núcleo recursivo, dispatcha por `p.KindValue()`, cobre TODAS as branch families (String/Numeric/Boolean/DateTime-Date/Array/Object), usa `ItemRef()`/`MetadataRef()` pra `$ref` em vez de inline
- `registerSchema(m *metadata.Metadata, doc *OpenAPI, visiting map[*metadata.Metadata]bool) string` -- dedup via `doc.schemaNames` (pointer-keyed), nome default = nome do tipo Go, override via `TitleText()`
- `func (doc *OpenAPI) Document() map[string]any` -- monta estrutura OpenAPI 3.1 completa (info/paths/components/security) a partir de TODOS os campos do doc (já existentes + novos)

**Where**: `internal/openapi/generate.go` (novo), `internal/openapi/openapi.go` (estendido -- novos campos), `internal/openapi/generate_test.go` (novo)

**Depends on**: T0, T1

**Reuses**: `Module.ImportedModules()`/`OwnControllers()`, `Controller.OwnRoutes()`, `Metadata.OwnProperties()`, TODA a superfície de getters de `PropertyBuilder` (AD-012: `KindValue`/`MinValue`/`MaxValue`/`PatternValue`/`ItemBuilder`/`ItemRef`/`MetadataRef`/`IsAdditionalProperties`)

**Requirement**: SG-04, SG-05, SG-06

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] TODA rota registrada (root + módulos importados, recursivo) NÃO marcada `ExcludeFromDocs()` aparece em `paths`, chave = prefixo do controller + path da rota + método
- [x] Rota sem NENHUMA declaração de documentação ainda aparece em `paths` (método/path/status inferidos do que já existe em `Route`)
- [x] `*Metadata` referenciada de MÚLTIPLOS lugares aparece em `components.schemas` EXATAMENTE 1 vez, resto vira `$ref`
- [x] Schema correto pra CADA branch family: String (type+format+minLength/maxLength/pattern), Numeric (type+format+minimum/maximum), Boolean (type), DateTime/Date (type+format), Array (type+items, item inline OU `$ref`, minItems/maxItems), Object (`$ref` OU `additionalProperties:true`)
- [x] `Required`/`Nullable` refletidos corretamente no schema
- [x] Campo com `Custom(fn)` aparece no schema SEM type/format (só description/nullable/required se setados) -- limitação documentada, não erro
- [x] Tags/BearerAuth: herdado do controller quando rota não chamou os próprios; SUBSTITUÍDO quando rota chamou (nunca soma)
- [x] Reprodução do `UserEntity`/`AddressEntity` do INSIGHT.md (Array/Object aninhado, reuso de `addressMetadata`) -- schema completo e correto, dedup confirmado
- [x] `Document()` monta estrutura completa e é `json.Marshal`-ável sem erro
- [x] Ciclo de import de módulo (A importa B, B importa A) não causa loop infinito (`visitedModules`)
- [x] Gate check passa
- [x] Test count: 25+ (é o núcleo da feature, cobertura ampla necessária)

**Tests**: unit + integration
**Gate**: full

**Commit**: `feat(openapi): add Generate() -- walk module tree, build paths + components.schemas`

---

### T3: Root re-exports + INSIGHT.md settle + reprodução end-to-end ✅ DONE (evaluator: PASS, commits `0d20a9c`/`b27f7b2` -- fechou débito antigo, `gonest.Route` alias faltava desde sempre)

**What**: confirma `gonest.NewOpenAPI`/`OpenAPI` já re-exportados (T da feature anterior) ganham acesso aos novos métodos (`Document()`) automaticamente via alias. Adiciona `gonest.SetupSwagger`? NÃO -- isso é "Swagger UI Setup", feature separada; aqui só confirma que `openapi.Generate` está acessível de algum jeito pro dev chamar manualmente ainda que sem `SetupSwagger` (avaliar se precisa de um wrapper raiz tipo `gonest.GenerateOpenApiSchema(doc, app)` temporário, OU se isso fica pra "Swagger UI Setup" terminar de amarrar -- SE ficar pra depois, documentar isso claramente). Reescreve a seção "dúvida: Schema Generation from Metadata" do INSIGHT.md removendo o framing de dúvida, virando exemplo settled reproduzindo o fluxo completo (Controller.Tags/BearerAuth, Route.Summary/RequestBody/Response/PathParams/ExcludeFromDocs, Metadata.Title).

**Where**: `gonest.go` (se precisar de wrapper novo), `gonest_test.go` (novo teste), `INSIGHT.md` (reescrito)

**Depends on**: T2

**Requirement**: SG-01 até SG-06 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] Toda API nova (`Metadata.Title`, `Controller.Tags`/`BearerAuth`, `Route`'s 10 métodos) resolve na raiz via os aliases já existentes (`gonest.Metadata`/`gonest.Controller`/`gonest.Route` são todos aliases de tipo, então métodos novos aparecem automaticamente -- confirmar isso é verdade, não assumir)
- [x] `openapi.Generate` acessível de alguma forma na raiz (decidir wrapper mínimo se necessário)
- [x] Reprodução do INSIGHT.md via teste real (registra app com Controller documentado, chama Generate, confirma JSON gerado)
- [x] Gate check passa
- [x] Test count: 2+

**Tests**: integration
**Gate**: quick

**Commit**: `feat(openapi): finalize root access for schema generation, settle INSIGHT.md`

---

## Parallel Execution Map

```
Sequencial: T0 → T1 → T2 → T3
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate. T2 em particular: evaluator deve rodar suite completa + verificar cobertura de TODAS as branch families, não só um subset.

---

## Task Granularity Check

| Task                                                    | Scope                                                    | Status                                   |
| ------------------------------------------------------- | -------------------------------------------------------- | ---------------------------------------- |
| T0: App.Root()                                          | 1 método, trivial                                        | ✅ Granular                               |
| T1: Title/Tags/BearerAuth/documentation builder methods | 3 arquivos, mecânico (getters/setters aditivos)          | ✅ Granular                               |
| T2: Generate + schemaFor + Document()                   | 1 pacote, NÚCLEO real da feature, maior escopo aceitável | ✅ Granular (isolado, evaluator dedicado) |
| T3: Root cleanup + INSIGHT.md                           | Mecânico + reprodução                                    | ✅ Granular                               |

---

## Test Co-location Validation

| Task | Code Layer                 | Matrix Requires    | Task Says          | Status |
| ---- | -------------------------- | ------------------ | ------------------ | ------ |
| T0   | Accessor puro              | unit               | unit               | ✅ OK   |
| T1   | Getters/setters aditivos   | unit               | unit               | ✅ OK   |
| T2   | Walker + geração de schema | unit + integration | unit + integration | ✅ OK   |
| T3   | Re-export + reprodução E2E | integration        | integration        | ✅ OK   |

Nenhuma violação.
