# Param, Query & Custom Validation Tasks

**Design**: `.specs/features/param-query-validation/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T0-T4, todos evaluator PASS)

---

## Execution Plan

```
T0 (Custom(fn) + shared populate core + refactor MustJsonBody -- HIGH RISK, touches shipped code)
  → T1 (MustParams[T], new, additive)
  → T2 (MustQuery[T], new, additive -- needs Context.Queries() infra)
  → T3 (Remove Pipe + old MustParam[T](ctx,name), migrate all callers to MustParams/MustQuery)
  → T4 (root re-export cleanup + INSIGHT.md rewrite + final dangling-reference sweep)
```

Sequencial estrito. T3 (removal) só roda DEPOIS de T1/T2 existirem -- senão quebra build removendo o único mecanismo de path/query param disponível. T0 é a mais arriscada (mesma classe de risco do T0 de "JSON Body Validation": refatora `MustJsonBody`, já fechado/evaluated, sem mudar comportamento observável) -- evaluator próprio, full suite antes/depois.

---

## Task Breakdown

### T0: `Custom(fn)` + `populate` core compartilhado + refatora `MustJsonBody` ✅ DONE (evaluator: PASS, commit `8d1aa85` -- zero regressão confirmada, `setField` usa fallback JSON marshal/unmarshal pra tipos compostos, verificado são)

**What**: (ver design.md's Components/Data Models pra detalhe completo)

1. `internal/metadata/metadata.go`'s `PropertyBuilder` ganha campo `custom func(raw any) (any, error)` + `func (p *PropertyBuilder) Custom(fn func(raw any) (any, error)) *PropertyBuilder` (bare, sem wrapper, mesmo padrão de `Boolean()`) + `func (p *PropertyBuilder) CustomFunc() (func(raw any) (any, error), bool)`
2. `internal/validate/validate.go`'s `validateValue` ganha um check NOVO no TOPO: se `p.CustomFunc()` setado, chama `fn(raw)`, trata erro como violation, RETORNA (nunca cai pro dispatch de `KindValue()` existente)
3. Novo `populate(dest reflect.Value, presence map[string]any, m *metadata.Metadata, tag string) error` -- itera `m.OwnProperties()`, resolve chave via `tagKey(field, tag)`, se `Custom` setado chama `fn(raw)` de novo (aceita chamar 2x, ver design.md's Tech Decisions) e usa o valor devolvido; senão usa o valor já validado direto, convertendo via `setField`
4. Novo `setField(fieldVal reflect.Value, raw any) error` -- converte `raw` pro tipo Go do campo e faz `Set`, devolve erro (nunca panica) se incompatível
5. `tagKey(field reflect.StructField, tag string) string` -- generaliza o helper de tag `json` já existente pra aceitar `tag` como parâmetro
6. `MustJsonBody[T]` REFATORADO: validação (pass 1+2, presença+tipo) fica IDÊNTICA; o passo final muda de `json.Unmarshal(body, result)` pra `populate(reflect.ValueOf(result).Elem(), presenceMap, m, "json")`

**Where**: `internal/metadata/metadata.go` (estendido), `internal/validate/validate.go` (estendido/reestruturado), `internal/validate/validate_test.go` (existente, NENHUMA modificação de asserção -- só talvez ajuste de setup se necessário)

**Depends on**: nenhuma

**Reuses**: `Metadata.OwnProperties()`, toda a lógica de `validateStruct`/`validateArray`/`validateObject`/`validatePrimitive` já existente (inalterada)

**Requirement**: PQ-00

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer -- RED inicial é a suite EXISTENTE de `internal/validate`, igual T0 de "JSON Body Validation"), `verification-before-completion` (evaluator)

**Done when**:
- [x] TODOS os testes EXISTENTES de `internal/validate` passam SEM MUDANÇA de asserção (`validate_test.go` não tocado, confirmado por evaluator via `git show --stat`)
- [x] Novo teste: `Custom(fn)` decodifica formato próprio end-to-end via `MustJsonBody` (dispatch HTTP real)
- [x] Novo teste: `Custom(fn)` retornando erro gera violation coletada junto com outras
- [x] Novo teste: `Custom(fn)` retornando tipo incompatível gera violation via `setField`, nunca panica
- [x] Novo teste: campo sem `Custom` populando como antes (regressão provada)
- [x] Gate check passa
- [x] `gofmt -l` limpo

**Tests**: unit + integration (dispatch real)
**Gate**: full

**Commit**: `refactor(validate): add Custom(fn) escape hatch, unify T population via reflect`

---

### T1: `MustParams[T]` -- path params, whole-object ✅ DONE (evaluator: PASS, commit `9b0d22d`)

**What**: `internal/validate` ganha `func MustParams[T any](ctx *execution.Context) T` -- lookup registry, resolve `*route.Route` via `ctx.Route()` type assertion, pra cada `m.OwnProperties()` resolve chave via `tagKey(field, "param")`, checa `route.HasParam(key)` (presença), lê `ctx.Param(key)` (raw string), coerce via `coerceParamString(raw, p.KindValue())` (OU passa raw direto pra `Custom(fn)` se setado, SEM coerção), valida via `validateValue` reusado, coleta violations, panic `BadRequestException` ou `populate(..., "param")` e devolve.

Novo `coerceParamString(raw string, kind string) (any, error)` -- `"string"` fica string; `"integer"`/`"number"` via `strconv`; `"boolean"` via `strconv.ParseBool`.

**Where**: `internal/validate/validate.go` (ou novo `internal/validate/params.go`, dev decide), `internal/validate/params_test.go` (novo)

**Depends on**: T0

**Reuses**: `route.Route.HasParam` (já existente), `Context.Param` (já existente), `validateValue`/`populate` de T0

**Requirement**: PQ-02, PQ-03

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] Rota com 2+ path params, todos válidos, devolve `*T` populado sem panic
- [x] Path param ausente gera violation
- [x] Path param presente mas inválido gera violation
- [x] 2 violações simultâneas: AMBAS aparecem em `details`
- [x] `Custom(fn)` recebe STRING crua, não coagida
- [x] `T` nunca registrado panica ANTES de ler qualquer param
- [x] Gate check passa
- [x] Test count: 10 entregues

**Tests**: unit + integration (dispatch real, `app.Test`)
**Gate**: full

**Commit**: `feat(validate): add MustParams[T] whole-object path param validation`

---

### T2: `MustQuery[T]` -- query params, whole-object ✅ DONE (evaluator: PASS, commit `00cda54` -- dev achou proativamente bug real de buffer reuse do Fiber Queries(), evaluator confirmou lendo source do Fiber, fix aceito)

**What**: `internal/execution.Responder` ganha `Queries() map[string]string`; `Context` ganha `Queries() map[string]string` (delegação de 1 linha); `internal/adapter/fiber`'s `fiberResponder` implementa via `r.c.Queries()`. `internal/validate` ganha `func MustQuery[T any](ctx *execution.Context) T` -- MESMA lógica de `MustParams`, mas presença/raw vêm de `ctx.Queries()` em vez de `route.HasParam`/`ctx.Param`.

**Where**: `internal/execution/context.go` (estendido), `internal/adapter/fiber/fiber.go` (estendido), `internal/validate/validate.go` (ou `query.go`), `internal/validate/query_test.go` (novo)

**Depends on**: T0, T1 (reusa `coerceParamString`/`populate` de T0, mesmo padrão de T1)

**Reuses**: `coerceParamString`, `populate`, `validateValue`

**Requirement**: PQ-04, PQ-05

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] `Context.Queries()` devolve o mapa cru de um `Responder` fake, teste isolado
- [x] Dispatch HTTP real confirma `fiberResponder.Queries()` funciona (+ fix de buffer reuse do Fiber, achado pelo dev, confirmado pelo evaluator lendo o source)
- [x] Query com `Required` ausente + outro campo fora de range simultaneamente: AMBAS violations aparecem
- [x] `Custom(fn)` num campo de query recebe string crua
- [x] Happy path devolve `*T` populado
- [x] Gate check passa
- [x] Test count: 8 entregues em query_test.go + testes de infra

**Tests**: unit + integration (dispatch real)
**Gate**: full

**Commit**: `feat(validate): add MustQuery[T] whole-object query param validation`

---

### T3: Remove Pipe + `MustParam[T](ctx,name)` singular ✅ DONE (evaluator: PASS, commit `db19cfc` -- sweeps limpos, migrações de teste preservam cobertura, deleções confirmadas redundantes antes de remover)

**What**: DELETA `internal/pipe/` (pacote inteiro), `internal/route/param.go` (arquivo inteiro -- `MustParam[T]`/`defaultCoerce`/`callPipeHandler`), `Route.Param`/`Route.PipeFor`/`Route.paramPipes` de `internal/route/route.go`. Remove `gonest.Pipe`/`gonest.NewPipe`/`gonest.MustParam[T](ctx,name)` de `gonest.go`. MIGRA todo call-site existente nos testes (root `gonest_test.go`, `internal/route/route_test.go`, qualquer outro) que usava `MustParam[T](ctx,name)`/`ParseIntPipe`/`route.Param(name,pipe)` pra usar `MustParams[T](ctx)` (T1) em vez disso.

**Where**: `internal/pipe/` (deletado), `internal/route/param.go` (deletado), `internal/route/route.go` (estendido -- remove campos/métodos), `internal/route/param_test.go` (deletado se só testava o que saiu), `internal/route/route_test.go` (migrado), `gonest.go` (estendido -- remove exports), `gonest_test.go` (migrado)

**Depends on**: T0, T1, T2 (precisa de `MustParams`/`MustQuery` existindo antes de remover o mecanismo antigo)

**Reuses**: nada novo -- remoção pura + migração de call-site

**Requirement**: PQ-01

**Tools**:
- MCP: NONE
- Skill: `verification-before-completion` (evaluator -- sem `test-driven-development` aqui, é remoção não feature nova)

**Done when**:
- [x] Sweep `internal/pipe`/`gonest.Pipe`/`gonest.NewPipe`: ZERO resultados (confirmado por evaluator, disco + grep)
- [x] Sweep `MustParam[...](ctx, "...")` (forma antiga 2-arg): ZERO resultados
- [x] `route.Param`/`route.PipeFor` não existem mais, `route.HasParam` intacto
- [x] Todo teste migrado, cobertura preservada (evaluator confirmou cada migração prova o mesmo que a versão antiga); 3 testes deletados como redundantes com `Custom(fn)` coverage já existente, confirmado antes de deletar
- [x] `go build ./...` limpo, `go vet ./...` limpo
- [x] Gate check passa (18 pacotes)

**Tests**: (nenhum teste NOVO -- migração de testes existentes)
**Gate**: full

**Commit**: `refactor(route)!: remove Pipe, MustParam[T](ctx,name) superseded by MustParams/MustQuery`

---

### T4: Root re-export cleanup + reprodução INSIGHT.md + sweep final ✅ DONE (evaluator: PASS, commits `cf6fd3c`/`ea9f72a` -- feature completa end-to-end, sem regressão)

**What**: confirma `gonest.MustParams[T]`/`gonest.MustQuery[T]`/`gonest.Custom` (via `PropertyBuilder`, já promovido) resolvem na raiz (T1/T2 já devem ter feito isso via `internal/validate` -- se não, adiciona aqui). Reescreve `gonest_test.go` reproduzindo o exemplo `UserIdParams`/`ListUsersQuery` do INSIGHT.md via dispatch HTTP real. Roda sweep final de `grep` confirmando zero referência solta.

**Where**: `gonest.go` (se faltando), `gonest_test.go` (estendido)

**Depends on**: T3

**Reuses**: tudo já construído

**Requirement**: PQ-02 até PQ-05 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.MustParams[T]`/`gonest.MustQuery[T]` resolvem na raiz (não existiam, adicionados nesta task)
- [x] Reprodução do exemplo INSIGHT.md via dispatch HTTP real (path + query na mesma rota, caso feliz + violação)
- [x] Gate check passa (18 pacotes, sem regressão)
- [x] Test count: 1 teste (2 subtests) + INSIGHT.md reescrito (3 seções: "exemplo mais simples", Middleware/Guard/Interceptor/Filter com `Custom(fn)`, seção final settled)

