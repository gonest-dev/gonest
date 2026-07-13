# Provider & DI Graph Tasks

**Design**: `.specs/features/provider-di-graph/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Done — T1-T11 completas, evaluator PASS em todas (95/95 testes, ver STATE.md)

---

## Execution Plan

### Phase 1: Foundation (Sequential)

```
T1 → T2
```

### Phase 2: Structural Types (revisado 2026-07-12 — AD-004)

**Revisão 1 (L-003):** T3/T4/T5 estavam `[P]` num pacote único `gonest` — colisão real de compilação. **Revisão 2 (AD-004):** reestruturado pra 1 pacote Go por tipo (`internal/module`, `internal/provider`, `internal/controller`) — T4/T5 voltam a ser paralelizáveis de verdade (pacotes diferentes), só T3 fica sequencial antes (T4/T5 dependem do `module.Owner` que T3 define).

```
T2 → T3 ──┬→ T4 ─┐
          └→ T5 ─┴→ T6
```

### Phase 3: Resolution Engine (Sequential)

```
T6 → T7 → T8 → T9
```

### Phase 4: Scopes (Sequential — mesmo pacote internal/resolver, ver L-003)

```
T9 → T10 → T11
```

---

## Task Breakdown

### T1: Setup go.mod + dependências ✅ DONE (evaluator: PASS)

**What**: cria `go.mod` (`module github.com/gonest-dev/gonest`), adiciona `golang.org/x/sync/errgroup` e `github.com/stretchr/testify`.
**Where**: `go.mod`, `go.sum`
**Depends on**: None
**Reuses**: —
**Requirement**: infra (não mapeado a DI-XX)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `go.mod` existe com module path correto e versão Go definida (`go 1.25.0` — floor real imposto por `golang.org/x/sync@v0.22.0`, verificado via `go list -m -json`)
- [x] `go build ./...` roda sem erro (mesmo sem código ainda, só valida módulo)
- [x] `go get golang.org/x/sync/errgroup github.com/stretchr/testify` resolve sem erro

**Tests**: none
**Gate**: full (`go test ./... -race` — sem testes ainda, só confirma que roda sem erro de build)

**Commit**: `chore(setup): initialize go.mod with core deps`

---

### T2: Definir Scope enum + interface ownerModule ✅ DONE (evaluator: PASS-WITH-NOTE)

**What**: define `type Scope int` (`ScopeSingleton`, `ScopeTransient`, `ScopeRequest`) e a interface interna `interface{ ownerModule() *Module }` usada por `MustResolve`.
**Where**: `scope.go`
**Depends on**: T1
**Reuses**: —
**Requirement**: DI-01 (base pra scope), DI-06, DI-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Scope` exportado com os 3 valores, `String()` implementado (debug-friendly)
- [x] Interface `ownerModule` definida (não-exportada, mecanismo interno) — **nota**: implementada como `ownerModule() any` (não `*Module`, que ainda não existe). Design.md especifica a interface como constraint anônima inline dentro de `MustResolve[T](owner interface{ownerModule() *Module})`, não um tipo nomeado em scope.go — T3/T6 provavelmente vão substituir esta declaração em vez de reusá-la. Ver STATE.md.
- [x] Gate check passa: `go test ./... -race` — resolvido (MinGW-w64 instalado + override `CC=gcc CXX=g++` inline, ver TESTING.md e B-001 em STATE.md). 4/4 testes passam com `-race` ligado.
- [x] Test count: 3+ testes (um por valor de `Scope.String()`) — 4 testes, incluindo caso `default`/unknown

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Scope enum and ownerModule contract`

---

### T3: Module — API declarativa (Stage 1)

**What**: `module.New(fn)` guarda `fn` sem executar; `Module.Imports/Providers/Controllers/Exports` registram referências; função interna de Stage 1 faz o BFS/DFS e valida que todo `Exports` é subconjunto de `Providers`. Define `module.Owner` (contrato de ownership, substitui o `ownerModule() any` provisório de T2).
**Where**: `internal/module/module.go`, `internal/module/assemble.go` (implementação) + `module.go` na raiz (re-export)
**Depends on**: T2
**Reuses**: `internal/scope.Scope` só pro tipo, sem uso direto ainda
**Requirement**: DI-10 (parte: export inválido)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (✅ DONE — evaluator: PASS, commit `96cfd6c`):
- [x] `module.New(fn)` NÃO executa `fn` na chamada (teste explícito: flag booleana só vira true depois do assemble)
- [x] Stage 1 percorre `Imports` recursivamente sem duplicar módulo visitado 2x (grafo pode ter diamante) — diamante real testado (A→B,C; B,C→D), D visitado 1x
- [x] `Exports` de provider não declarado em `Providers` do mesmo módulo → erro `"module X exports provider *Y it does not declare"` — mensagem funcional, mas `%Y`/nome do módulo ainda cai em fallback `%p` (endereço) por falta de campo de nome; polimento adiado, não bloqueia
- [x] Gate check passa (comando abaixo) — 15/15 testes, `-race` ok
- [x] Test count: 5+ (fn adiado, BFS simples, import diamante, export inválido, export válido) — 7 entregues

**Nota pra T7 (registrada por evaluator):** `providerRef`/`controllerRef` (marker interfaces, Option A) dão identidade/comparabilidade mas não introspecção de tipo — T7 vai precisar alargar essas interfaces em `internal/module` (algo tipo `ResolvedType() reflect.Type`) pra fazer a busca "esse Provider resolve o tipo X". Mudança aditiva, não quebra T4/T5.

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Module declarative API and Stage 1 assembly`

