# Provider & DI Graph Tasks

**Design**: `.specs/features/provider-di-graph/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

### Phase 1: Foundation (Sequential)

```
T1 → T2
```

### Phase 2: Structural Types (Parallel OK)

```
      ┌→ T3 ─┐
T2 ───┼→ T4 ─┼──→ T6
      └→ T5 ─┘
```

### Phase 3: Resolution Engine (Sequential)

```
T6 → T7 → T8 → T9
```

### Phase 4: Scopes (Parallel OK)

```
      ┌→ T10
T9 ───┤
      └→ T11
```

---

## Task Breakdown

### T1: Setup go.mod + dependências

**What**: cria `go.mod` (`module github.com/gonest-dev/gonest`), adiciona `golang.org/x/sync/errgroup` e `github.com/stretchr/testify`.
**Where**: `go.mod`, `go.sum`
**Depends on**: None
**Reuses**: —
**Requirement**: infra (não mapeado a DI-XX)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `go.mod` existe com module path correto e versão Go definida
- [ ] `go build ./...` roda sem erro (mesmo sem código ainda, só valida módulo)
- [ ] `go get golang.org/x/sync/errgroup github.com/stretchr/testify` resolve sem erro

**Tests**: none
**Gate**: full (`go test ./... -race` — sem testes ainda, só confirma que roda sem erro de build)

**Commit**: `chore(setup): initialize go.mod with core deps`

---

### T2: Definir Scope enum + interface ownerModule

**What**: define `type Scope int` (`ScopeSingleton`, `ScopeTransient`, `ScopeRequest`) e a interface interna `interface{ ownerModule() *Module }` usada por `MustResolve`.
**Where**: `scope.go`
**Depends on**: T1
**Reuses**: —
**Requirement**: DI-01 (base pra scope), DI-06, DI-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Scope` exportado com os 3 valores, `String()` implementado (debug-friendly)
- [ ] Interface `ownerModule` definida (não-exportada, mecanismo interno)
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 3+ testes (um por valor de `Scope.String()`)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Scope enum and ownerModule contract`

---

### T3: Module — API declarativa (Stage 1) [P]

**What**: `NewModule(fn)` guarda `fn` sem executar; `Module.Imports/Providers/Controllers/Exports` registram referências; função interna `assemble(root *Module) (*moduleTree, error)` faz o BFS/DFS de Stage 1 e valida que todo `Exports` é subconjunto de `Providers`.
**Where**: `module.go`, `internal/assemble.go`
**Depends on**: T2
**Reuses**: `Scope` de T2 só pro tipo, sem uso direto ainda
**Requirement**: DI-10 (parte: export inválido)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewModule(fn)` NÃO executa `fn` na chamada (teste explícito: flag booleana só vira true depois de `assemble`)
- [ ] `assemble` percorre `Imports` recursivamente sem duplicar módulo visitado 2x (grafo pode ter diamante)
- [ ] `Exports` de provider não declarado em `Providers` do mesmo módulo → erro `"module X exports provider *Y it does not declare"`
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 5+ (fn adiado, BFS simples, import diamante, export inválido, export válido)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Module declarative API and Stage 1 assembly`

---

### T4: Provider — API declarativa (Stage 2 shell) [P]

**What**: `NewProvider(fn)` guarda `fn` sem executar; `Provider.Scope(s)`, `Provider.Constructor(fn any)` armazenam config. `Constructor` aceita via reflect as 4 assinaturas do design (`func() T`, `func() (T, error)`, `func(context.Context) T`, `func(context.Context) (T, error)`) — validação de assinatura acontece aqui, execução real fica pra T9.
**Where**: `provider.go`
**Depends on**: T2
**Reuses**: `Scope` de T2
**Requirement**: DI-01 (parte declarativa)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewProvider(fn)` não executa `fn` na chamada
- [ ] `Constructor` aceita as 4 assinaturas válidas via reflect, panica em tempo de declaração se a assinatura não bater com nenhuma (`"gonest: invalid Constructor signature"`)
- [ ] `Scope` default é `ScopeSingleton` se não chamado explicitamente
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 6+ (fn adiado, 4 assinaturas válidas, 1 assinatura inválida, default scope)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Provider declarative API with Constructor signature validation`

---

### T5: Controller — shell mínimo pra MustResolve [P]

**What**: `NewController(fn)` guarda `fn` sem executar; implementa só o suficiente pra satisfazer `ownerModule()` (Path/Route/Handler ficam pra feature "Controller & Route Registration" — aqui é só o shell que permite `MustResolve` funcionar dentro do `fn` do Controller).
**Where**: `controller.go`
**Depends on**: T2
**Reuses**: mesmo padrão de adiamento de `fn` do T3/T4
**Requirement**: DI-01 (Controller como consumidor)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewController(fn)` não executa `fn` na chamada
- [ ] `ownerModule()` retorna o módulo dono depois que `assemble` (T3) processa esse Controller
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 2+ (fn adiado, ownerModule populado pós-assemble)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add minimal Controller shell for MustResolve consumers`

---

### T6: MustResolve[T] — placeholder via reflect

**What**: `MustResolve[T any](owner ownerModuleGetter) T` — valida que `T` é ponteiro (panic claro se não for), aloca placeholder via `reflect.New(...).Elem().Elem()`, registra a chamada como "pending edge" (owner → tipo T) numa lista interna consultada no Stage 2.
**Where**: `resolve.go`
**Depends on**: T3, T4, T5
**Reuses**: `ownerModule()` de T3/T4/T5
**Requirement**: DI-05, DI-09 (parte: panic de tipo não-ponteiro é achado aqui, "não registrado" só fecha em T7)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `MustResolve[*Foo](owner)` devolve ponteiro não-nil (placeholder alocado) sem panicar quando `T` é ponteiro válido
- [ ] `MustResolve[Foo](owner)` (tipo não-ponteiro) panica com `"gonest: MustResolve[T] requires T to be a pointer type, got Foo"`
- [ ] Cada chamada registra exatamente 1 pending edge, consultável por um helper interno de teste
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 4+ (placeholder válido, panic tipo errado, edge registrada, múltiplas chamadas não colidem)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add MustResolve placeholder allocation via reflect`

