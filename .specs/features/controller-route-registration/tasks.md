# Controller & Route Registration Tasks

**Design**: `.specs/features/controller-route-registration/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

### Phase 1: Foundation (Parallel OK — sem dependência real entre si)

```
T1 [P] (go.mod + HttpMethod)
T2 [P] (Context shell)
```

### Phase 2: Independent Types (Parallel OK — pacotes diferentes)

```
      ┌→ T3 (internal/httpctx) ─┐
T2 ───┤                          ├──→ T5 (internal/route)
      └→ T4 (internal/pipe) ────┘
```

### Phase 3: Integration (Sequential)

```
T5 → T6 → T7 → T8 → T9
```

---

## Task Breakdown

### T1: `go get` Fiber v3 + `HttpMethod` enum ✅ DONE (evaluator: PASS, commit `e7c277b`)

**What**: adiciona `github.com/gofiber/fiber/v3` ao `go.mod`; define `HttpMethod` (`HttpGet`/`HttpPost`/`HttpPut`/`HttpDelete`/`HttpQuery` já usado no INSIGHT.md) em `internal/route`.
**Where**: `go.mod`, `go.sum`, `internal/route/method.go`
**Depends on**: None
**Reuses**: —
**Requirement**: infra

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `go get github.com/gofiber/fiber/v3` resolve sem erro
- [ ] `HttpMethod` com os 5 valores + `String()` (debug-friendly, mesmo padrão de `Scope`)
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 5+ (1 por valor de `String()`)

**Tests**: unit
**Gate**: quick

**Commit**: `chore(http): add Fiber v3 dependency and HttpMethod enum`

---

### T2: `Context` — shell mínimo (sem Fiber ainda) ✅ DONE (evaluator: PASS, commit `c767832`)

**What**: `internal/httpctx.Context` — struct que por dentro guarda uma interface mínima `responder` (não o `fiber.Ctx` direto ainda — isola o resto do pacote de Fiber até T7/T8) com `Json`/`Status`/`Header`/`SetHeader`/`Param`. Pra ESTA task, teste com uma implementação fake de `responder` (Fiber real só entra em T7).
**Where**: `internal/httpctx/context.go`, `internal/httpctx/context_test.go`
**Depends on**: None
**Reuses**: —
**Requirement**: CTRL-02 (parte)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Context.Json(value)` delega pro `responder` fake nos testes
- [ ] `Context.Status(code)` retorna `*Context` (chainable)
- [ ] `Context.Header`/`SetHeader`/`Param` funcionam contra o fake
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 5+ (Json, Status chainable, Header, SetHeader, Param)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): add Context shell with fake responder for testing`

---

### T3: `Pipe` — validação de assinatura via reflect [P] ✅ DONE (evaluator: PASS, commit `3cb48c6`)

**What**: `pipe.New(fn)` guarda `fn` sem executar; `Pipe.Handler(fn any)` reflect-valida assinatura `func(ctx *httpctx.Context, raw string) T`, panica em tempo de declaração se não bater (mesmo padrão de `isValidConstructorSignature` do `internal/provider`).
**Where**: `internal/pipe/pipe.go`, `internal/pipe/pipe_test.go`
**Depends on**: T2 (precisa de `*httpctx.Context` no tipo da assinatura validada)
**Reuses**: padrão de reflect-validação de `internal/provider/provider.go`
**Requirement**: CTRL-05, CTRL-06 (parte)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `pipe.New(fn)` não executa `fn` na chamada
- [ ] `Handler` aceita `func(ctx *httpctx.Context, raw string) T` pra qualquer `T`, panica com mensagem clara se assinatura não bater
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 3+ (fn adiado, assinatura válida, assinatura inválida)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): add Pipe with reflect-validated Handler signature`

---

### T4: `MustParam[T]` — coerção default via reflect+strconv [P] ✅ DONE (evaluator: PASS, commit `47afc26`)

