# GraphQL Support — Tasks

**Spec**: `.specs/features/graphql-support/spec.md`
**Design**: `.specs/features/graphql-support/design.md`
**Status**: Draft

---

## Subagent Roles (ver .specs/project/STATE.md's "Subagent workflow convention")

Mesmo padrão de 3 papéis já validado nas features `request-response-split`
e `schema-value-support`:

- **Planner** — já rodou (esta sessão) pra produzir este `tasks.md`.
- **Implementer** — 1 subagente por task abaixo (ou por grupo `[P]` em
  paralelo). Recebe SÓ a definição da task, não as outras tasks nem o
  histórico da conversa.
- **Evaluator** — roda depois de CADA Implementer, antes da task virar
  `completed`. Roda o `Gate` de verdade (não confia só no relatório do
  Implementer), confere `Done when` item a item. Aprova ou devolve com
  motivo específico — nunca corrige código ele mesmo.

Todo prompt de Implementer deve incluir: a task inteira (What/Where/
Depends on/Reuses/Requirement/Done when/Tests/Gate/Commit), o trecho
relevante de `design.md`/`spec.md`/`context.md` referenciado, e — como
esta feature mexe em pacotes reais já existentes (`internal/controller`,
`internal/module`, `internal/emitter`, `internal/openapi/generate.go`,
`internal/app/app.go`) — os trechos reais lidos desses arquivos (não
deixar o Implementer adivinhar a shape atual; ele deve ler o arquivo real
primeiro, mesma regra já aplicada em `schema-value-support/tasks.md`).

Esta feature é maior que `schema-value-support` (nova dependência externa,
2 pacotes novos, 1 novo protocolo de transporte) — a Knowledge
Verification Chain (STATE.md) se aplica com força total em T3 (resolver
a API real de `graphql-go/graphql` via Context7/docs antes de escrever
código, não assumir a partir do design.md sozinho) e em T8/T9 (WebSocket
via Fiber v3, idem).

---

## Execution Plan

### Phase 1: Fundação declarativa (Resolver/Query/Mutation/Subscription — sem execução real ainda)

```
T1 -> T2 -> T3
```

### Phase 2: Geração de schema GraphQL a partir de Schema (depende de T3 pra ter o que gerar)

```
T3 -> T4 -> T5 -> T6
```

### Phase 3: Dispatch real de Query/Mutation via HTTP (depende de T6)

```
T6 -> T7
```

### Phase 4: Subscription — Emitter.Subscribe + transporte (paralelo entre si após T7, mas ambos dependem de T7 existir)

```
T7 -> T8
T7 -> T9[P] -> T10[P]
```

### Phase 5: Gate final

```
T8, T10 -> T11
```

---

## Task Breakdown

### T1: `internal/gqlresolver` — `Resolver` (shell, mirror de `Controller`)

**What**: Ler `internal/controller/controller.go` por inteiro primeiro (já parcialmente lido em Design — reler, pode ter mudado). Criar `internal/gqlresolver/resolver.go` com `Resolver` struct (`fn func(*Resolver)`, `owner *module.Module`, `declared bool`, mais slices `queries []*Query`, `mutations []*Mutation`, `subscriptions []*Subscription` — vazios até T2/T3), `New(fn) *Resolver`, `Declare()` (roda `fn` uma vez, idêntico a `Controller.Declare`), `IsResolver()` (marker method, mesma razão de `Controller.IsController()` ser exportado — Go liga métodos de interface não-exportados ao pacote declarante), `SetOwnerModule`/`OwnerModule`, `ResolveDirect`/`ResolveDirectAll` delegando a `internal/resolver.FindDirect`/`FindDirectAll` (reuso, NÃO reimplementar busca).
**Where**: `internal/gqlresolver/resolver.go`, `internal/gqlresolver/resolver_test.go`
**Depends on**: None
**Reuses**: `Controller`'s shape inteiro (design.md's Components: `Resolver`), `internal/resolver.FindDirect`/`FindDirectAll` sem mudança
**Requirement**: GQL-01

**Done when**:
- [ ] `internal/gqlresolver/resolver.go` existe, compila, sem depender de `graphql-go/graphql` ainda (essa dependência só entra em T4)
- [ ] Testes espelham `internal/controller/controller_test.go`'s casos equivalentes (`Declare` idempotente, `IsResolver` marker, `SetOwnerModule`/`OwnerModule`)
- [ ] `go test ./internal/gqlresolver/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/gqlresolver/...`)
**Commit**: `feat(gqlresolver): add Resolver shell (mirrors Controller)`