---

### T4: Provider — API declarativa (Stage 2 shell) [P]

**What**: `provider.New(fn)` guarda `fn` sem executar; `Provider.Scope(s)`, `Provider.Constructor(fn any)` armazenam config. `Constructor` aceita via reflect as 4 assinaturas do design (`func() T`, `func() (T, error)`, `func(context.Context) T`, `func(context.Context) (T, error)`) — validação de assinatura acontece aqui, execução real fica pra T9. Implementa `module.Owner` (`OwnerModule() *module.Module`).
**Where**: `internal/provider/provider.go` + `provider.go` na raiz (re-export)
**Depends on**: T3 (precisa de `internal/module.Owner`/`*Module` pra implementar o contrato)
**Reuses**: `internal/scope.Scope`, `internal/module.Owner`
**Requirement**: DI-01 (parte declarativa)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (✅ DONE — evaluator pendente, commit `80b46e4` + fix `40725e5`):
- [x] `NewProvider(fn)` não executa `fn` na chamada
- [x] `Constructor` aceita as 4 assinaturas válidas via reflect, panica em tempo de declaração se a assinatura não bater com nenhuma (`"gonest: invalid Constructor signature"`)
- [x] `Scope` default é `ScopeSingleton` se não chamado explicitamente
- [x] Gate check passa (comando abaixo) — 10/10 testes
- [x] Test count: 6+ (fn adiado, 4 assinaturas válidas, 1 assinatura inválida, default scope) — 12 entregues (10 originais + 2 pós-fix)