**What**: função genérica que converte `ctx.Param(name)` (string) pro tipo `T` pedido via reflect+strconv (`string`/`int`/`int64`/`bool`/`float64`), panic claro se não converter ou se o param não existir. Fica isolada de `Route`/`Pipe` nessa task — a integração "se a rota tem Pipe customizado, usa ele em vez do default" acontece em T5.
**Where**: `internal/route/param.go` (função `defaultCoerce[T](raw string) (T, error)`, sem depender de `*Route` ainda), `internal/route/param_test.go`
**Depends on**: T2 (precisa de `*httpctx.Context` pra `ctx.Param`)
**Reuses**: —
**Requirement**: CTRL-04, CTRL-06 (parte)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Converte pros 5 tipos básicos corretamente
- [ ] Panic claro quando o valor não converte pro tipo pedido
- [ ] Panic claro quando o param não existe no Context (via `ctx.Param` devolvendo indicação de ausência — decidir formato exato na implementação)
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 7+ (5 tipos válidos, 1 conversão inválida, 1 param ausente)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): add default param coercion via reflect+strconv`

---

### T5: `Route` — builder + integração Pipe/MustParam ✅ DONE (evaluator: PASS-WITH-NOTE, commit `96cbb4c`)

**What**: `route.New(method, path, fn)` guarda `fn`, roda IMEDIATAMENTE quando chamado (diferente de Provider/Module — nesse ponto o módulo já está resolvido, é só Stage 2 populando struct, ver design.md). `Route.HttpCode(status)`, `Route.Param(name, pipe)` (registra Pipe customizado), `Route.Handler(fn func(ctx *httpctx.Context))`. `MustParam[T](ctx, name)` (wrapper público em `param.go` na raiz) agora checa se a rota atual tem Pipe customizado pro param e usa ele, senão cai no `defaultCoerce` de T4.
**Where**: `internal/route/route.go`, `internal/route/route_test.go`, `param.go` na raiz (wrapper `MustParam[T]`)
**Depends on**: T3, T4
**Reuses**: `pipe.Pipe` de T3, `defaultCoerce` de T4
**Requirement**: CTRL-01 (parte declarativa), CTRL-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Route.New(method, path, fn)` roda `fn` na hora (não adia)
- [ ] `HttpCode`/`Handler` armazenam corretamente
- [ ] `Route.Param(name, customPipe)` faz `MustParam[T]` usar o Pipe customizado em vez do default pra esse param específico
- [ ] Sem Pipe customizado, `MustParam[T]` usa `defaultCoerce` (T4)
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 6+ (fn roda na hora, HttpCode, Handler, Param com Pipe customizado, MustParam sem Pipe usa default, MustParam com Pipe usa customizado)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): add Route builder wired to Pipe and MustParam`

---

### T6: `Controller` estendido — Path/Route/Use/Guards/Interceptors/Filters (stubs) ✅ DONE (evaluator: PASS, commit `4e99255`)

**What**: adiciona ao `Controller` já existente (T9 da feature DI Graph): `Path(prefix)`, `Route(method, path, fn)` (cria `*route.Route`, guarda em slice interna), `OwnRoutes() []*route.Route` (accessor, mesmo padrão de `OwnProviders`), e os 4 stubs `Use`/`Guards`/`Interceptors`/`Filters` (tipos placeholder mínimos — `type Middleware struct{}` ou similar, decisão de quem implementar; só armazenam, nada lê ainda).
**Where**: `internal/controller/controller.go` (arquivo existente, estendido), `internal/controller/controller_test.go`
**Depends on**: T5
**Reuses**: `route.Route` de T5, padrão `Declare()` já existente
**Requirement**: CTRL-01 (parte), CTRL-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Controller.Path(prefix)` armazena o prefixo
- [x] `Controller.Route(...)` cria a rota e guarda na lista interna
- [x] `OwnRoutes()` devolve cópia defensiva (mesmo padrão de `OwnProviders`)
- [x] `Use`/`Guards`/`Interceptors`/`Filters` armazenam sem afetar nada (teste prova no-op: controller com e sem essas chamadas se comporta igual)
- [x] Gate check passa (comando abaixo)
- [x] Test count: 5+ (Path, Route+OwnRoutes, cada um dos 4 stubs comprovadamente no-op — pode agrupar num teste só se ficar coeso)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): extend Controller with Path/Route and pipeline stubs`

---

### T7: `internal/fiberapp` — adapter real ✅ DONE (evaluator: PASS, commit `53cd63f`)

**What**: `FiberApp` — único pacote (além de `internal/httpctx`) que importa Fiber de verdade. Implementa o contrato mínimo (`RegisterRoute(method HttpMethod, path string, h func(*httpctx.Context)) error`, `Listen(addr string) error`). O wrapper registrado no Fiber roda o `Handler` do gonest dentro de `recover()` próprio — panic vira `c.Status(500).SendString(...)`, nunca usa o error-return nativo do Fiber nem o middleware `recover` dele (ver design.md Tech Decisions).
**Where**: `internal/fiberapp/fiberapp.go`, `internal/fiberapp/fiberapp_test.go`
**Depends on**: T6
**Reuses**: `internal/httpctx.Context`
**Requirement**: CTRL-01, CTRL-03

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `RegisterRoute` registra rota real no `fiber.App` interno
- [x] Request real via `app.Test(req)` (helper do próprio Fiber, sem subir porta) bate na rota e roda o `Handler` do gonest
- [x] `Handler` que panica com algo não-Exception → resposta 500, teste confirma via `app.Test` que o status é 500 e o processo de teste não morre
- [x] `ctx.Json(value)`/`ctx.Status(code)` chegam corretos na resposta HTTP real (`app.Test`)
- [x] Gate check passa (comando abaixo)
- [x] Test count: 5+ (registro simples, dispatch real via app.Test, panic→500, Json correto, Status correto)

**Tests**: integration (usa `app.Test`, real dispatch Fiber — ver TESTING.md atualizado)
**Gate**: full

**Commit**: `feat(http): add Fiber v3 adapter with panic recovery`

---

### T8: `NewApp[T]` genérico + Stage 2.5 (coleta e registro de rota + detecção de colisão) ✅ DONE (evaluator: PASS, commit `129d2da`)

**What**: `NewApp`/`MustNewApp` viram genéricos de verdade (`NewApp[T HttpAdapter](root *Module) (*App, error)`). Depois de Stage 2 (`Declare()` em todos providers/controllers, já existente), passo novo: percorre a árvore de módulos já assembleada (reusa a lista que `Assemble()` devolve), pra cada `Module.OwnControllers()` monta prefixo (`Controller.Path()+Route.path`) e detecta colisão (mesmo método+path) ANTES de registrar no adapter — colisão retorna erro, sem colisão registra cada rota via `adapter.RegisterRoute(...)`.
**Where**: `internal/app/app.go` (pós-migração AD-004, ver STATE.md) + `app.go` na raiz (re-export) — **migração já executada** (commit `4d2d7c9`, antes de T8 rodar), T8 só estende o arquivo que já existe em `internal/app`
**Depends on**: T7
**Reuses**: Stage 1/2/3 inteiros de T9 (DI Graph), `internal/fiberapp.FiberApp`
**Requirement**: CTRL-01, CTRL-08

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `NewApp[gonest.FiberApp](AppModule)` compila e resolve — grafo DI + rotas registradas no mesmo bootstrap
- [x] Rota duplicada (mesmo método+path considerando prefixo) → erro `"duplicate route: GET /user/:id"`, servidor não sobe
- [x] App sem nenhum Controller → bootstrap funciona normal (edge case do spec.md)
- [x] Gate check passa (comando abaixo)
- [x] Test count: 4+ (bootstrap com rotas ok, colisão detectada, app sem controller, edge de prefixo vazio)

**Tests**: unit (a parte de colisão/coleta é lógica pura; dispatch real já coberto em T7's integration)
**Gate**: full

**Commit**: `feat(http): make NewApp generic over HttpAdapter, add Stage 2.5 route registration`

---

### T9: Exemplo end-to-end — `UserController` do INSIGHT.md

**What**: adapta o `UserController`/`UserService` do INSIGHT.md (exemplo mais simples) num teste real — 5 rotas (List/Get/Create/Update/Delete), sem `MustInject` complexo (service simples em memória, como já foi feito em T9 da feature DI Graph pro exemplo `UserProvider`). Prova a feature inteira funcionando junto via `app.Test(req)`.
**Where**: `internal/app/app_test.go` (pós-migração AD-004) — testes de integração podem ficar aqui já que não expõem nada privado extra
**Depends on**: T8
**Reuses**: exemplo já adaptado em T9 da feature DI Graph (`UserProvider`/`UserService`), estende com Controller/Routes reais
**Requirement**: Success Criteria do spec.md

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] As 5 rotas do `UserController` respondem corretamente via `app.Test` (status + body JSON)
- [ ] `MustParam[int64](ctx, "user_id")` funciona nas rotas `Get`/`Update`/`Delete`
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 5+ (1 por rota, no mínimo)

**Tests**: integration
**Gate**: full

**Commit**: `test(http): add end-to-end UserController example from INSIGHT.md`

---

## Parallel Execution Map

```
Phase 1 (Parallel — sem dependência real entre si):
  T1 [P], T2 [P]