---

### T7: Resolução escopada por módulo (busca + Export)

**What**: implementa a busca real de `MustResolve`: próprio módulo primeiro, depois imports que exportam o tipo. Resolve as pending edges de T6 em edges reais (`providerNode → providerNode`). Gera as 2 mensagens de erro distintas (não registrado em lugar nenhum vs existe mas não exportado).
**Where**: `internal/resolver.go`
**Depends on**: T6
**Reuses**: `moduleTree` de T3, pending edges de T6
**Requirement**: DI-10, DI-09

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Tipo resolvido no próprio módulo tem prioridade sobre imports
- [ ] Tipo só em módulo importado E exportado resolve corretamente
- [ ] Tipo em módulo importado mas NÃO exportado → panic `"gonest: type *X exists in module Y but is not exported"`
- [ ] Tipo em lugar nenhum → panic `"gonest: no provider registered for type *X"`
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 5+ (prioridade local, import exportado ok, import não-exportado erro, tipo inexistente erro, diamante de imports)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): resolve MustResolve targets scoped by module + exports`

---

### T8: Detecção de ciclo (DFS com cadeia completa)

**What**: DFS com 3 cores sobre o grafo de `providerNode`s montado em T7; ao achar ciclo, reconstrói a cadeia completa (`A -> B -> C -> A`) pra mensagem de erro.
**Where**: `internal/resolver.go`
**Depends on**: T7
**Reuses**: grafo de edges de T7
**Requirement**: DI-08

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Grafo sem ciclo passa sem erro
- [ ] Ciclo direto (A→B→A) retorna erro `"circular dependency: A -> B -> A"`
- [ ] Ciclo indireto (A→B→C→A) retorna cadeia completa, não só "ciclo achado"
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 4+ (sem ciclo, ciclo direto, ciclo indireto, self-loop A→A)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add cycle detection with full dependency chain in error`

---

### T9: Stage 3 — resolução paralela + NewApp mínimo (MVP)

**What**: motor de resolução via `errgroup` — cada `providerNode` (Singleton) tem goroutine própria que espera `deps` (channel `done` por node), roda `Constructor` (com `ctx` de timeout quando a assinatura pedir), copy-in-place no placeholder, fecha `done`. Erro/panic em qualquer `Constructor` cancela o `context.Context` compartilhado. `NewApp(root *Module) (*App, error)` / `MustApp` chamam Stage 1→2→3 nessa ordem e devolvem erro/panic conforme design. **Sem `Listen`/`AppOptions` completo ainda** — isso é escopo da feature "App Bootstrap & Listen"; aqui só o suficiente pra grafo resolver e o teste de integração do exemplo `UserProvider`/`UserService` do INSIGHT.md compilar e passar.
**Where**: `app.go`, `internal/resolver.go`
**Depends on**: T8
**Reuses**: grafo + DFS de T7/T8, `errgroup.WithContext`
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
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 7+ (paralelismo real, ordem de dependência, ctx timeout, erro cancela, panic recuperado, copy-in-place, exemplo end-to-end)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): implement parallel Stage 3 resolution and minimal NewApp`

---

### T10: Scope Transient [P]

**What**: providers `ScopeTransient` rodam `Constructor` a cada `MustResolve` (não reusam instância); dependências singleton continuam compartilhadas.
**Where**: `internal/resolver.go`
**Depends on**: T9
**Reuses**: motor de T9, só muda o critério de "já resolvido" por scope
**Requirement**: DI-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Duas resoluções do mesmo provider transient devolvem ponteiros diferentes
- [ ] Dependência singleton desse transient é a mesma instância nas duas resoluções
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 2+ (instâncias diferentes, dependência singleton igual)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Transient scope resolution`