**Achado crítico (T4+T5, corrigido fora da task em `40725e5`):** `providerRef`/`controllerRef` de T3 eram interfaces com método não-exportado — Go não deixa satisfazer isso fora do pacote `module`. `internal/module` corrigido pra exportar `ProviderRef`/`ControllerRef` + `IsProvider()`/`IsController()`. Ver STATE.md L-004 (gap no processo de evaluation do T3).

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Provider declarative API with Constructor signature validation`

---

### T5: Controller — shell mínimo pra MustResolve [P]

**What**: `controller.New(fn)` guarda `fn` sem executar; implementa só o suficiente pra satisfazer `module.Owner` (Path/Route/Handler ficam pra feature "Controller & Route Registration" — aqui é só o shell que permite `MustResolve` funcionar dentro do `fn` do Controller).
**Where**: `internal/controller/controller.go` + `controller.go` na raiz (re-export)
**Depends on**: T3 (precisa de `internal/module.Owner`/`*Module` pra implementar o contrato) — roda em paralelo com T4, pacotes diferentes (`internal/controller` vs `internal/provider`)
**Reuses**: mesmo padrão de adiamento de `fn` do T3/T4, `internal/module.Owner`
**Requirement**: DI-01 (Controller como consumidor)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (✅ DONE — evaluator pendente, commit `e539922` + fix `40725e5`):
- [x] `controller.New(fn)` não executa `fn` na chamada
- [x] `OwnerModule()` implementado (populado via `SetOwnerModule`; wiring automático pós-assemble fica pra task futura — Stage 1 hoje só roda `fn`, não seta ownership de volta, ver nota abaixo)
- [x] Gate check passa (comando abaixo) — 4/4 testes (+ compile-proof novo pós-fix)
- [x] Test count: 2+ (fn adiado, OwnerModule populado pós-assemble) — 5 entregues

**Nota (também vale pra T4):** nem T4 nem T5 automatizam o "wiring" de `OwnerModule` durante o Stage 1 (assemble hoje só chama `fn`, não seta dono nos providers/controllers registrados) — ambos usam `SetOwnerModule` manual por enquanto. Quem pegar T7 (busca escopada) precisa fechar esse wiring de verdade, senão `OwnerModule()` sempre retorna nil em uso real via `NewModule(...)`.

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add minimal Controller shell for MustResolve consumers`

---

### T6: MustResolve[T] — placeholder via reflect

**What**: `resolve.MustResolve[T any](owner module.Owner) T` (+ wrapper genérico público em `resolve.go` na raiz) — valida que `T` é ponteiro (panic claro se não for), aloca placeholder via `reflect.New(...).Elem().Elem()`, registra a chamada como "pending edge" (owner → tipo T) numa lista interna consultada no Stage 2.
**Where**: `internal/resolve/resolve.go` + `resolve.go` na raiz (wrapper genérico)
**Depends on**: T4, T5 (precisa de `Provider`/`Controller` implementando `module.Owner`)
**Reuses**: `module.Owner` de T3, implementações de T4/T5
**Requirement**: DI-05, DI-09 (parte: panic de tipo não-ponteiro é achado aqui, "não registrado" só fecha em T7)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (✅ DONE — evaluator: PASS, commit `7bfca23`):
- [x] `MustResolve[*Foo](owner)` devolve ponteiro não-nil (placeholder alocado) sem panicar quando `T` é ponteiro válido — usável de verdade (escreve/lê campo através do ponteiro), não só non-nil
- [x] `MustResolve[Foo](owner)` (tipo não-ponteiro) panica com `"gonest: MustResolve[T] requires T to be a pointer type, got Foo"` — mensagem dinâmica confirmada com 3 tipos diferentes
- [x] Cada chamada registra exatamente 1 pending edge, consultável por um helper interno de teste
- [x] Gate check passa (comando abaixo) — 35/35 testes totais
- [x] Test count: 4+ (placeholder válido, panic tipo errado, edge registrada, múltiplas chamadas não colidem) — 7 entregues

