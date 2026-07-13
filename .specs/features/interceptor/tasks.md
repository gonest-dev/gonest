# Interceptor Tasks

**Design**: `.specs/features/interceptor/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (internal/interceptor: Next/Interceptor/New/Handler) → T2 (Controller.Interceptors real type + OwnInterceptors) → T3 (Stage 2.5: interceptedHandler) → T4 (root re-exports)
```

Fully sequential — mesmo padrão de "Guard" (sem `Module.Interceptors`, sem segundo pacote pra paralelizar contra T2).

---

## Task Breakdown

### T1: `internal/interceptor` — `Next`/`Interceptor`/`New`/`Handler`

**What**: pacote novo. `type Next func(ctx *httpctx.Context)` (tipo PRÓPRIO, não reusa `middleware.Next` — ver design.md's Tech Decisions), `type Interceptor struct { handler func(ctx *httpctx.Context, next Next) }` (campo não-exportado), `func New(fn func(*Interceptor)) *Interceptor` (roda `fn` IMEDIATAMENTE, mesmo padrão de `middleware.New`/`guard.New`, AD-008), `func (i *Interceptor) Handler(h func(ctx *httpctx.Context, next Next))`, `func (i *Interceptor) HandlerFunc() func(ctx *httpctx.Context, next Next)` (`nil` se `Handler` nunca foi chamado).
**Where**: `internal/interceptor/interceptor.go`, `internal/interceptor/interceptor_test.go`
**Depends on**: None
**Reuses**: `httpctx.Context`, o padrão exato de "execução imediata" + `Handler`/`HandlerFunc` já estabelecido em `internal/middleware`/`internal/guard`
**Requirement**: ITC-01

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `New(fn)` roda `fn` imediatamente (teste prova via efeito colateral observável)
- [ ] `Handler(h)` armazena `h`, `HandlerFunc()` devolve exatamente essa função — chamar em teste com `ctx`/`next` fake, confirmar que ambos chegam corretamente no corpo do handler
- [ ] `HandlerFunc()` devolve `nil` se `Handler` nunca foi chamado
- [ ] Um `func(ctx *httpctx.Context)` puro é atribuível direto a `Next` sem conversão (prova de identidade de tipo, mesma prova que `internal/middleware`'s T1 já fez pro próprio `Next`)
- [ ] Gate check passa
- [ ] Test count: 4+ (execução imediata, round-trip Handler/HandlerFunc, nil zero-value, identidade de tipo de Next)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(interceptor): add Next/Interceptor core types`

---

### T2: `Controller.Interceptors` real type + `OwnInterceptors`

**What**: `internal/controller/controller.go`'s `Interceptors(items ...Middleware)` (stub desde T6, tipo placeholder `Middleware struct{}`) muda pro tipo real `*interceptor.Interceptor` (T1). Campo `guards`... digo, `interceptors []Middleware` → `[]*interceptor.Interceptor`. `func (c *Controller) Interceptors(items ...*interceptor.Interceptor)`. Novo accessor: `func (c *Controller) OwnInterceptors() []*interceptor.Interceptor` (cópia defensiva, espelha `OwnGuards`). NÃO tocar `Filters` nem o tipo placeholder `Middleware struct{}` (continuam intocados — `Filters` fica sendo o ÚNICO stub restante depois desta feature).
**Where**: `internal/controller/controller.go` (existente, estende), `internal/controller/controller_test.go` (existente — migrar `TestPipelineStubs_DoNotAffectObservableState` pra usar `interceptor.New(nil)` real no `Interceptors(...)`, mantendo `Filters(...)` com o stub antigo)
**Depends on**: T1
**Reuses**: `interceptor.Interceptor` de T1, o padrão exato `Guards`/`OwnGuards` que esse arquivo já cresceu em "Guard"
**Requirement**: ITC-01, ITC-03 (parte de armazenamento/ordem)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Controller.Interceptors(i1, i2, ...)` armazena valores reais `*interceptor.Interceptor` em ordem de registro
- [ ] `OwnInterceptors()` devolve cópia defensiva — teste prova que mutar o slice devolvido não afeta estado interno
- [ ] `TestPipelineStubs_DoNotAffectObservableState` migrado: `Interceptors(...)` usa valores reais, `Filters(...)` continua com o stub antigo, asserções refletem corretamente que `Interceptors` (como `Use`/`Guards` antes) agora armazena de verdade enquanto `Filters` continua no-op
- [ ] Gate check passa
- [ ] Test count: 3+ (Interceptors armazena em ordem, OwnInterceptors cópia defensiva, teste pré-existente migrado e ainda verde)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): Controller.Interceptors stores real Interceptor, add OwnInterceptors accessor`

---

### T3: Stage 2.5 `interceptedHandler` em `internal/app`

**What**: insere uma NOVA camada entre o `gatedHandler` já existente (de "Guard") e o loop de composição de middleware já existente (de "Middleware") — nenhum dos dois é reescrito, essa feature só insere um passo novo de composição entre eles. Encadeia `controllerRC.OwnInterceptors()` em torno de `gatedHandler`, na mesma forma de composição (ordem de registro, de fora pra dentro) já usada por middleware. Estende `routableController` com `OwnInterceptors() []*interceptor.Interceptor` (já satisfeito por `*controller.Controller` pós-T2).
**Where**: `internal/app/app.go` (existente, estendido — novo import `internal/interceptor`), `internal/app/app_test.go` (existente — adiciona testes)
**Depends on**: T2
**Reuses**: `interceptor.Interceptor`/`HandlerFunc` (T1), `Controller.OwnInterceptors()` (T2), `gatedHandler` já existente (de "Guard", lógica intocada), o loop de composição de middleware já existente (de "Middleware", lógica intocada, só o argumento de entrada muda)
**Requirement**: ITC-01, ITC-02, ITC-03, ITC-04, ITC-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (tudo via dispatch real `app.Test`):
- [ ] Um interceptor único roda código ANTES de `next(ctx)`, depois o Handler roda, depois roda código DEPOIS de `next(ctx)` retornar — prova via recorder de ordem observável (ex: `["before","handler","after"]`), não só "os dois rodaram em algum momento"
- [ ] Interceptor que NÃO chama `next(ctx)` faz o Handler NÃO rodar (mesmo short-circuit já estabelecido pra Middleware)
- [ ] Múltiplos interceptors (2+) compõem em ordem de registro — prova via sequência ordenada explícita
- [ ] Controller com Guards + Interceptors + Middleware juntos: ordem final é Middleware → Guard → Interceptor(before) → Handler → Interceptor(after) — prova via sequência ordenada explícita cobrindo os 3 estágios ao mesmo tempo
- [ ] Interceptor que panica (antes ou depois de `next`) é capturado pelo MESMO recover wrapper existente, produz resposta correta (Exception ou 500 genérico conforme o tipo do panic)
- [ ] Controller com ZERO `Interceptors()` se comporta EXATAMENTE como antes desta feature (zero regressão) — confirmar teste pré-existente (de "Guard" ou "Middleware", ou T9's `UserController`) continua passando SEM MODIFICAÇÃO
- [ ] Gate check passa
- [ ] Test count: 8+ (before/handler/after em ordem, short-circuit sem next, múltiplos interceptors em ordem, pipeline completo Middleware→Guard→Interceptor→Handler, panic antes de next, panic depois de next, zero-regressão)

**Tests**: integration (dispatch real via `app.Test`)
**Gate**: full

**Commit**: `feat(app): compose Interceptor chain (before/after Handler) in Stage 2.5, between Guard and Middleware layers`

---

### T4: Root re-exports

**What**: pacote raiz `gonest` ganha `Interceptor` (alias de tipo) e `NewInterceptor` (`var NewInterceptor = interceptor.New` — alias simples, não-genérico, mesmo idioma de `NewGuard`/`NewMiddleware`).
**Where**: arquivo novo na raiz, `interceptor.go`, arquivo de teste raiz
**Depends on**: T1, T2, T3
**Reuses**: idioma exato `type X = pkg.X` / `var Y = pkg.Y` já usado na raiz (ver `guard.go` na raiz, precedente mais recente)
**Requirement**: ITC-01 até ITC-05 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.NewInterceptor(fn)`, `gonest.Interceptor` resolvem e funcionam na raiz
- [ ] Exemplo do INSIGHT.md `TimingInterceptor` (adaptado per spec.md's Out of Scope: sem `MustInject`, capturar logger via closure) reproduzido via aliases raiz, anexado via `controller.Interceptors(...)` através de `Controller`/`Module`/`NewApp` raiz, dispatch real via `app.Test`, confirma que before/after rodaram na ordem certa
- [ ] Gate check passa
- [ ] Test count: 2+ (smoke test raiz pra `NewInterceptor`/`Interceptor` resolverem, reprodução do `TimingInterceptor` end-to-end via aliases raiz)

**Tests**: unit (dispatch integration-style, convenção já estabelecida pros testes raiz)
**Gate**: quick

**Commit**: `feat(interceptor): re-export Interceptor/NewInterceptor at root`

---

## Parallel Execution Map

```
Totalmente sequencial: T1 → T2 → T3 → T4
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: internal/interceptor core | 1 arquivo novo, pacote novo pequeno e coeso | ✅ Granular |
| T2: Controller.Interceptors real | 1 arquivo existente, mecânico (repete padrão de Guard) | ✅ Granular |
| T3: Stage 2.5 interceptedHandler | 1 arquivo existente, 1 responsabilidade nova coesa (denso em testes) | ✅ Granular |
| T4: Root re-exports | 1 arquivo novo, mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP real | unit | unit | ✅ OK |
| T2 | Builder isolado (Controller), sem dispatch real | unit | unit | ✅ OK |
| T3 | Dispatch de rota via Fiber real (composição de handler + recovery) | integration | integration | ✅ OK |
| T4 | Re-export + reprodução end-to-end via root | unit (com 1 caso integration-style) | unit | ✅ OK |

Nenhuma violação.