---

### T2: `internal/module` ganha `ResolverRef`/`Resolvers`/`OwnResolvers`

**What**: Ler `internal/module/module.go` por inteiro primeiro (`ControllerRef` interface na linha ~39, `Controllers`/`OwnControllers` na linha ~145/191 — já lido em Design, reler). Adicionar `ResolverRef` interface (mesmo shape de `ControllerRef`: `IsResolver()`, mais o que `module.Owner`/assembly precisarem — confirmar lendo `ControllerRef`'s definição completa, não assumir 1 método só), `Resolvers(rs ...ResolverRef)` método, `OwnResolvers() []ResolverRef` getter (cópia defensiva, mesmo padrão de `OwnControllers`). `internal/gqlresolver.Resolver` precisa satisfazer `ResolverRef` (confirmar com um teste de compilação, ex: `var _ module.ResolverRef = (*gqlresolver.Resolver)(nil)`).
**Where**: `internal/module/module.go`, `internal/module/module_test.go`
**Depends on**: T1
**Reuses**: `Controllers`/`OwnControllers`'s exact pattern
**Requirement**: GQL-01

**Done when**:
- [ ] `Module.Resolvers(rs ...module.ResolverRef)` e `Module.OwnResolvers() []module.ResolverRef` existem
- [ ] Teste equivalente a `TestModule_Controllers_RegistersControllers`/`TestModule_OwnControllers_ReturnsCopyNotInternalSlice` (linhas 152/197 de `module_test.go`), adaptado pra Resolvers
- [ ] `var _ module.ResolverRef = (*gqlresolver.Resolver)(nil)` compila em algum arquivo de teste
- [ ] `go test ./internal/module/... ./internal/gqlresolver/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/module/... ./internal/gqlresolver/...`)
**Commit**: `feat(module): add ResolverRef/Resolvers/OwnResolvers (mirrors Controllers)`

---

### T3: `Query`/`Mutation`/`Subscription` (declaração, sem execução)

**What**: Criar `internal/gqlresolver/query.go`, `mutation.go`, `subscription.go`. Decidir e IMPLEMENTAR a escolha deixada em aberto no design.md's Tech Decisions (`Query`/`Mutation` tipo único vs alias) — documentar a escolha no commit. `Query`/`Mutation`: `New(name string, fn func(*Query)) *Query` (roda `fn` imediatamente, mesmo padrão de `route.New`), `.Args(s *schema.Schema) *Query`, `.Returns(s *schema.Schema) *Query`, `.Handler(fn func(ctx *GraphqlContext) any) *Query`, getters (`Name()`, `ArgsSchema()`, `ReturnsSchema()`, `HandlerFunc()`). `Subscription`: mesma forma, mas `.Handler(fn func(ctx *GraphqlContext, emit func(any))) *Subscription` (assinatura DIFERENTE, D3). `Resolver.Query(name, fn)`/`Mutation(name, fn)`/`Subscription(name, fn)` (T1's `Resolver`) chamam esses `New`s e apendam nas slices já criadas em T1. `GraphqlContext` fica STUB nesta task (`Args() execution.Parseable` pode retornar nil por enquanto — implementação real em T7, já que precisa do dispatch HTTP existir).
**Where**: `internal/gqlresolver/{query,mutation,subscription,context}.go` + testes
**Depends on**: T2 (para `Resolver` já ter as slices e o `ResolverRef` já estar validado)
**Reuses**: `route.New`'s deferred-run-immediately shape
**Requirement**: GQL-01, GQL-05 (declaração, não a execução)

**Done when**:
- [ ] `resolver.Query("hello", func(q *gqlresolver.Query) { q.Handler(func(ctx *gqlresolver.GraphqlContext) any { return "world" }) })` compila e é recuperável via `resolver.OwnQueries()` (getter novo, mesmo padrão de `OwnRoutes`)
- [ ] Idem para `Mutation`/`Subscription`
- [ ] `go test ./internal/gqlresolver/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/gqlresolver/...`)
**Commit**: `feat(gqlresolver): add Query/Mutation/Subscription declaration API`

---

### T4: Dependência `graphql-go/graphql` + `internal/graphqlgen` — tipos escalares nativos

