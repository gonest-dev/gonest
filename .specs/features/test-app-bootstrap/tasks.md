# Test App Bootstrap Tasks

**Design**: `.specs/features/test-app-bootstrap/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: PLANNED -- spec+design complete, execution intentionally NOT started this session (user's explicit instruction: "só especificar agora, execução depois")

---

## Execution Plan

```
T0 (three-phase bootstrap reorder + AD-008 reversal -- HIGHEST RISK OF THE WHOLE PROJECT)
  → T1 (interface-typed MustInject[T], exact + Implements() fallback)
  → T2 (MustInjectAll[T])
  → T3 (MustNewTestApp / TestBuilder / MustOverride[T])
  → T4 (root re-exports + INSIGHT.md final verification)
```

Sequencial estrito, sem exceção. T0 é maior/mais arriscado que qualquer task já executada nesta sessão (incluindo T0 de "JSON Body Validation" e T0 de "Param/Query & Custom Validation") -- toca Milestone 1 inteiro (`internal/inject`, `internal/resolver`, `internal/provider`, `internal/app`) E reverte AD-008 (Milestone 3, `internal/middleware`/`guard`/`interceptor`/`filter`). Antes de despachar T0, a sessão executora DEVE:

1. Ler TODO o design.md, especialmente a seção "Open Questions for the executing session" -- várias assinaturas ali são melhor-julgamento do orquestrador desta sessão, NÃO verificadas linha-a-linha contra o código atual.
2. Reler o código atual de `internal/inject/inject.go`, `internal/resolver/stage3.go`, `internal/resolver/resolver.go` (ou como quer que se chame o arquivo com `findOwn`/`findExported`), `internal/provider/provider.go`, `internal/controller/controller.go`, `internal/app/app.go`, `internal/middleware/middleware.go`, `internal/guard/guard.go`, `internal/interceptor/interceptor.go`, `internal/filter/filter.go` INTEIROS antes de escrever qualquer linha de código -- o código pode ter mudado desde que este documento foi escrito.
3. Considerar se T0, do jeito que está desenhado aqui, é grande demais pra UMA task só -- a "safety valve" do skill tlc-spec-driven diz pra parar e quebrar mais se a listagem de passos atômicos passar de 5 ou tiver dependência complexa. Este documento JÁ tenta quebrar T0 em sub-passos (ver "T0 breakdown" abaixo) -- avaliar se cada um desses sub-passos merece virar sua PRÓPRIA task com seu próprio evaluator, em vez de 1 task gigante.

---

## Task Breakdown

### T0: Bootstrap de 3 fases + reversão de AD-008 (HIGHEST RISK)

**What**: implementa TUDO que design.md's Architecture Overview descreve -- ver design.md's Components pra assinatura completa de cada peça. Sub-passos sugeridos (a sessão executora decide se cada um vira task própria):

**T0.a -- `internal/provider`**: `resolvedValue`/`SetResolvedValue`/`ResolvedValue` (aditivo puro, baixo risco isolado)

**T0.b -- `internal/resolver`**: `stage3.go` ganha UMA chamada nova (`SetResolvedValue` logo após `callConstructor` suceder, tanto em `invokeAndCopy` quanto `invokeAndCopyEdge`) -- resto do Stage 3 fica INTOCADO. Novo `direct.go` com `findDirect`/`findDirectAll` (reusa/adapta a lógica de escopo de `findOwn`/`findExported` já existente, não reinventa)

**T0.c -- `internal/inject`**: `directResolver` interface nova; `MustInject[T]` ganha o branch de dispatch (owner satisfaz `directResolver`? novo caminho : caminho antigo INTOCADO); `MustInjectAll[T]` novo

**T0.d -- `internal/middleware`/`guard`/`interceptor`/`filter`** (4 pacotes, MESMA mudança mecânica em cada, podem rodar em paralelo entre si já que não se importam mutuamente): `New(fn)` para de rodar `fn` na hora, passa a guardar; `Declare(scope []*module.Module)` novo (idempotente, mesmo contrato de `Pipe.Declare` -- L-012 em STATE.md); `ResolveDirect`/`ResolveDirectAll` implementados delegando pro `internal/resolver`'s `findDirect`/`findDirectAll`

**T0.e -- `internal/controller`**: `ResolveDirect`/`ResolveDirectAll` novos (escopo = só `OwnerModule()`, single-module, diferente do escopo união dos 4 tipos acima)

**T0.f -- `internal/app`**: `declareAll` quebrado em `declareProviders`/`declareControllers`; `discoverPipelineStageOwnership` novo; `declarePipelineStageTypes` novo; `NewApp`'s sequência reordenada pra 3 fases (ver design.md's Architecture Overview, a sequência numerada de 9 passos)

**Where**: `internal/provider/provider.go`, `internal/resolver/stage3.go`, `internal/resolver/direct.go` (novo), `internal/inject/inject.go`, `internal/middleware/middleware.go`, `internal/guard/guard.go`, `internal/interceptor/interceptor.go`, `internal/filter/filter.go`, `internal/controller/controller.go`, `internal/app/app.go`, e TODOS os respectivos `_test.go`

**Depends on**: nenhuma

**Reuses**: `findOwn`/`findExported`'s lógica de escopo (adapta pra `findDirect`), `Pipe.Declare`'s precedente de idempotência (L-012)

**Requirement**: TB-00, TB-00a, TB-00b

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer -- RED inicial é a suite EXISTENTE INTEIRA, não só um pacote), `verification-before-completion` (evaluator -- MÁXIMO rigor, este é o task de maior risco de todo o projeto)

**Done when**:
- [ ] Suite EXISTENTE inteira (`go test ./... -race`) passa com ZERO mudança de asserção em qualquer teste pré-existente (timing de side-effect PODE mudar -- ex: um teste que checava algo acontecendo "imediatamente" na criação de um Guard pode precisar de ajuste de QUANDO checar, mas NUNCA de mudança no QUE é esperado)
- [ ] Provider-a-Provider (`MustInject[*Other](provider)`) continua EXATAMENTE como hoje -- placeholder, PendingEdge, topológico, detecção de ciclo
- [ ] Controller declara DEPOIS que todo Provider resolveu (prova via log de ordem compartilhado, mesma técnica de `pipeline_ordering_test.go`)
- [ ] Guard/Middleware/Interceptor/Filter declaram DEPOIS que todo Controller declarou (prova via mesmo log de ordem)
- [ ] `*Guard` (ou qualquer um dos 4) referenciado por controllers em 2 módulos DIFERENTES: `Declare()` roda EXATAMENTE 1 vez (não 2), com escopo união correto (busca acha provider de QUALQUER um dos 2 módulos)
- [ ] `Declare()` dos 4 tipos é idempotente (chamar 2x não roda `fn` 2x)
- [ ] `MustInject[*Concrete]` (ponteiro) de dentro de Controller/Guard/Middleware/Interceptor/Filter continua funcionando, mesmo call-site, resultado idêntico
- [ ] Gate check passa
- [ ] Test count: 40+ (é o núcleo de risco mais alto do projeto inteiro, cobertura tem que ser exaustiva)

**Tests**: unit + integration (dispatch HTTP real onde fizer sentido, mesma disciplina de toda feature anterior desta sessão)
**Gate**: full

**Commit**: `refactor(app)!: three-phase bootstrap, reverse AD-008 for Middleware/Guard/Interceptor/Filter`

---

### T1: `MustInject[T]` com suporte a interface

**What**: (já implementado em T0.c como PARTE do dispatch -- esta task pode ser MERGED em T0, ou mantida separada se T0 focar só na reordenação e deixar o dispatch de interface pra depois -- decisão da sessão executora). Cobre especificamente: exact match, fallback `Implements()`, panic em 0 ou 2+ matches, mensagens claras.

**Where**: `internal/inject/inject.go`, `internal/resolver/direct.go`

**Depends on**: T0

**Requirement**: TB-01, TB-02

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [ ] `MustInject[Interface]` com exatamente 1 provider implementando resolve corretamente
- [ ] `MustInject[Interface]` com 2+ providers implementando panica mensagem clara mencionando `MustInjectAll`
- [ ] `MustInject[Interface]` com 0 providers implementando panica mensagem clara
- [ ] `MustInject[*Concrete]` continua funcionando idêntico (zero regressão, ponteiro nunca usa `Implements()`)
- [ ] Override provider cujo Constructor declara retorno EXATAMENTE como a interface (não tipo concreto) é achado via exact-match, não via `Implements()`
- [ ] Gate check passa
- [ ] Test count: 10+

**Tests**: unit
**Gate**: full

**Commit**: `feat(inject): MustInject[T] supports interface types via exact + Implements() fallback`

---

### T2: `MustInjectAll[T]`

**What**: novo, `T` deve ser interface (panica senão), devolve `[]T` com todo match, slice vazio (não panic) se zero matches.

**Where**: `internal/inject/inject.go`, `internal/resolver/direct.go`

**Depends on**: T0, T1

**Requirement**: TB-03

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [ ] Reprodução do exemplo `Animal`/`Cat`/`Dog` do INSIGHT.md verbatim -- 2 entradas, `Talk()` de cada correto
- [ ] Zero providers implementando: devolve slice vazio, SEM panic
- [ ] `MustInjectAll[*Concrete]` (ponteiro, não interface) panica mensagem clara
- [ ] `MustInjectAll` chamado a partir de `*provider.Provider` panica (Provider nunca suporta multi-binding)
- [ ] Escopo correto: `MustInjectAll` de dentro de Controller é module-scoped; de dentro de Guard/etc é union-scoped
- [ ] Gate check passa
- [ ] Test count: 8+

**Tests**: unit + integration (dispatch real reproduzindo o exemplo `AnimalController`)
**Gate**: full

**Commit**: `feat(inject): add MustInjectAll[T] for interface multi-binding`

---

### T3: `MustNewTestApp` / `TestBuilder` / `MustOverride[T]`

**What**: reusa TODA a sequência de 9 passos de `NewApp` (design.md's Architecture Overview), com UM ponto de injeção novo: registro de overrides (`map[reflect.Type]reflect.Value`) consultado durante a fase 1 (resolução de Provider) -- se um Provider real bate com um override registrado, usa o valor do override em vez de rodar o Constructor real. Sem `Listen` no final (rotas registradas no adapter, mas nenhuma porta de rede aberta).

**Where**: pacote novo (`internal/testapp`, seguindo AD-004, ou decisão diferente da sessão executora -- ver design.md's Open Questions), + root `gonest.go`/`gonest_test.go`

**Depends on**: T0, T1, T2

**Requirement**: TB-04, TB-05, TB-06

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [ ] Reprodução de `TestUserController_Get` do INSIGHT.md verbatim (override via interface + dispatch HTTP)
- [ ] Reprodução de `TestUserService_Get_NotFound` do INSIGHT.md verbatim (sem override, `MustInject` direto no tester)
- [ ] Provider real cujo tipo bate com um override NUNCA roda seu Constructor real (prova: efeito colateral do Constructor real -- ex: incremento de contador -- nunca acontece quando há override)
- [ ] `MustNewTestApp(module, nil)` (sem overrides) comporta-se identicamente a `NewApp` exceto não bindar porta nenhuma
- [ ] Gate check passa
- [ ] Test count: 12+

**Tests**: unit + integration
**Gate**: full

**Commit**: `feat(testapp): add MustNewTestApp/TestBuilder/MustOverride[T]`

---

### T4: Root re-exports + verificação final do INSIGHT.md

**What**: confirma toda API nova resolve na raiz (`gonest.MustInjectAll`, `gonest.MustNewTestApp`, `gonest.TestBuilder`, `gonest.MustOverride`). Roda TODOS os exemplos do INSIGHT.md que usam `MustInject`/`MustInjectAll`/Testing como teste real, confirma que o arquivo já reflete a API final construída (`MustInjectAll` já foi adicionado nesta sessão -- conferir se o exemplo compila/roda contra o código de verdade, ajustar texto se algo mudou durante a implementação).

**Where**: `gonest.go`, `gonest_test.go`, `INSIGHT.md` (ajustes se necessário)

**Depends on**: T3

**Requirement**: TB-00 até TB-06 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Toda API nova resolve na raiz
- [ ] Reprodução completa do INSIGHT.md (`Animal`/`Cat`/`Dog` E `exemplo de Testing`) via dispatch real
- [ ] Gate check passa (suite INTEIRA, confirma feature completa T0-T4 sem regressão)
- [ ] Test count: 4+

**Tests**: integration
**Gate**: full

**Commit**: `feat(testapp): re-export at root, verify INSIGHT.md examples`

---

## Parallel Execution Map

```
Sequencial estrito: T0 → T1 → T2 → T3 → T4
```

Dentro de T0, os sub-passos `T0.d` (4 pacotes `internal/middleware`/`guard`/`interceptor`/`filter`) SÃO paralelizáveis entre si (não se importam mutuamente) -- mas `T0.a`/`T0.b`/`T0.c` (provider/resolver/inject) devem terminar ANTES de `T0.d`, e `T0.f` (app.go, a orquestração) só faz sentido DEPOIS de `T0.a` até `T0.e` todos prontos.

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate. T0 EXIGE o evaluator mais rigoroso já usado nesta sessão -- suite completa, timing de TODOS os testes de Middleware/Guard/Interceptor/Filter/Controller/Provider auditados individualmente, não só "passou".

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T0: Bootstrap 3 fases + AD-008 | 10 arquivos + 4 pacotes, MAIOR risco do projeto | ⚠️ Considerar quebrar em T0.a-T0.f como tasks próprias -- ver nota no topo desta seção |
| T1: MustInject interface | 2 arquivos, pode já estar em T0 | ✅ Granular (ou já coberto) |
| T2: MustInjectAll | 2 arquivos | ✅ Granular |
| T3: MustNewTestApp/TestBuilder/MustOverride | 1 pacote novo | ✅ Granular |
| T4: Root + INSIGHT.md | Mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T0 | Refactor arquitetural + orquestração | unit + integration | unit + integration | ✅ OK |
| T1 | Núcleo de resolução | unit | unit | ✅ OK |
| T2 | Núcleo de resolução | unit + integration | unit + integration | ✅ OK |
| T3 | Bootstrap variante + override | unit + integration | unit + integration | ✅ OK |
| T4 | Re-export + reprodução E2E | integration | integration | ✅ OK |

Nenhuma violação.