Phase 2 (Parallel — pacotes diferentes, T3/T4 ambos só dependem de T2):
  T2 complete, then:
    ├── T3 [P] (internal/pipe)
    └── T4 [P] (internal/route/param.go, função isolada sem tipo Route ainda)

Phase 3 (Sequential):
  T3, T4 complete, then:
    T5 ──→ T6 ──→ T7 ──→ T8 ──→ T9
```

**Nota de paralelismo (L-003):** T3 e T4 tocam pacotes DIFERENTES (`internal/pipe` vs `internal/route`), sem tipo cruzado entre si nessa fase (T4 só usa `*httpctx.Context`, já existente de T2) — seguro rodar em paralelo. T5 em diante é sequencial porque cada task edita o mesmo arquivo que a anterior tocou por último ou depende de tipo que a anterior acabou de criar.

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: go get + HttpMethod | 1 dependency + 1 enum | ✅ Granular |
| T2: Context shell | 1 arquivo, tipo coeso | ✅ Granular |
| T3: Pipe | 1 arquivo | ✅ Granular |
| T4: MustParam coerção default | 1 função isolada | ✅ Granular |
| T5: Route builder | 1 arquivo + wrapper raiz | ✅ Granular |
| T6: Controller estendido | 1 arquivo existente, métodos novos coesos | ✅ Granular |
| T7: FiberApp adapter | 1 arquivo, único ponto de contato com Fiber | ✅ Granular |
| T8: NewApp genérico + Stage 2.5 | 1 arquivo existente, 1 responsabilidade nova coesa | ✅ Granular |
| T9: Exemplo end-to-end | 1 arquivo de teste | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | None | ✅ Match |
| T2 | None | Phase 1 (paralelo com T1, sem edge entre eles) | ✅ Match |
| T3 | T2 | T2 → T3 (Phase 2) | ✅ Match |
| T4 | T2 | T2 → T4 (Phase 2) | ✅ Match |
| T5 | T3, T4 | T3,T4 → T5 | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |
| T7 | T6 | T6 → T7 | ✅ Match |
| T8 | T7 | T7 → T8 | ✅ Match |
| T9 | T8 | T8 → T9 | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | infra + enum | unit | unit | ✅ OK |
| T2 | `Route`/`Pipe`/`Context` isolados | unit | unit | ✅ OK |
| T3 | `Route`/`Pipe`/`Context` isolados | unit | unit | ✅ OK |
| T4 | `Route`/`Pipe`/`Context` isolados | unit | unit | ✅ OK |
| T5 | `Route`/`Pipe`/`Context` isolados | unit | unit | ✅ OK |
| T6 | `Route`/`Pipe`/`Context` isolados | unit | unit | ✅ OK |
| T7 | Dispatch de rota via Fiber real | integration | integration | ✅ OK |
| T8 | Dispatch de rota via Fiber real (parte lógica) | integration ou unit conforme camada | unit (lógica de colisão isolada da rede) | ✅ OK — colisão é lógica pura, sem HTTP real |
| T9 | Dispatch de rota via Fiber real | integration | integration | ✅ OK |

Nenhuma violação.