**What**: **Antes de escrever qualquer código**, resolver via Context7 (`resolve-library-id` + `query-docs` para `graphql-go/graphql`) OU documentação oficial (`pkg.go.dev/github.com/graphql-go/graphql`) a API real de `graphql.NewSchema`, `graphql.ObjectConfig`, `graphql.Fields`, `graphql.Field`, `graphql.FieldConfigArgument`, `graphql.ResolveParams`, `graphql.NewScalar`/`graphql.ScalarConfig` — NÃO assumir a partir do design.md sozinho (Knowledge Verification Chain, STATE.md). `go get github.com/graphql-go/graphql@latest`. Criar `internal/graphqlgen/scalar.go`: mapa formato-OpenAPI→nome-de-scalar-GraphQL para os formatos nativos listados no spec.md (`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`/`DateTime`/`Date`), função que dado um `[]*schema.PropertyBuilder` (via `FormatValue()`) retorna o conjunto DEDUPLICADO (por nome) de scalars nativos usados.
**Where**: `internal/graphqlgen/scalar.go` + teste, `go.mod`/`go.sum`
**Depends on**: T3 (nada de T3 é usado diretamente aqui, mas mantém a ordem sequencial do Phase 2 — `internal/graphqlgen` só faz sentido depois que há algo pra gerar)
**Reuses**: `PropertyBuilder.FormatValue()` (`internal/schema/schema.go`), dedup-by-name pattern do design.md
**Requirement**: GQL-03

**Done when**:
- [ ] `go.mod` lista `github.com/graphql-go/graphql` como dependência direta
- [ ] Teste prova: 2 `PropertyBuilder`s com `.Email()` produzem UM único scalar `Email` no conjunto deduplicado
- [ ] `go test ./internal/graphqlgen/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/graphqlgen/...`)
**Commit**: `feat(graphqlgen): add graphql-go/graphql dependency + native scalar mapping`

---

### T5: `PropertyBuilder.GraphqlScalar(name)` + dedup de Custom Scalars nomeados

**What**: Ler `internal/schema/schema.go`'s `PropertyBuilder` struct e `Custom`/`CustomFunc` (linhas ~181-189, ~566-594) por inteiro primeiro. Adicionar campo `graphqlScalar string` + método `GraphqlScalar(name string) *PropertyBuilder` (retorna `p`, mesmo padrão de todo outro modificador) + getter `GraphqlScalarValue() (string, bool)` (mesmo padrão bool-return de `MinValue`/`ItemRef`/etc). Estender `internal/graphqlgen/scalar.go` (T4) pra também coletar scalars nomeados via `GraphqlScalar`, deduplicados por nome junto com os nativos. Implementar a checagem de erro do spec.md AC4 (`Custom(fn)` sem `.GraphqlScalar(name)` usado por um Resolver → erro de build, não fabricar comportamento — a mensagem exata de erro é decisão do Implementer, seguir o tom das mensagens de panic já existentes em `schema.go`, ex: `"gonest: field already registered via Property"`).
**Where**: `internal/schema/schema.go` (campo + 2 métodos novos, nenhuma linha existente alterada), `internal/schema/schema_test.go`, `internal/graphqlgen/scalar.go`
**Depends on**: T4
**Reuses**: Todo o resto de `PropertyBuilder` inalterado
**Requirement**: GQL-04

**Done when**:
- [ ] `PropertyBuilder.Custom(fn).GraphqlScalar("ObjectID")` compila e `GraphqlScalarValue()` retorna `("ObjectID", true)`
- [ ] Teste prova dedup: 2 campos com `GraphqlScalar("ObjectID")` → 1 só `scalar ObjectID` no conjunto gerado
- [ ] Teste prova o erro do AC4 (`Custom` sem `GraphqlScalar`, usado em contexto de Resolver, produz erro/panic determinístico)
- [ ] `internal/schema/schema.go`'s suite existente passa sem regressão (nenhuma linha antiga tocada)
- [ ] `go test ./internal/schema/... ./internal/graphqlgen/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/schema/... ./internal/graphqlgen/...`)
**Commit**: `feat(schema): add GraphqlScalar(name) modifier for Custom(fn) fields`

---

### T6: `internal/graphqlgen.Build` — Schema → `graphql.Schema` real