**Correção achada (evaluator, verificada empiricamente):** fórmula de reflect do design.md tava errada — invertia o `Kind()` (tratava `*S` válido como struct) e causava panic não-recuperável no caso `T=S` que devia gerar o panic limpo do framework. Fórmula corrigida documentada em design.md.

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add MustResolve placeholder allocation via reflect`

---

### T7: Resolução escopada por módulo (busca + Export) ✅ DONE (evaluator: PASS, commit `492fc37`)

**What**: implementa a busca real de `MustResolve`: próprio módulo primeiro, depois imports que exportam o tipo. Resolve as pending edges de T6 em edges reais (`providerNode → providerNode`). Gera as 2 mensagens de erro distintas (não registrado em lugar nenhum vs existe mas não exportado). **Escopo maior que o nome sugere** — fecha 3 débitos deixados propositalmente por T3/T4/T5 (ver notas nessas tasks e L-003/L-004 em STATE.md), porque a busca real não funciona sem eles:
1. `internal/module`: expor accessors (`OwnProviders()`, `Imports()`, `Exports()` ou equivalente) — hoje os campos são privados, T7 não consegue nem percorrer a árvore sem isso.
2. `internal/module`: alargar `ProviderRef`/`ControllerRef` com um jeito de obter o tipo resolvido (algo tipo `ResolvedType() reflect.Type`) — hoje são marker interfaces vazias, não dá pra responder "esse Provider resolve o tipo X".
3. Fechar o wiring de `OwnerModule`: `Stage 1` (assemble, em `internal/module`) precisa chamar `SetOwnerModule` de verdade nos providers/controllers registrados — hoje isso só existe se alguém chamar manualmente (gap documentado em T4/T5).

**Where**: `internal/module/module.go` (accessors + interfaces alargadas + wiring de Stage 1), `internal/provider/provider.go` (implementa o método de tipo resolvido), `internal/controller/controller.go` (idem, se aplicável), `internal/resolver/resolver.go` (busca em si, novo pacote)
**Depends on**: T6
**Reuses**: árvore de módulos de T3, pending edges de T6, `SetOwnerModule`/`OwnerModule` de T4/T5
**Requirement**: DI-10, DI-09

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `internal/module` expõe accessors suficientes pra `internal/resolver` percorrer providers/imports/exports de um `*Module` sem acessar campo privado
- [ ] `ProviderRef` alargado permite obter o `reflect.Type` que o provider resolve
- [ ] Stage 1 (assemble) chama `SetOwnerModule` de verdade em cada provider/controller registrado — `OwnerModule()` retorna não-nil sem chamada manual, usando só `NewModule(...)` normal
- [ ] Tipo resolvido no próprio módulo tem prioridade sobre imports
- [ ] Tipo só em módulo importado E exportado resolve corretamente
- [ ] Tipo em módulo importado mas NÃO exportado → panic `"gonest: type *X exists in module Y but is not exported"`
- [ ] Tipo em lugar nenhum → panic `"gonest: no provider registered for type *X"`
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 8+ (accessors, tipo resolvido exposto, wiring automático, prioridade local, import exportado ok, import não-exportado erro, tipo inexistente erro, diamante de imports)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): resolve MustResolve targets scoped by module + exports`

---

### T8: Detecção de ciclo (DFS com cadeia completa) ✅ DONE (evaluator: PASS, commit `2a306d2`)