---

### T11: Scope Request (mecanismo isolado, sem Pipeline) [P]

**What**: interface `RequestScope` mínima (contexto mockável, sem acoplar ao HTTP Pipeline real — isso chega no Milestone 3) que permite resolver o mesmo provider `ScopeRequest` como mesma instância dentro do "contexto" e instância diferente entre contextos distintos.
**Where**: `scope_request.go`, `internal/resolver.go`
**Depends on**: T9
**Reuses**: motor de T9
**Requirement**: DI-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `RequestScope` interface definida e documentada como stub (comentário explícito: "wiring real com HTTP Pipeline é feature futura")
- [ ] Duas resoluções no mesmo "request context" mockado devolvem mesma instância
- [ ] Duas resoluções em "request contexts" diferentes devolvem instâncias diferentes
- [ ] Gate check passa: `go test ./... -race`
- [ ] Test count: 3+ (interface existe, mesmo context = mesma instância, contexts diferentes = instâncias diferentes)

**Tests**: unit
**Gate**: full

**Commit**: `feat(di): add Request scope mechanism (pipeline wiring deferred)`

---

## Parallel Execution Map

```
Phase 1 (Sequential):
  T1 ──→ T2

Phase 2 (Parallel):
  T2 complete, then:
    ├── T3 [P]
    ├── T4 [P]  } Podem rodar simultâneas — tipos independentes
    └── T5 [P]

Phase 3 (Sequential):
  T3, T4, T5 complete, then:
    T6 ──→ T7 ──→ T8 ──→ T9

Phase 4 (Parallel):
  T9 complete, then:
    ├── T10 [P]
    └── T11 [P]  } Scopes independentes entre si
```

**Papéis por task (AD-001 em STATE.md):** cada task acima é executada por um sub-agent **developer**; ao reportar Complete, um sub-agent **evaluator** separado confere o checklist "Done when" + roda o Gate check antes de marcar `[x]` nesta lista.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Setup go.mod | 1 arquivo de config | ✅ Granular |
| T2: Scope enum + ownerModule | 1 arquivo, 2 tipos coesos | ✅ Granular |
| T3: Module declarativo | 1 arquivo + Stage 1 assembly | ✅ Granular |
| T4: Provider declarativo | 1 arquivo | ✅ Granular |
| T5: Controller shell | 1 arquivo | ✅ Granular |
| T6: MustResolve placeholder | 1 arquivo, 1 função genérica | ✅ Granular |
| T7: Resolução escopada | 1 função (busca) no resolver | ✅ Granular |
| T8: Detecção de ciclo | 1 função (DFS) no resolver | ✅ Granular |
| T9: Stage 3 + NewApp mínimo | 2 arquivos, 1 motor coeso — maior task do conjunto, mas indivisível (paralelismo/erro/ctx são a mesma responsabilidade) | ✅ Granular (cohesive) |
| T10: Transient scope | 1 função no resolver | ✅ Granular |
| T11: Request scope | 2 arquivos pequenos | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | None | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | T2 | T2 → T3 (Phase 2) | ✅ Match |
| T4 | T2 | T2 → T4 (Phase 2) | ✅ Match |
| T5 | T2 | T2 → T5 (Phase 2) | ✅ Match |
| T6 | T3, T4, T5 | T3,T4,T5 → T6 | ✅ Match |
| T7 | T6 | T6 → T7 | ✅ Match |
| T8 | T7 | T7 → T8 | ✅ Match |
| T9 | T8 | T8 → T9 | ✅ Match |
| T10 | T9 | T9 → T10 (Phase 4) | ✅ Match |
| T11 | T9 | T9 → T11 (Phase 4) | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | infra (go.mod) | none | none | ✅ OK |
| T2 | Builders públicos (Scope) | unit | unit | ✅ OK |
| T3 | Builders públicos (Module) | unit | unit | ✅ OK |
| T4 | Builders públicos (Provider) | unit | unit | ✅ OK |
| T5 | Builders públicos (Controller shell) | unit | unit | ✅ OK |
| T6 | Motor interno (MustResolve) | unit | unit | ✅ OK |
| T7 | Motor interno (resolver) | unit | unit | ✅ OK |
| T8 | Detecção de ciclo | unit | unit | ✅ OK |
| T9 | Motor interno (Stage 3) | unit | unit | ✅ OK |
| T10 | Motor interno (scope transient) | unit | unit | ✅ OK |
| T11 | Motor interno (scope request) | unit | unit | ✅ OK |

Nenhuma violação — projeto não tem camada e2e/integration nesta feature (100% biblioteca Go, sem HTTP ainda).
