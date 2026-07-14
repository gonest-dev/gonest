# Pipeline Ordering Tasks

**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1, evaluator PASS) — **Milestone 3 (Request Pipeline) COMPLETE**

Escopo pequeno o suficiente (validação, não construção — ver spec.md's Out of Scope) que não justifica um `design.md` formal. Toda decisão de composição já foi tomada e implementada nas features anteriores (Middleware/Guard/Interceptor/Pipe/Filter) — essa feature só prova, num cenário combinado único, que a ordem documentada em ROADMAP.md se sustenta.

---

## Execution Plan

```
T1 (teste de integração único — todas as 5 peças combinadas)
```

Task única — não há paralelismo a planejar.

---

## Task Breakdown

### T1: Teste combinado — reproduz o `UserController` completo do INSIGHT.md ✅ DONE (evaluator: PASS, commit `52cff89` — nenhum bug de código real achado, arquitetura já correta; evaluator reproduziu o experimento de inverter a ordem e confirmou falha genuína da asserção)

**What**: em `internal/app/app_test.go` (ou arquivo novo `internal/app/pipeline_ordering_test.go` no mesmo pacote — decisão do developer, ambos válidos), escrever um teste (ou pequena tabela de subtestes) que reproduz o exemplo completo de pipeline do INSIGHT.md (seção "aplicando tudo no controller"): controller com `Use`/`Guards`/`Interceptors`/`Filters` TODOS registrados, módulo raiz com `Use`/`Filters` GLOBAIS também registrados, e uma rota usando `Route.Param` com um Pipe customizado. Dispatch via `app.Test` real, cobrindo os 3 cenários do spec.md (ORD-01/02/03):
1. Caminho feliz completo — recorder de ordem prova a sequência exata: middleware global → middleware controller → guard → interceptor(before) → handler (que roda o Pipe via `MustParam[T]`) → interceptor(after)
2. Guard rejeita — interceptor(before) e handler (e portanto o Pipe) NÃO rodam
3. Pipe panica (param inválido) — Filter registrado pra esse tipo de exception captura corretamente (ou, se não capturado, cai no default `{name,message,details}`)

Se a checagem revelar alguma lacuna real de ordem (comportamento atual não bate com o documentado em ROADMAP.md), trate como bug de verdade — corrija o código de composição real em `internal/app`, não contorne no teste. Documente o achado no relatório final com o mesmo rigor de L-011.

**Where**: `internal/app/app_test.go` ou `internal/app/pipeline_ordering_test.go`
**Depends on**: None (tudo que precisa já está commitado — Middleware/Guard/Interceptor/Pipe/Filter, todas completas)
**Reuses**: `UserService`/`UserProvider`-style exemplo em memória já estabelecido (T9 de "Controller & Route Registration"), técnica de order-recorder já usada em "Middleware"/"Guard"/"Interceptor"'s próprios testes T4
**Requirement**: ORD-01, ORD-02, ORD-03

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] Caminho feliz: ordem exata provada via recorder — `global-middleware → controller-middleware → guard → interceptor-before → handler-before-pipe → pipe → interceptor-after` (confere com ROADMAP.md)
- [x] Guard-rejeita: interceptor-before e handler NÃO aparecem no recorder, resposta 403
- [x] Pipe panica dentro do handler: ambos sub-cenários testados (Filter captura, e não-capturado cai no default)
- [x] Gate check passa
- [x] Test count: 3+ (4 subtestes via `t.Run`)
- [x] Nenhuma lacuna de ordem encontrada — arquitetura já correta desde as features individuais; evaluator reproduziu experimento de ordem invertida e confirmou asserção genuína

**Tests**: integration (dispatch real via `app.Test`)
**Gate**: full

**Commit**: `test(app): add combined pipeline ordering test (Middleware→Guard→Interceptor→Pipe→Handler), closes Milestone 3`

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: teste combinado de ordem | 1 arquivo de teste, 1 responsabilidade coesa (validação, não construção) | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Dispatch de rota via Fiber real, todos os estágios de pipeline combinados | integration | integration | ✅ OK |

Nenhuma violação.