**What**: T7 não monta grafo persistente — só resolve sob demanda (`resolver.Find`). T8 precisa primeiro **construir** o grafo real de dependências entre providers, combinando as pending edges de T6 (`internal/resolve`, formato `{owner, targetType}`) com `resolver.Find` (resolve `targetType` → `ProviderRef` real) — produzindo edges `providerRef → providerRef`. Só depois disso roda DFS com 3 cores sobre esse grafo; ao achar ciclo, reconstrói a cadeia completa (`A -> B -> C -> A`) pra mensagem de erro.
**Where**: `internal/resolver/resolver.go` (ou arquivo novo `internal/resolver/graph.go` no mesmo pacote, à critério de quem implementar)
**Depends on**: T7
**Reuses**: `resolver.Find` de T7, pending edges de `internal/resolve` (T6)
**Requirement**: DI-08

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Grafo de dependências entre providers é construído combinando pending edges (T6) + `Find` (T7) — não reimplementa a busca, reusa
- [ ] Grafo sem ciclo passa sem erro
- [ ] Ciclo direto (A→B→A) retorna erro `"circular dependency: A -> B -> A"`
- [ ] Ciclo indireto (A→B→C→A) retorna cadeia completa, não só "ciclo achado"
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 5+ (construção do grafo, sem ciclo, ciclo direto, ciclo indireto, self-loop A→A)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add cycle detection with full dependency chain in error`

---

### T9: Stage 3 — resolução paralela + NewApp mínimo (MVP) ✅ DONE (evaluator: PASS-WITH-NOTE, commit `b102180`)

**What**: motor de resolução via `errgroup` — cada `providerNode` (Singleton) tem goroutine própria que espera `deps` (channel `done` por node), roda `Constructor` (com `ctx` de timeout quando a assinatura pedir), copy-in-place no placeholder, fecha `done`. Erro/panic em qualquer `Constructor` cancela o `context.Context` compartilhado. `NewApp(root *Module) (*App, error)` / `MustNewApp` chamam Stage 1→2→3 nessa ordem e devolvem erro/panic conforme design. **Sem `Listen`/`AppOptions` completo ainda** — isso é escopo da feature "App Bootstrap & Listen"; aqui só o suficiente pra grafo resolver e o teste de integração do exemplo `UserProvider`/`UserService` do INSIGHT.md compilar e passar.

**Débito adicional achado (igual T7/T8, mais um elo faltando na corrente):** nada hoje executa o `fn` adiado de `Provider`/`Controller` fora de teste (`internal/provider`/`internal/controller` só têm o helper de teste `runFn`, não-exportado). Stage 1 (`module.Assemble`) roda só o `fn` do *Module* — nunca os `fn` de Provider/Controller registrados nele. T9 precisa expor um jeito de rodar isso (Stage 2 de verdade) antes de poder montar o grafo (T6-T8 dependem de `MustResolve` já ter sido chamado dentro desses `fn`, o que só acontece se alguém rodar o `fn`).

**Where**: `internal/provider/provider.go` (expõe execução do `fn`), `internal/controller/controller.go` (idem), `internal/resolver/resolver.go` (motor de Stage 3) + `app.go` na raiz (`NewApp`/`MustNewApp`, orquestra Stage 1→2→3)
**Depends on**: T8
**Reuses**: grafo + DFS de T7/T8, `errgroup.WithContext`, `module.Assemble` (Stage 1)
**Requirement**: DI-01 (1-5 completo), DI-02, DI-03, DI-04

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] 2 providers independentes resolvem em paralelo (teste mede que não serializa — timestamps de início próximos, ou canal de sincronização forçando ambos rodarem concorrentemente)
- [ ] Provider dependente só roda `Constructor` depois que sua dependência fecha `done`
- [ ] `Constructor(ctx context.Context)` recebe ctx com timeout de bootstrap configurável
- [ ] `Constructor` retornando `error` cancela as demais goroutines em andamento (teste: goroutine cancelada não termina de rodar até o fim)
- [ ] `Constructor` panicando é recuperado (`recover()`) e tratado como erro, não derruba o processo de teste
- [ ] Copy-in-place: placeholder devolvido em `MustResolve` reflete os dados reais após `NewApp` retornar
- [ ] Exemplo `UserProvider`/`UserService` (adaptado do INSIGHT.md, sem HTTP) compila e resolve via `MustResolve[*UserService]`
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 7+ (paralelismo real, ordem de dependência, ctx timeout, erro cancela, panic recuperado, copy-in-place, exemplo end-to-end)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): implement parallel Stage 3 resolution and minimal NewApp`

**Fix pós-task (commit `0f6871b`, evaluator PASS):** `resolve.Reset()` exportado + chamado no início de `NewApp` — fecha o estado global de pending edges achado pelo evaluator do T9 (ver L-006 em STATE.md).

---

### T10: Scope Transient ✅ DONE (evaluator: PASS-WITH-NOTE, commit `36b2a5c`)