**What**: Implementar `Build(queries []*gqlresolver.Query, mutations []*gqlresolver.Mutation, subscriptions []*gqlresolver.Subscription) (*graphql.Schema, error)`. Percorre cada `*schema.Schema` (via `.Args()`/`.Returns()`), lê `OwnProperties()`, converte cada `PropertyBuilder` (`KindValue()`/`FormatValue()`/`MinValue()`/`MaxValue()`/`PatternValue()`/`ItemBuilder()`/`ItemRef()`/`SchemaRef()`/`CustomFunc()`/`GraphqlScalarValue()`) em `graphql.Field`/`graphql.InputObjectFieldConfig` equivalente, usando os scalars nativos+nomeados de T4/T5. Monta `graphql.ObjectConfig{Name: "Query", Fields: ...}` (idem Mutation/Subscription) e `graphql.NewSchema(graphql.SchemaConfig{Query: ..., Mutation: ..., Subscription: ...})`. Retorna erro (não panic) pra qualquer mismatch de configuração detectado em build-time (T5's AC4 check, nomes duplicados entre Query/Mutation/Subscription).
**Where**: `internal/graphqlgen/generate.go` + `internal/graphqlgen/generate_test.go`
**Depends on**: T5
**Reuses**: Toda a leitura de `PropertyBuilder` já validada em T4/T5, dedup de `registerSchema` como referência (design.md)
**Requirement**: GQL-02, GQL-03, GQL-04

**Done when**:
- [ ] `Build` com 1 Query simples (`.Returns` de um `NewSchema[UserEntity]` com campo `Email` `.Email()`) produz um `*graphql.Schema` cujo SDL impresso (via `graphql-go`'s próprio printer, se existir, ou `graphql.PrintSchema` — confirmar API real) contém `scalar Email` e o campo tipado corretamente
- [ ] Reproduz o exemplo de `INSIGHT-GRAPHQL.md`'s seção "Branches de formato viram Custom Scalars" (spec.md's Independent Test, P1 story 2)
- [ ] `go test ./internal/graphqlgen/...` passa

**Tests**: unit + 1 golden-SDL-string test
**Gate**: quick (`go test ./internal/graphqlgen/...`)
**Commit**: `feat(graphqlgen): build graphql.Schema from gonest Schema declarations`

---

### T7: Dispatch real de Query/Mutation via HTTP + `GraphqlContext` real

**What**: Ler `internal/app/app.go`'s Stage 2.5 (`registerRoutes`, linhas ~451-560) por inteiro primeiro. Adicionar um passo equivalente: coletar `OwnResolvers()` de todo módulo, rodar `Declare()`, alimentar `internal/graphqlgen.Build`, registrar UMA `POST /graphql` via `adapter.RegisterRoute` já existente (endpoint fixo nesta task — configurabilidade é Tech Decision aberta, não bloqueia P1). O handler HTTP: decodifica o corpo da requisição GraphQL padrão (`{query, variables, operationName}` — formato HTTP de GraphQL é bem estabelecido, confirmar via Context7/docs se necessário), usa `graphql.Do(graphql.Params{...})` (ou API equivalente confirmada em T4) pra executar, escrevendo `{data, errors}` de volta via `execution.Response`. Implementar `GraphqlContext.Args()` de verdade agora (constrói um `Parseable` real a partir do `ResolveParams.Args` que graphql-go passa pro field resolver).
**Where**: `internal/app/app.go`, `internal/gqlresolver/context.go`, testes de integração (`internal/app/graphql_test.go` ou similar)
**Depends on**: T6
**Reuses**: `adapter.RegisterRoute` sem mudança, `gonest.MustParse[T]` sem mudança
**Requirement**: GQL-01, GQL-02

**Done when**:
- [ ] Uma Query real, via `app.Test` (mesmo padrão de dispatch de teste HTTP já usado pelo resto do framework), retorna o `data` esperado do exemplo de `INSIGHT-GRAPHQL.md`
- [ ] Um `Args` inválido produz um erro de validação equivalente ao REST (`gonest.MustParse` propagando, spec.md's Independent Test P1 story 1)
- [ ] `go test ./internal/app/... ./internal/gqlresolver/...` passa

**Tests**: integration (dispatch HTTP real)
**Gate**: quick (`go test ./internal/app/... ./internal/gqlresolver/...`)
**Commit**: `feat(app): wire real GraphQL Query/Mutation dispatch over POST /graphql`

---

### T8: `Emitter.Subscribe[T]`

**What**: Ler `internal/emitter/emitter.go` e `listener.go` por inteiro primeiro. Adicionar `subscribers map[reflect.Type][]chan any` (ou tipo equivalente — decisão do Implementer, documentar) ao `Emitter` struct, protegido pelo `mu` já existente. Implementar `func Subscribe[T any](e *Emitter, done <-chan struct{}) <-chan T` (free function, mesma razão de `MustOn[EventType]` ser livre): cria um canal `chan T`, registra em `subscribers`, dispara uma goroutine interna que bloqueia em `<-done` e então remove+fecha o canal (design.md's Component: "responsabilidade de fechar é do Subscribe"). Estender `Emit` (linha ~60-75) pra TAMBÉM encaminhar o evento a cada canal registrado em `subscribers[t]`, sem quebrar o comportamento existente de `listeners` (fire-and-forget, goroutine própria, recover already in place).
**Where**: `internal/emitter/subscribe.go` (novo arquivo), `internal/emitter/emitter.go` (extensão mínima do struct + `Emit`), `internal/emitter/subscribe_test.go`
**Depends on**: T7 (ordem sequencial do Phase 4 — na prática T8 não depende tecnicamente de T7, mas roda depois pra manter o app já tendo dispatch real antes de adicionar streaming)
**Reuses**: `Emitter`'s `mu`/`listeners` model, `Emit`'s existing recover-per-goroutine pattern
**Requirement**: GQL-05

**Done when**:
- [ ] Teste prova: `Subscribe[SomeEvent](e, done)` recebe todo `Emit(SomeEvent{...})` publicado depois da chamada
- [ ] Teste prova: fechar `done` fecha o canal retornado (um `range` sobre ele termina) SEM vazar goroutine — confirmar via contagem de goroutines antes/depois (`runtime.NumGoroutine()`, ou padrão de teste de concorrência já usado em outro lugar do repo — grep primeiro)
- [ ] Suite existente de `internal/emitter` passa sem regressão
- [ ] `go test ./internal/emitter/... -race` passa

**Tests**: unit + regressão de goroutine leak
**Gate**: quick (`go test ./internal/emitter/... -race`)
**Commit**: `feat(emitter): add Subscribe[T] dynamic channel subscription`

---

### T9 [P]: Transporte SSE para Subscription

**What**: Criar `internal/gqltransport/sse.go`. Handler HTTP (registrado via `adapter.RegisterRoute` já existente, endpoint `GET /graphql/stream` ou path decidido em T7 — confirmar consistência) que: recebe a query de subscription (via querystring, formato TBD — Implementer decide e documenta, inspirado no protocolo público `graphql-sse` referenciado em context.md sem precisar implementá-lo 1:1), chama o `Subscription.HandlerFunc()` correspondente passando um `emit func(any)` que escreve eventos formatados como SSE (`data: {...}\n\n`) direto na `execution.Response`'s stream (confirmar se `execution.Response` já suporta escrita incremental/flush — ler `internal/execution/response.go` primeiro; se não suportar, essa é uma extensão mínima necessária, documentar como SPEC_DEVIATION se for o caso), e um `ctx.Done()` (`GraphqlContext`) que fecha quando a conexão HTTP encerra.
**Where**: `internal/gqltransport/sse.go`, `internal/gqltransport/sse_test.go`
**Depends on**: T8
**Reuses**: `adapter.RegisterRoute` (sem mudança de interface), `Emitter.Subscribe[T]`, `execution.Response`
**Requirement**: GQL-07

**Done when**:
- [ ] Um client de teste (via `app.Test` com leitura incremental do body, ou helper equivalente) conectado via SSE recebe um evento publicado via `Emitter.Emit` depois da conexão
- [ ] Desconectar o client fecha o `ctx.Done()` passado ao Handler (spec.md AC5)
- [ ] Panic dentro do Handler não derruba o processo (design.md's Error Handling Strategy — recover na goroutine da conexão)
- [ ] `go test ./internal/gqltransport/...` passa

**Tests**: integration
**Gate**: quick (`go test ./internal/gqltransport/...`)
**Commit**: `feat(gqltransport): add SSE transport for GraphQL Subscription`

---

### T10 [P]: Transporte WebSocket para Subscription

**What**: **Antes de escrever código**, resolver via Context7/docs a API real de upgrade de WebSocket do Fiber v3 (ex: `github.com/gofiber/contrib/websocket` ou equivalente nativo v3 — NÃO assumir, confirmar o pacote real). Ler `internal/adapter/fiber/fiber.go` e a interface `HttpAdapter` (`internal/app/app.go:224-227`) por inteiro. Adicionar a capability decidida no design.md's Tech Decisions (`RegisterWebSocket(path string, h func(...)) error` — assinatura exata a confirmar contra a API real do Fiber) ao `HttpAdapter` e implementá-la em `internal/adapter/fiber`. Criar `internal/gqltransport/ws.go`: handler que faz upgrade, recebe mensagens de subscribe do client (protocolo simplificado, referência de compatibilidade em `graphql-ws`/`graphql-transport-ws` sem implementar 1:1 — documentar exatamente o subconjunto suportado), chama `Subscription.HandlerFunc()` com `emit` escrevendo frames WS, `ctx.Done()` fechando quando a conexão WS fecha.
**Where**: `internal/adapter/fiber/fiber.go` (extensão da interface + implementação), `internal/app/app.go` (interface `HttpAdapter`), `internal/gqltransport/ws.go`, testes
**Depends on**: T8
**Reuses**: `Emitter.Subscribe[T]`, mesmo padrão de recover-per-connection de T9
**Requirement**: GQL-06

**Done when**:
- [ ] Um client de teste WS conecta, assina uma Subscription, recebe um evento publicado via `Emitter.Emit`
- [ ] Desconectar fecha `ctx.Done()` (spec.md AC5)
- [ ] Um evento publicado chega a um client WS E a um client SSE (T9) simultaneamente, assinando a MESMA Subscription (spec.md's Independent Test, P2) -- desconectar um não afeta o outro
- [ ] `go test ./internal/adapter/fiber/... ./internal/gqltransport/... -race` passa

**Tests**: integration
**Gate**: quick (`go test ./internal/adapter/fiber/... ./internal/gqltransport/... -race`)
**Commit**: `feat(gqltransport): add WebSocket transport for GraphQL Subscription`

---

### T11: Gate final + STATE.md/ROADMAP.md + INSIGHT-GRAPHQL.md

**What**: Rodar suite completa. Atualizar `STATE.md` com o AD final (decisões tomadas DURANTE a execução -- Query/Mutation tipo único ou não, endpoint path, assinatura real de `RegisterWebSocket`, biblioteca de WS usada -- qualquer SPEC_DEVIATION do design.md). Atualizar `ROADMAP.md`'s Milestone 16 → COMPLETE. Atualizar `INSIGHT-GRAPHQL.md` (repo root) substituindo a reflexão especulativa pelo estado REAL implementado (mesmo processo já seguido por `unified-parse-api`/`request-response-split`/planejado para `schema-value-support`'s T6).
**Where**: raiz, `.specs/project/{STATE,ROADMAP}.md`, `.specs/features/graphql-support/spec.md`, `INSIGHT-GRAPHQL.md`
**Depends on**: T9, T10

**Done when**:
- [ ] `go test ./... -race` passa, zero falha nova
- [ ] `go build ./...` passa, `.examples/*` buildam (se algum exemplo novo de GraphQL foi adicionado, senão apenas confirmar que nada quebrou)
- [ ] `STATE.md` tem novo AD documentando a execução completa
- [ ] `ROADMAP.md`'s Milestone 16 → COMPLETE
- [ ] `spec.md`'s traceability table → todo GQL-0x → Verified
- [ ] `INSIGHT-GRAPHQL.md` reflete a implementação real (exemplo de código compilando de verdade)

**Tests**: integration (suite completa)
**Gate**: full (`go test ./... -race`)
**Commit**: `chore: finalize graphql-support feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Resolver shell | 1 arquivo novo + teste, mirror de Controller | ✅ |
| T2: Module.Resolvers/OwnResolvers | 1 interface + 2 métodos, mirror de Controllers | ✅ |
| T3: Query/Mutation/Subscription declaration | 4 arquivos novos + testes | ✅ |
| T4: graphql-go dependency + native scalars | 1 dependência + 1 arquivo + teste | ✅ |
| T5: GraphqlScalar(name) modifier | 2 métodos novos em arquivo existente + teste | ✅ |
| T6: graphqlgen.Build | 1 gerador completo + teste golden | ✅ |
| T7: HTTP dispatch real + GraphqlContext | integração Stage 2.5 + testes | ✅ |
| T8: Emitter.Subscribe[T] | 1 arquivo novo + extensão mínima + teste concorrência | ✅ |
| T9: SSE transport | 1 pacote novo (parcial) + teste | ✅ |
| T10: WebSocket transport | 1 pacote novo (parcial) + extensão de adapter + teste | ✅ |
| T11: Gate final | verificação + docs | ✅ |