**Tests**: integration
**Gate**: quick

**Commit**: `feat(validate): re-export MustParams/MustQuery at root`

---

## Parallel Execution Map

```
Sequencial estrito: T0 → T1 → T2 → T3 → T4
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`. T0 e T3 em particular: evaluator DEVE rodar a suite completa, não só o pacote tocado (T0 por refatorar código já fechado, T3 por ser remoção com risco de quebrar build).

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T0: Custom + populate + refatora MustJsonBody | 2 arquivos, ALTO RISCO (reabre código fechado) | ✅ Granular (isolado com barreira própria) |
| T1: MustParams | 1-2 arquivos novos, aditivo | ✅ Granular |
| T2: MustQuery + infra Queries() | 3 arquivos, aditivo | ✅ Granular |
| T3: Remove Pipe + migração | Múltiplos arquivos deletados/migrados, risco de build quebrado se fora de ordem | ✅ Granular (mas exige T0-T2 prontos antes) |
| T4: Root cleanup + E2E | Mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T0 | Refactor + reflection puro | unit + integration | unit + integration | ✅ OK |
| T1 | Núcleo de validação + dispatch HTTP | unit + integration | unit + integration | ✅ OK |
| T2 | Infra HTTP + núcleo de validação | unit + integration | unit + integration | ✅ OK |
| T3 | Remoção/migração | integration (via testes migrados) | integration | ✅ OK |
| T4 | Re-export + E2E | integration | integration | ✅ OK |

Nenhuma violação.