**What**: **mais complexo do que o nome sugere** — Stage 3 (T9) hoje roda 1 goroutine por **Provider** (nó do grafo), resolve 1 vez, copia a mesma instância em TODOS os placeholders que apontam pra ele (`placeholdersFor`). Isso é semântica Singleton embutida na estrutura. Providers `ScopeTransient` precisam de semântica por **pending edge** (por chamada de `MustResolve`), não por nó: cada edge que aponta pra um provider Transient dispara sua própria execução de `Constructor`, resultando em instância própria só pra aquele placeholder — nunca compartilhada com outra edge, mesmo que aponte pro mesmo provider. Dependências do provider transient (se ele mesmo depender de outros providers) continuam resolvendo 1 vez e compartilhadas normalmente, a menos que também sejam Transient.
**Where**: `internal/resolver/stage3.go` (ou arquivo novo no mesmo pacote, ex. `transient.go`, à critério de quem implementar — mas a mudança toca a lógica de resolução em si, não é isolável num arquivo separado sem tocar `stage3.go`)
**Depends on**: T9 (+ fix `0f6871b`)
**Reuses**: motor de T9 (`errgroup`, canais `done`), `resolve.PendingEdges()` já escopado por `NewApp` (pós-fix)
**Requirement**: DI-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Providers Transient resolvem por edge, não por nó — 2 pending edges diferentes apontando pro mesmo provider Transient disparam 2 execuções de `Constructor`, cada uma copiada só no seu próprio placeholder
- [ ] Duas resoluções do mesmo provider transient devolvem ponteiros diferentes
- [ ] Dependência singleton desse transient é a mesma instância nas duas resoluções
- [ ] Providers Singleton continuam com a semântica de T9 intacta (1 execução, compartilhada) — teste de regressão confirma que T10 não quebrou o comportamento default
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 4+ (resolução por edge pra Transient, instâncias diferentes, dependência singleton igual, regressão do comportamento Singleton)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Transient scope resolution`

---

### T11: Scope Request (mecanismo isolado, sem Pipeline) ✅ DONE (evaluator: PASS, commit `5beaeb3`)

**Nota:** `Where` original citava `scope_request.go` na raiz (re-export "se aplicável") — dev decidiu não expor, sem consumidor real ainda (Pipeline é feature futura), evaluator concordou como decisão defensável (spec.md P3 só exige o mecanismo existir/ser testável, não exposição pública).

**What**: interface `RequestScope` mínima (contexto mockável, sem acoplar ao HTTP Pipeline real — isso chega no Milestone 3) que permite resolver o mesmo provider `ScopeRequest` como mesma instância dentro do "contexto" e instância diferente entre contextos distintos.
**Where**: `internal/resolver/request.go` (arquivo novo, não mexe em `resolver.go`/`transient.go`) + `scope_request.go` na raiz (re-export, se aplicável)
**Depends on**: T10 (mesmo pacote `internal/resolver` que T10 acabou de tocar — sequencial evita colisão de arquivo/import no mesmo pacote, ver L-003)
**Reuses**: motor de T9
**Requirement**: DI-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `RequestScope` interface definida e documentada como stub (comentário explícito: "wiring real com HTTP Pipeline é feature futura")
- [ ] Duas resoluções no mesmo "request context" mockado devolvem mesma instância
- [ ] Duas resoluções em "request contexts" diferentes devolvem instâncias diferentes
- [ ] Gate check passa (comando abaixo)
- [ ] Test count: 3+ (interface existe, mesmo context = mesma instância, contexts diferentes = instâncias diferentes)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Request scope mechanism (pipeline wiring deferred)`

---

## Parallel Execution Map

```
Phase 1 (Sequential):
  T1 ──→ T2

Phase 2 (T3 sequential, depois T4/T5 paralelo — pacotes Go diferentes):
  T2 complete, then:
    T3 (internal/module — define module.Owner, bloqueia T4/T5)
  T3 complete, then:
    ├── T4 [P] (internal/provider)
    └── T5 [P] (internal/controller)  } Pacotes diferentes, sem colisão de tipo

Phase 3 (Sequential):
  T4, T5 complete, then:
    T6 ──→ T7 ──→ T8 ──→ T9

Phase 4 (Sequential — mesmo pacote internal/resolver, ver L-003):
  T9 complete, then:
    T10 ──→ T11
```

**Papéis por task (AD-001 em STATE.md):** cada task acima é executada por um sub-agent **developer**; ao reportar Complete, um sub-agent **evaluator** separado confere o checklist "Done when" + roda o Gate check antes de marcar `[x]` nesta lista.

---

## Task Granularity Check

| Task                         | Scope                                                                                                                   | Status                |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------- | --------------------- |
| T1: Setup go.mod             | 1 arquivo de config                                                                                                     | ✅ Granular            |
| T2: Scope enum + ownerModule | 1 arquivo, 2 tipos coesos                                                                                               | ✅ Granular            |
| T3: Module declarativo       | 1 arquivo + Stage 1 assembly                                                                                            | ✅ Granular            |
| T4: Provider declarativo     | 1 arquivo                                                                                                               | ✅ Granular            |
| T5: Controller shell         | 1 arquivo                                                                                                               | ✅ Granular            |
| T6: MustResolve placeholder  | 1 arquivo, 1 função genérica                                                                                            | ✅ Granular            |
| T7: Resolução escopada       | 1 função (busca) no resolver                                                                                            | ✅ Granular            |
| T8: Detecção de ciclo        | 1 função (DFS) no resolver                                                                                              | ✅ Granular            |
| T9: Stage 3 + NewApp mínimo  | 2 arquivos, 1 motor coeso — maior task do conjunto, mas indivisível (paralelismo/erro/ctx são a mesma responsabilidade) | ✅ Granular (cohesive) |
| T10: Transient scope         | 1 função no resolver                                                                                                    | ✅ Granular            |
| T11: Request scope           | 2 arquivos pequenos                                                                                                     | ✅ Granular            |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows      | Status  |
| ---- | ---------------------- | ------------------ | ------- |
| T1   | None                   | None               | ✅ Match |
| T2   | T1                     | T1 → T2            | ✅ Match |
| T3   | T2                     | T2 → T3 (Phase 2)  | ✅ Match |
| T4   | T3                     | T3 → T4 (Phase 2)  | ✅ Match |
| T5   | T3                     | T3 → T5 (Phase 2)  | ✅ Match |
| T6   | T4, T5                 | T4,T5 → T6         | ✅ Match |
| T7   | T6                     | T6 → T7            | ✅ Match |
| T8   | T7                     | T7 → T8            | ✅ Match |
| T9   | T8                     | T8 → T9            | ✅ Match |
| T10  | T9                     | T9 → T10 (Phase 4) | ✅ Match |
| T11  | T10                    | T10 → T11 (Phase 4)| ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified          | Matrix Requires | Task Says | Status |
| ---- | ------------------------------------ | --------------- | --------- | ------ |
| T1   | infra (go.mod)                       | none            | none      | ✅ OK   |
| T2   | Builders públicos (Scope)            | unit            | unit      | ✅ OK   |
| T3   | Builders públicos (Module)           | unit            | unit      | ✅ OK   |
| T4   | Builders públicos (Provider)         | unit            | unit      | ✅ OK   |
| T5   | Builders públicos (Controller shell) | unit            | unit      | ✅ OK   |
| T6   | Motor interno (MustResolve)          | unit            | unit      | ✅ OK   |
| T7   | Motor interno (resolver)             | unit            | unit      | ✅ OK   |
| T8   | Detecção de ciclo                    | unit            | unit      | ✅ OK   |
| T9   | Motor interno (Stage 3)              | unit            | unit      | ✅ OK   |
| T10  | Motor interno (scope transient)      | unit            | unit      | ✅ OK   |
| T11  | Motor interno (scope request)        | unit            | unit      | ✅ OK   |

Nenhuma violação — projeto não tem camada e2e/integration nesta feature (100% biblioteca Go, sem HTTP ainda).
