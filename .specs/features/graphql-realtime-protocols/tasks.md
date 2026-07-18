# GraphQL Realtime Protocols — Tasks

**Spec**: `.specs/features/graphql-realtime-protocols/spec.md`
**Context**: `.specs/features/graphql-realtime-protocols/context.md`
**Status**: Draft

**Nota sobre `design.md`**: `.specs/features/graphql-realtime-protocols/design.md` foi
escrito em paralelo (mesma sessão) e mesclado depois deste `tasks.md`. Conferido:
arquitetura idêntica (decisão central `HttpAdapter.RegisterWebSocket` REMOVIDO,
substituído por `execution.Response.UpgradeWebSocket`/`execution.Request.
IsWebSocketUpgrade`/`execution.WSConn`; mesmos componentes novos
`internal/graphql/{wsprotocol,ssedistinct,ssesingle,reservation}.go`). Sem divergência.

---

## Subagent Roles (ver `.specs/project/STATE.md`'s "Subagent workflow convention")

- **Planner** — já rodou (esta sessão) pra produzir este `tasks.md`. Não roda de novo
  pra esta feature a menos que o escopo mude.
- **Implementer** — 1 subagente por task abaixo (ou por grupo `[P]` em paralelo).
  Recebe SÓ a definição da task (não as outras tasks, não o histórico desta conversa).
- **Evaluator** — roda depois de CADA Implementer, antes da task virar `completed`.
  Recebe a definição da task + o diff real (nunca só o relatório do Implementer), roda
  o `Gate` de verdade, confere `Done when` item a item. Aprova ou devolve com motivo
  específico — nunca corrige código ele mesmo.

Todo prompt de Implementer deve incluir: a task inteira (What/Where/Depends
on/Reuses/Done when/Tests/Gate/Commit), `.specs/codebase/CONVENTIONS.md` (se
existir) + `.specs/codebase/TESTING.md`, e o trecho relevante de `spec.md`/`context.md`
referenciado pela task (este `tasks.md`'s nota acima sobre `design.md`, se ainda
ausente na hora da execução).

Toda API externa nova referenciada por uma task (ex: método real de fechar WS com
código em `github.com/gofiber/contrib/v3/websocket`/`github.com/fasthttp/websocket`)
deve ser CONFIRMADA via Context7/leitura de código-fonte vendorizado antes de
implementar — mesma Knowledge Verification Chain que AD-036/AD-037/AD-039 já
documentam em `STATE.md`, não assumida/fabricada.

---

## Execution Plan

### Track 0: Helper compartilhado (paraleliza com tudo)

```
T1 [P] (Execute helper, usado por T6, T9, T13, e pelo POST /graphql já existente)
```

### Track A: WebSocket (`graphql-transport-ws`) — Sequential

```
T2 -> T3 -> T4 -> T5 -> T6 -> T7 -> T8
```

### Track B: SSE Distinct connections — Sequential, paralelo à Track A

```
T9 [P] -> T10
```

### Track C: SSE Single connection — Sequential, paralelo às Tracks A/B

```
T11 [P] -> T12 -> T13 -> T14
```

### Merge: Wiring + limpeza + exemplo + gate final

```
T8 + T10 + T14 ─> T15 -> T16 -> T17 -> T18
```

---

## Task Breakdown

### T1: `internal/graphql.Execute` — helper de dispatch compartilhado [P]

**What**: Extrair de `internal/app/graphql.go`'s `graphqlHandler` a lógica de "rodar uma
operação GraphQL contra um `*gql.Schema` já construído e devolver `{data, errors}`" pra
uma função nova `Execute(sch *gql.Schema, query string, variables map[string]any,
operationName string) (data any, errors []map[string]any)` em `internal/graphql`
(arquivo novo `execute.go`). Hoje essa lógica (`gql.Do(gql.Params{...})` + o loop que
converte `result.Errors` pra `[]any` de `map[string]any{"message": ...}`) só existe
INLINE dentro de `graphqlHandler` (`internal/app/graphql.go` linhas ~138-150). Ela vai
ser reusada por 4 call sites depois desta feature: `POST /graphql` (já existe, T15
refatora pra chamar `Execute`), WS single-result operation (T6), SSE Distinct
Query/Mutation (T9), SSE Single connection `POST` de operação (T13) — centralizar agora
evita 4 cópias divergentes da mesma chamada `gql.Do`. `graphqlHandler` (`internal/app/
graphql.go`) é atualizado NESTA task pra chamar `graphql.Execute` em vez de `gql.Do`
inline, provando que o helper é comportamentalmente idêntico ao código que substitui
(teste existente de `internal/app/graphql_test.go` não pode quebrar).
**Where**: `internal/graphql/execute.go` (novo), `internal/app/graphql.go` (refatorado
pra usar o helper)
**Depends on**: None
**Reuses**: `gql.Do`/`gql.Params`/`gql.Schema` (já usados por `graphqlHandler`), o
mapeamento de erro já existente
**Requirement**: GQLRT-02 (base pra WS/SSE reusarem o mesmo dispatch de Query/Mutation)

**Done when**:
- [ ] `graphql.Execute(sch, query, variables, operationName) (data any, errors []map[string]any)` existe e tem exatamente o mesmo comportamento observável do `gql.Do` inline que substitui
- [ ] `internal/app/graphql.go`'s `graphqlHandler` chama `graphql.Execute` em vez de `gql.Do` diretamente
- [ ] `go test ./internal/app/... ./internal/graphql/... -race` passa sem nenhuma asserção alterada em `internal/app/graphql_test.go`

**Tests**: unit — `TestExecute_ValidQuery_ReturnsData`, `TestExecute_InvalidQuery_ReturnsErrors` (novos, `internal/graphql/execute_test.go`); `internal/app/graphql_test.go` existente continua passando inalterado (prova de equivalência comportamental)
**Gate**: quick (`go test ./internal/app/... ./internal/graphql/... -race`)
**Commit**: `refactor(graphql): extract Execute helper from POST /graphql handler`

---

### T2: Mover `WSConn` de `internal/graphql` pra `internal/execution`; adicionar `CloseWithCode`

**What**: Mover a definição de `WSConn` (hoje em `internal/graphql/ws.go`, interface
`ReadMessage`/`WriteMessage`/`Close`/`Params`/`Query`) pra `internal/execution` (arquivo
novo `wsconn.go`), adicionando um método novo `CloseWithCode(code int, reason string)
error` — necessário pros fechamentos com código específico do protocolo
`graphql-transport-ws` (4400/4401/4408/4409/4429, spec.md's Edge Cases). Em
`internal/graphql`, `type WSConn = execution.WSConn` vira um ALIAS de volta (não
remover o nome do pacote `graphql` inteiramente ainda — `ws.go`/`ws_test.go` antigos,
que só são removidos em T16, continuam referenciando `graphql.WSConn` até lá). NÃO
tocar `ws.go`/`sse.go`/`internal/adapter/fiber` nesta task — só a definição do tipo.
**Where**: `internal/execution/wsconn.go` (novo), `internal/graphql/ws.go` (a definição
de `WSConn`/`wsTextMessage` sai daqui, vira `type WSConn = execution.WSConn` — o resto
do arquivo, `WSHandler`, fica intacto até T16)
**Depends on**: None
**Reuses**: A definição já existente de `WSConn` (copy+trim, não reescrever do zero)
**Requirement**: GQLRT-01 (fundação pro handshake WS)

**Done when**:
- [ ] `execution.WSConn` definido com `ReadMessage`/`WriteMessage`/`Close`/`CloseWithCode`/`Params`/`Query`
- [ ] `graphql.WSConn` é `= execution.WSConn` (alias, não redefinição)
- [ ] `ws.go`'s `WSHandler`/`fakeWSConn` (em `ws_test.go`) continuam compilando sem qualquer outra mudança além de satisfazer o método novo `CloseWithCode` (`fakeWSConn` em `ws_test.go` ganha uma implementação mínima, ex: delega pra `Close()` ignorando `code`/`reason` — só precisa satisfazer a interface, comportamento real de fechamento com código chega em T5/T4)
- [ ] `go build ./internal/execution/... ./internal/graphql/...` passa

**Tests**: nenhum novo específico — `go test ./internal/graphql/... -race` (ws_test.go existente) continua passando com `fakeWSConn` atualizado
**Gate**: quick (`go test ./internal/execution/... ./internal/graphql/... -race`)
**Commit**: `refactor(execution): move WSConn from internal/graphql, add CloseWithCode`

---

### T3: `Request.IsWebSocketUpgrade()` / `Response.UpgradeWebSocket()` em `internal/execution`

**What**: Adicionar 2 métodos novos ao contrato `Responder` (`internal/execution/
request.go`): `IsUpgradeRequest() bool` e `Upgrade(handler func(conn WSConn))`.
Expor via `Request.IsWebSocketUpgrade() bool` (delega pra `req.res.IsUpgradeRequest()`)
e `Response.UpgradeWebSocket(handler func(conn execution.WSConn))` (delega pra
`res.res.Upgrade(handler)`) — mesma forma de delegação fina que todo outro método de
`Request`/`Response` já usa (`Method()`→`GetMethod()`, `Json()`→`JSON()`, etc). Como
`Responder` ganhou 2 métodos novos, TODA implementação existente da interface precisa
dos 2 métodos novos pra continuar compilando — atualizar as ~12 `fakeResponder`
espalhadas pelos arquivos de teste (`internal/validate/{validate,query,params,form}_test.go`,
`internal/route/route_test.go`, `internal/{middleware,interceptor,guard,filter}/
*_test.go`, `internal/execution/{response,request}_test.go`, `gonest_test.go`) com
implementações mínimas (`IsUpgradeRequest() bool { return false }`,
`Upgrade(handler func(conn execution.WSConn)) {}` — nenhuma delas testa WS de verdade,
só precisam satisfazer a interface). `internal/adapter/fiber/fiber.go`'s
`fiberResponder` NÃO ganha implementação real ainda (isso é T4) — se precisar compilar
antes de T4, adicionar um stub que panica (`panic("not implemented")`) só pra esta task
específica não quebrar o build do pacote `fiber`; T4 substitui pelo real.
**Where**: `internal/execution/request.go` (`Responder` interface, `Request.IsWebSocketUpgrade`), `internal/execution/response.go` (`Response.UpgradeWebSocket`), as ~12 `fakeResponder`/similares listadas acima, `internal/adapter/fiber/fiber.go` (stub temporário)
**Depends on**: T2 (precisa de `WSConn` já em `execution`)
**Reuses**: Padrão de delegação fina já usado por todo outro método de `Request`/`Response`
**Requirement**: GQLRT-01

**Done when**:
- [ ] `Responder` interface tem `IsUpgradeRequest() bool` e `Upgrade(handler func(conn WSConn))`
- [ ] `Request.IsWebSocketUpgrade() bool` e `Response.UpgradeWebSocket(handler func(conn execution.WSConn))` existem e delegam corretamente
- [ ] Toda `fakeResponder`/similar do repo (grep por `SetHeaderValue` como proxy pra achar todas as implementações de `Responder`) tem os 2 métodos novos
- [ ] `go build ./...` passa (incluindo o stub temporário em `fiber.go`)

**Tests**: unit — `TestRequest_IsWebSocketUpgrade_DelegatesToResponder`, `TestResponse_UpgradeWebSocket_DelegatesToResponder` (novos, `internal/execution/{request,response}_test.go`, usando uma `fakeResponder` local que grava se `IsUpgradeRequest`/`Upgrade` foram chamados)
**Gate**: full (`go test ./... -race`)
**Commit**: `feat(execution): Request.IsWebSocketUpgrade/Response.UpgradeWebSocket, extend Responder`

---

### T4: Fiber adapter — `fiberResponder.IsUpgradeRequest`/`Upgrade` reais, `fiberWSConn.CloseWithCode`; remover `HttpAdapter.RegisterWebSocket`

**What**: Substituir o stub temporário de T3 em `internal/adapter/fiber/fiber.go`'s
`fiberResponder` por implementação real: `IsUpgradeRequest() bool` chama
`websocket.IsWebSocketUpgrade(r.c)` (mesma função já usada pelo `RegisterWebSocket`
atual, confirmar via Context7 que segue disponível/mesma assinatura); `Upgrade(handler
func(conn execution.WSConn))` invoca `websocket.New(func(c *websocket.Conn) {
handler(&fiberWSConn{c: c}) })(r.c)` diretamente sobre `r.c` (CONFIRMAR via Context7 se
`websocket.New(fn)` retorna um `fiber.Handler` invocável assim, dentro de um handler já
em curso, em vez de precisar ser registrado via `app.Get` — essa é a mudança que permite
WS e SSE conviverem no mesmo `GET /graphql` sob o mesmo `RegisterRoute`, sem o
`app.Use(path, ...)` do `RegisterWebSocket` antigo que interceptava TODO método no
mesmo path, spec.md's Edge Case). `fiberWSConn.CloseWithCode(code int, reason string)
error` -- CONFIRMAR a API real de fechar com código de close em
`github.com/gofiber/contrib/v3/websocket`/`github.com/fasthttp/websocket` (candidato
mais provável: `c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code,
reason))` seguido de `c.Close()`, mesma forma que `gorilla/websocket` populariza — NÃO
assumir, confirmar via Context7/fonte vendorizada antes de codar). Remover
`RegisterWebSocket` de `HttpAdapter` (`internal/app/app.go`) e de `FiberApp`
(`internal/adapter/fiber/fiber.go`) por inteiro. Isso QUEBRA `internal/app/graphql.go`'s
`registerGraphql` (chama `adapter.RegisterWebSocket`) e `internal/graphql/ws.go`'s
consumidor implícito — quebra ESPERADA, documentada, resolvida em T15 (mesma convenção
de T5 em `request-response-split/tasks.md`, quebra transitória aceita entre tasks de uma
mesma track).
**Where**: `internal/adapter/fiber/fiber.go` (`fiberResponder.IsUpgradeRequest`/`Upgrade`
reais, `fiberWSConn.CloseWithCode`, remoção de `RegisterWebSocket`), `internal/app/app.go`
(remoção de `RegisterWebSocket` de `HttpAdapter`)
**Depends on**: T3
**Reuses**: `websocket.IsWebSocketUpgrade`/`websocket.New` já usados pelo `RegisterWebSocket` atual (mesma lib, uso diferente)
**Requirement**: GQLRT-01, GQLRT-07 (edge case de roteamento por método no mesmo path)

**Done when**:
- [ ] `fiberResponder.IsUpgradeRequest()`/`Upgrade(...)` reais (sem stub/panic)
- [ ] `fiberWSConn.CloseWithCode(code, reason) error` real, API confirmada via Context7 (não assumida)
- [ ] `HttpAdapter.RegisterWebSocket` não existe mais em `internal/app/app.go`
- [ ] `FiberApp.RegisterWebSocket` não existe mais em `internal/adapter/fiber/fiber.go`
- [ ] `go build ./internal/adapter/fiber/...` passa isoladamente
- [ ] `go build ./internal/app/...` FALHA neste ponto por causa de `registerGraphql` ainda chamando `RegisterWebSocket` (esperado — não é regressão desta task, ver T15)

**Tests**: integration — `fiber_test.go` ganha um teste de dial TCP real (mesmo padrão de `internal/app/app_test.go`'s `TestMustListen_RealFiberApp_IntegrationSmoke`/Milestone 12's lição sobre `app.Test` não servir pra streaming/upgrade) provando que uma request com header `Upgrade: websocket`/`Connection: Upgrade`/`Sec-WebSocket-*` corretos, batendo numa rota registrada via `RegisterRoute` cujo handler chama `res.UpgradeWebSocket(...)`, completa o handshake HTTP 101 de verdade (usar `github.com/fasthttp/websocket` -- já dependência transitiva do repo via `go.mod`, promover pra direta -- como client de teste, `Dialer.Dial` contra `ws://127.0.0.1:<porta>/...`)
**Gate**: quick (`go test ./internal/adapter/fiber/... -race`)
**Commit**: `refactor(adapter/fiber)!: real WS upgrade via Responder, remove RegisterWebSocket`

---

### T5: `wsprotocol.go` — handshake (`ConnectionInit`/`ConnectionAck`, timeouts, fechamentos)

**What**: Novo arquivo `internal/graphql/wsprotocol.go` implementando o início da
máquina de estado `graphql-transport-ws` (PROTOCOL.md já lido em `context.md`, não
reler do zero — reusar o entendimento documentado ali). `WSProtocolHandler(sch
*gql.Schema, subs map[string]*Subscription) func(conn execution.WSConn)`: ao conectar,
espera uma mensagem `{"type":"connection_init"}` dentro de um timeout (constante nova,
ex: `wsConnectionInitTimeout = 10 * time.Second` — v1 não expõe isso como config,
mesmo espírito de `sseHeartbeatInterval` em `sse.go`); se não chegar a tempo, fecha com
`4408` via `conn.CloseWithCode`. Se chegar, responde `{"type":"connection_ack"}`. Uma
SEGUNDA `connection_init` na mesma conexão fecha com `4429`. Uma mensagem com `type`
desconhecido/JSON inválido fecha com `4400`. `Ping`/`Pong`: ao receber
`{"type":"ping"}`, responde `{"type":"pong"}` (context.md's Deferred Ideas: v1 NÃO
inicia `Ping` por conta própria, só responde). Esta task cobre SÓ o handshake — o loop
de leitura pós-`connection_ack` só precisa reconhecer `ping`/`connection_init`
duplicado/tipo inválido; o tratamento de `subscribe`/`complete` (single-result e
streaming) fica pras próximas tasks (T6/T7), que ESTENDEM o mesmo loop de leitura, não
o reescrevem.
**Where**: `internal/graphql/wsprotocol.go` (novo)
**Depends on**: T4
**Reuses**: `execution.WSConn`/`CloseWithCode` (T2/T4), `wsTextMessage` (mover de `ws.go` pra `wsprotocol.go` já que `ws.go` é removido em T16 -- ou manter em `ws.go` até lá e só reusar a constante, decisão do Implementer, sem impacto observável)
**Requirement**: GQLRT-01

**Done when**:
- [ ] `WSProtocolHandler(sch, subs)` existe e implementa: timeout de `ConnectionInit` -> `4408`; `ConnectionAck` na primeira `ConnectionInit` válida; `ConnectionInit` duplicado -> `4429`; mensagem de `type` desconhecido/JSON inválido -> `4400`; `Ping` -> `Pong`
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestWSProtocolHandler_NoConnectionInitWithinTimeout_Closes4408`, `TestWSProtocolHandler_ConnectionInit_RespondsAck`, `TestWSProtocolHandler_DuplicateConnectionInit_Closes4429`, `TestWSProtocolHandler_UnknownMessageType_Closes4400`, `TestWSProtocolHandler_Ping_RespondsPong` (novos, `internal/graphql/wsprotocol_test.go`, usando uma `fakeWSConn` no mesmo estilo de `ws_test.go`'s -- reaproveitar/copiar o padrão, ajustado pra também gravar o `code`/`reason` passado a `CloseWithCode`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): wsprotocol handshake (ConnectionInit/Ack, timeouts, 4400/4429)`

---

### T6: `wsprotocol.go` — `Subscribe` de Query/Mutation (single-result operation)

**What**: Estender `WSProtocolHandler` (T5) pra tratar `{"id":"1","type":"subscribe",
"payload":{"query":...,"variables":...,"operationName":...}}` PÓS-`ConnectionAck`.
Descobrir se o campo requisitado é Query/Mutation OU Subscription (D4/context.md:
decidido em runtime pelo NOME do campo) checando se algum nome em `subs` aparece na
`payload.query` -- **não** fazer parsing AST pra isso na v1 (fora de escopo, YAGNI):
uma forma pragmática é tentar `graphql.Execute` primeiro (reusa T1) e, SE
`graphql-go` reportar "Cannot query field ... on type Subscription"/erro equivalente
de root-type mismatch, então tentar como Subscription -- OU (alternativa mais simples
e determinística, preferida se o Implementer confirmar que funciona): extrair o nome do
campo raiz da query via `graphql-go/graphql/language/parser` (já é dependência
transitiva do projeto, usado por `generate.go`'s `ast.Value`) e checar contra
`subs[name]` ANTES de decidir a rota -- exige pesquisa real de API (Context7), não
assumida. Se resolver pra Query/Mutation: chama `graphql.Execute(sch, ...)`, manda
EXATAMENTE 1 `{"id":..,"type":"next","payload":{"data":...,"errors":...}}` seguido de
`{"id":..,"type":"complete"}`. Qualquer operação (`subscribe`/`complete`/etc) recebida
ANTES do `ConnectionAck` fecha a conexão com `4401` (edge case do spec.md, cobrir aqui
já que é o primeiro tipo de operação implementado).
**Where**: `internal/graphql/wsprotocol.go`
**Depends on**: T5, T1
**Reuses**: `graphql.Execute` (T1)
**Requirement**: GQLRT-02

**Done when**:
- [ ] `Subscribe` cujo campo bate com Query/Mutation produz exatamente 1 `Next` + `Complete`, usando `graphql.Execute`
- [ ] Operação (`subscribe` ou qualquer outra) recebida antes do `ConnectionAck` fecha a conexão com `4401`
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestWSProtocolHandler_SubscribeQuery_RespondsNextThenComplete`, `TestWSProtocolHandler_SubscribeMutation_RespondsNextThenComplete`, `TestWSProtocolHandler_OperationBeforeAck_Closes4401` (novos, `internal/graphql/wsprotocol_test.go`, construindo um `*gql.Schema` real pequeno via `graphql.Build` com 1 Query registrada, mesmo padrão que `internal/app/graphql_test.go` já usa pra montar schemas de teste)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): wsprotocol Subscribe dispatches Query/Mutation as single-result operation`

---

### T7: `wsprotocol.go` — `Subscribe` de Subscription (streaming operation)

**What**: Estender `WSProtocolHandler` (T6) pra quando o campo requisitado bate com uma
`Subscription` registrada em `subs`: chamar `sub.HandlerFunc()(ctx, emit)` numa goroutine
própria (mesmo padrão de `ws.go`'s `WSHandler` atual -- `NewGraphqlContext` com um
`done chan struct{}`, `emit` serializando escrita via mutex), mandando UM
`{"id":..,"type":"next","payload":{"data":{<fieldName>: <value>}}}` por chamada de
`emit(value)` (D5/spec.md: reaproveita `Subscription.HandlerFunc()`/`gonest.Subscribe[T]`
sem tocar). Quando o client manda `{"id":..,"type":"complete"}` pra um `id` de
Subscription ativa, fecha o `done` channel daquele `id` especificamente (spec.md P1's
AC2: "não afeta outros ids ativos" -- prep direto pra T8's multiplexação, mesmo que
esta task ainda não precise rodar 2 Subscriptions concorrentes pra passar seus próprios
testes). Quando a conexão cai (`ReadMessage` retorna erro), toda Subscription ativa
naquela conexão para (fecha todo `done` ativo).
**Where**: `internal/graphql/wsprotocol.go`
**Depends on**: T6
**Reuses**: `Subscription.HandlerFunc()`, `NewGraphqlContext`, o padrão de `emit`/`done` já usado por `ws.go`'s `WSHandler` atual (copy+adapt pro formato de mensagem novo, não reescrever a lógica de concorrência do zero)
**Requirement**: GQLRT-03

**Done when**:
- [ ] `Subscribe` cujo campo bate com uma Subscription registrada produz um `Next` por `emit(value)` chamado pelo Handler
- [ ] `Complete` de um `id` ativo para SÓ aquele `id` (outro `id` ativo continua)
- [ ] Conexão caindo para toda Subscription ativa naquela conexão
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestWSProtocolHandler_SubscribeSubscription_EmitsNextPerEmittedValue`, `TestWSProtocolHandler_CompleteOneId_DoesNotAffectOtherActiveId`, `TestWSProtocolHandler_ConnectionDrops_StopsAllActiveSubscriptions` (novos, `internal/graphql/wsprotocol_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): wsprotocol Subscribe dispatches Subscription as streaming operation`

---

### T8: `wsprotocol.go` — multiplexação + `4409`

**What**: Provar (com testes NOVOS, não só confiar que T6/T7 "já funcionam assim" por
acaso) que N operações concorrentes (misturando Query/Mutation E Subscription) rodam
isoladas por `id` na MESMA conexão -- exige um registro interno (`map[string]<cancel
func ou done chan>` protegido por mutex, chaveado por `id`) que T6/T7 já devem ter
introduzido organicamente; esta task é o ponto de fechamento que garante e testa
explicitamente o requisito de multiplexação (GQLRT-04) e adiciona o fechamento que
faltava: um `Subscribe` chegando com um `id` JÁ EM USO (outra operação ativa com o mesmo
`id`, ainda não completada) fecha a conexão inteira com `4409` (spec.md P2's AC2).
**Where**: `internal/graphql/wsprotocol.go`
**Depends on**: T7
**Reuses**: O registro por-`id` já introduzido em T6/T7 (se T6/T7 não tiverem centralizado isso ainda, esta task é o ponto que força a centralização)
**Requirement**: GQLRT-04

**Done when**:
- [ ] Duas operações com `id`s diferentes (uma Query, uma Subscription) rodam concorrentemente na mesma conexão sem uma bloquear a outra
- [ ] `Subscribe` com `id` já ativo fecha a conexão com `4409`
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestWSProtocolHandler_TwoConcurrentOperationsDifferentIds_BothRunIndependently`, `TestWSProtocolHandler_SubscribeWithIdAlreadyActive_Closes4409` (novos, `internal/graphql/wsprotocol_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): wsprotocol multiplexing + 4409 on duplicate id`

---

### T9: `ssedistinct.go` — Query/Mutation via GraphQL-over-HTTP + `Accept: text/event-stream` [P]

**What**: Novo arquivo `internal/graphql/ssedistinct.go`. `SSEDistinctHandler(sch
*gql.Schema, subs map[string]*Subscription) func(req *execution.Request, res
*execution.Response)` -- serve `GET /graphql` (GraphQL over HTTP via query string:
`?query=...&variables=...&operationName=...`, spec.md's referência ao [GraphQL over
HTTP spec](https://github.com/graphql/graphql-over-http), CONFIRMAR o formato exato dos
3 query params via leitura real do spec antes de codar, não assumir). Se o campo
requisitado é Query/Mutation (mesma heurística de decisão de T6, reusar a MESMA função
de decisão -- extrair pra um helper compartilhado em vez de duplicar, ex:
`resolveOperationKind(query string, subs map[string]*Subscription) (isSubscription
bool, fieldName string, err error)` chamado tanto por `wsprotocol.go` quanto por
`ssedistinct.go`): chama `graphql.Execute` (T1), escreve exatamente 1 evento
`data: {"data":...,"errors":...}\n\n` seguido de `data: {}\n\n` como `complete`
(CONFIRMAR o formato exato do evento `complete` do `graphql-sse` PROTOCOL.md --
context.md já leu esse arquivo, não reler do zero, mas confirmar o shape exato do
evento antes de codar) via `res.Stream` (já existe, reusar de `sse.go` sem reescrever a
mecânica de `bufio.Writer`+mutex). Erros de VALIDAÇÃO (antes da execução -- ex: query
GraphQL sintaticamente inválida) chegam como evento `next` com o erro no `data`, NUNCA
como um `400` HTTP puro (spec.md AC3 -- `EventSource` nativo não expõe corpo de erro).
Esta task cobre só Query/Mutation; Subscription é T10.
**Where**: `internal/graphql/ssedistinct.go` (novo), possivelmente um helper novo
compartilhado (`resolveOperationKind` ou nome equivalente) extraído pra um arquivo
comum se T6 já tiver rodado antes -- se rodar em paralelo e T6 ainda não existir,
implementar a heurística aqui e deixar uma nota `// TODO(wsprotocol): reusar quando
T6 existir` (o Implementer desta task NÃO vê o estado de T6, pode estar rodando em
paralelo -- não é regressão duplicar por enquanto, ajuste de dedup fica pro Evaluator
sinalizar se notar as duas implementações divergindo)
**Depends on**: T1
**Reuses**: `graphql.Execute` (T1), `res.Stream` (já existe, `sse.go`'s mecânica de escrita)
**Requirement**: GQLRT-05

**Done when**:
- [ ] `GET /graphql` com `Accept: text/event-stream` e uma query/mutation válida responde 1 evento `next` + `complete`
- [ ] Erro de validação (antes da execução) chega como evento `next`, nunca `400` HTTP puro
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit/integration — `TestSSEDistinctHandler_ValidQuery_RespondsNextThenComplete`, `TestSSEDistinctHandler_InvalidQuery_RespondsNextWithErrorNot400` (novos, `internal/graphql/ssedistinct_test.go`, mesmo padrão de `fakeSSEResponder`/`io.Pipe` que `sse_test.go` já usa -- reusar, não reinventar)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): ssedistinct handles Query/Mutation via GraphQL-over-HTTP SSE`

---

### T10: `ssedistinct.go` — Subscription via SSE Distinct connections

**What**: Estender `SSEDistinctHandler` (T9) pro caso em que o campo requisitado bate
com uma Subscription registrada: mantém a conexão aberta, um evento `next` por
`emit(value)` (mesmo padrão de heartbeat/disconnect-detection que `sse.go`'s
`SSEHandler` atual já implementa -- copy+adapt pro formato de evento
`graphql-sse`/`{"data":{<fieldName>:<value>}}` em vez do formato ad-hoc atual, não
reescrever a mecânica de ticker/mutex/goroutine do zero).
**Where**: `internal/graphql/ssedistinct.go`
**Depends on**: T9
**Reuses**: `Subscription.HandlerFunc()`, `NewGraphqlContext`, o padrão de heartbeat/emit/disconnect de `sse.go`'s `SSEHandler`
**Requirement**: GQLRT-05

**Done when**:
- [ ] Operação Subscription mantém a conexão aberta, um evento `next` por `emit(value)`
- [ ] Cliente desconectando (write falha, detectado por heartbeat ou por um `emit`) encerra a goroutine do Handler sem vazar
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit/integration — `TestSSEDistinctHandler_Subscription_EmitsNextPerEmittedValue`, `TestSSEDistinctHandler_ClientDisconnects_HandlerGoroutineEnds` (novos, `internal/graphql/ssedistinct_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): ssedistinct handles Subscription via SSE Distinct connections`

---

### T11: `reservation.go` — registro de reservas (token → conexão SSE) [P]

**What**: Novo arquivo `internal/graphql/reservation.go`. Um registro thread-safe
(`map[string]*reservation` protegido por mutex, ou `sync.Map`) de token → estado da
reserva (D7/context.md: "token → conexão SSE correspondente"). `NewReservationRegistry()
*ReservationRegistry`. `(*ReservationRegistry) Reserve() (token string)` -- gera um
token único (`github.com/google/uuid`, já dependência do projeto, reusar em vez de
inventar geração própria) e registra uma reserva vazia (sem conexão SSE ainda
associada -- só o `PUT` aconteceu, o `GET` correspondente pode nunca chegar, spec.md's
Edge Case: "comportamento de expiração/limpeza fica em aberto pra Design" -- v1 desta
task NÃO implementa expiração/TTL, só o registro básico, YAGNI documentado igual ao
spec). `(*ReservationRegistry) Attach(token string, write func(frame string) error)
(ok bool)` -- associa a função de escrita da conexão SSE (aberta via `GET`, T12) a um
token JÁ reservado; `ok=false` se o token não existe. `(*ReservationRegistry)
Route(token, operationId string) (write func(frame string) error, ok bool)` -- usado
por T13/T14 pra rotear uma operação pro `write` certo. `(*ReservationRegistry)
Release(token string)` -- remove a reserva (chamado quando a conexão SSE do `GET`
cai).
**Where**: `internal/graphql/reservation.go` (novo)
**Depends on**: None
**Reuses**: `github.com/google/uuid` (já em `go.mod`, usado em outro lugar do projeto -- confirmar via grep, não assumir se não achar)
**Requirement**: GQLRT-06

**Done when**:
- [ ] `NewReservationRegistry`/`Reserve`/`Attach`/`Route`/`Release` existem, thread-safe (testado com `-race`)
- [ ] `Reserve()` retorna um token único a cada chamada
- [ ] `Route` retorna `ok=false` pra um token/estado inexistente ou não-anexado ainda
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestReservationRegistry_Reserve_ReturnsUniqueToken`, `TestReservationRegistry_AttachThenRoute_ReturnsWriteFunc`, `TestReservationRegistry_RouteBeforeAttach_ReturnsNotOk`, `TestReservationRegistry_ConcurrentReserveAttachRoute_NoRace` (novos, `internal/graphql/reservation_test.go`, o último rodando N goroutines concorrentes sob `-race`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): reservation registry for SSE Single connection mode`

---

### T12: `ssesingle.go` — `PUT` (reserva) + `GET` (conexão SSE única)

**What**: Novo arquivo `internal/graphql/ssesingle.go`. `SSESingleReserveHandler(reg
*ReservationRegistry) func(req *execution.Request, res *execution.Response)` -- serve
`PUT /graphql`: chama `reg.Reserve()`, responde `201` com o token no corpo (formato a
decidir pelo Implementer seguindo a convenção JSON já usada em outras respostas do
pacote, ex: `{"token": "..."}`) -- CONFIRMAR no PROTOCOL.md do `graphql-sse`
(já lido em `context.md`) se o formato de resposta do `PUT` é especificado com mais
rigor (header específico, corpo específico) antes de inventar o shape.
`SSESingleConnectHandler(reg *ReservationRegistry) func(req *execution.Request, res
*execution.Response)` -- serve `GET /graphql` com o token (header
`X-GraphQL-Event-Stream-Token` ou query `token`, spec.md AC2): abre a conexão SSE única
via `res.Stream` (mesma mecânica de `bufio.Writer`+mutex que `sse.go`/T9 já usam),
chama `reg.Attach(token, write)`, mantém a conexão aberta até o client desconectar
(nesse ponto chama `reg.Release(token)`). Token ausente ou não reservado -> `404`
(a decidir se HTTP puro é aceitável aqui, já que este NÃO é o caminho de erro de uma
OPERAÇÃO GraphQL em si -- é a conexão de transporte não encontrando sua reserva, mais
parecido com um erro de infraestrutura do que um erro de execução GraphQL; documentar a
decisão tomada no código).
**Where**: `internal/graphql/ssesingle.go` (novo)
**Depends on**: T11
**Reuses**: `res.Stream`, o padrão de `bufio.Writer`+mutex de `sse.go`, `ReservationRegistry` (T11)
**Requirement**: GQLRT-06

**Done when**:
- [ ] `PUT /graphql` responde `201` com um token de reserva
- [ ] `GET /graphql` com o token abre e mantém a conexão SSE associada àquele token via `reg.Attach`
- [ ] `GET /graphql` com token ausente/inexistente responde erro claro (não trava, não gera goroutine vazando)
- [ ] Conexão SSE caindo chama `reg.Release`
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit/integration — `TestSSESingleReserveHandler_Put_Responds201WithToken`, `TestSSESingleConnectHandler_ValidToken_AttachesConnection`, `TestSSESingleConnectHandler_UnknownToken_RespondsError`, `TestSSESingleConnectHandler_ClientDisconnects_ReleasesReservation` (novos, `internal/graphql/ssesingle_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): ssesingle PUT (reservation) + GET (single SSE connection)`

---

### T13: `ssesingle.go` — `POST` (executa operação, roteia resultado pro token)

**What**: Estender `ssesingle.go` (T12) com `SSESingleOperationHandler(sch *gql.Schema,
subs map[string]*Subscription, reg *ReservationRegistry) func(req *execution.Request,
res *execution.Response)` -- serve `POST /graphql` QUANDO a requisição carrega um token
+ `extensions.operationId` no corpo (spec.md AC3 -- distinguir esse `POST` do `POST
/graphql` JSON simples original é responsabilidade de T15's roteamento por
método+corpo, esta task só implementa o handler assumindo que já foi roteado
corretamente pra cá). Decodifica `{query, variables, operationName, extensions:
{operationId}}`, resolve token, usa a MESMA heurística de decisão de T6/T9
(Query/Mutation vs Subscription) -- reusar `resolveOperationKind` se já existir
(T6/T9), senão implementar aqui como as duas tasks anteriores já preveem. Se
Query/Mutation: `graphql.Execute` (T1), roteia o resultado via `reg.Route(token,
operationId)` como `next`+`complete` (`data` carregando `{id: operationId, payload:
{data, errors}}`, spec.md AC3). Se Subscription: inicia `sub.HandlerFunc()` numa
goroutine, cada `emit(value)` roteado como `next` pelo mesmo `reg.Route`, guardando uma
forma de CANCELAR essa Subscription especificamente depois (registro
`operationId -> done chan`, consumido por T14's `DELETE`). Responde `202` imediatamente
(antes do resultado real chegar pela conexão SSE).
**Where**: `internal/graphql/ssesingle.go`
**Depends on**: T12, T1
**Reuses**: `graphql.Execute` (T1), `ReservationRegistry.Route` (T11/T12), `Subscription.HandlerFunc()`
**Requirement**: GQLRT-06

**Done when**:
- [ ] `POST /graphql` com token+`operationId` executa a operação e responde `202` imediatamente
- [ ] Resultado (Query/Mutation: 1 vez; Subscription: N vezes) chega pela conexão SSE já aberta daquele token, como `next`/`complete` carregando `{id, payload}`
- [ ] Uma Subscription iniciada por este handler fica rastreável por `operationId` pra T14 conseguir cancelá-la
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit/integration — `TestSSESingleOperationHandler_QueryOperation_RoutesNextCompleteToToken`, `TestSSESingleOperationHandler_SubscriptionOperation_RoutesMultipleNextToToken`, `TestSSESingleOperationHandler_UnknownToken_RespondsError` (novos, `internal/graphql/ssesingle_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): ssesingle POST executes operation, routes result to reserved token`

---

### T14: `ssesingle.go` — `DELETE` (cancela operação streaming)

**What**: Estender `ssesingle.go` (T13) com `SSESingleCancelHandler(reg
*ReservationRegistry) func(req *execution.Request, res *execution.Response)` -- serve
`DELETE /graphql?operationId=X` com o token: encerra a Subscription ativa daquele
`operationId` especificamente (via o registro `operationId -> done chan` que T13
introduziu), SEM afetar outras Subscriptions ativas no MESMO token (spec.md AC4).
`operationId` inexistente/já completada -> resposta de erro clara (não pânico, não
travamento).
**Where**: `internal/graphql/ssesingle.go`
**Depends on**: T13
**Reuses**: O registro `operationId -> done chan` introduzido por T13
**Requirement**: GQLRT-06

**Done when**:
- [ ] `DELETE /graphql?operationId=X` com token válido encerra a Subscription daquele `operationId`
- [ ] Outra Subscription ativa no MESMO token, `operationId` diferente, continua rodando após o `DELETE`
- [ ] `operationId` inexistente responde erro claro, não pânico
- [ ] `go test ./internal/graphql/... -race` passa

**Tests**: unit — `TestSSESingleCancelHandler_ActiveOperationId_StopsThatSubscriptionOnly`, `TestSSESingleCancelHandler_UnknownOperationId_RespondsError` (novos, `internal/graphql/ssesingle_test.go`)
**Gate**: quick (`go test ./internal/graphql/... -race`)
**Commit**: `feat(graphql): ssesingle DELETE cancels one streaming operation by id`

---

### T15: `internal/app/graphql.go`'s `registerGraphql` — reescrito pra 4 `RegisterRoute` no mesmo path

**What**: Reescrever `registerGraphql` por inteiro. Em vez de 1 `RegisterRoute(POST)` +
1 `RegisterRoute(GET .../stream/:name)` + 1 `RegisterWebSocket(.../ws/:name)` (formato
antigo, removido), registrar EXATAMENTE 4 rotas, todas no MESMO `graphqlPath`
(`opts.GraphqlPath` ou `/graphql` default, inalterado):
- `RegisterRoute(HttpPost, graphqlPath, ...)` -- dispatcher que checa o CORPO
  decodificado: se carrega um token (`X-GraphQL-Event-Stream-Token`
  header ou `extensions.operationId` no corpo) delega pra
  `graphql.SSESingleOperationHandler` (T13); senão, é o `POST /graphql` JSON simples
  original (`graphqlHandler`, já refatorado em T1 pra usar `graphql.Execute`).
- `RegisterRoute(HttpPut, graphqlPath, graphql.SSESingleReserveHandler(reg))` (T12).
- `RegisterRoute(HttpGet, graphqlPath, ...)` -- dispatcher: se
  `req.IsWebSocketUpgrade()` (T3), delega pra `res.UpgradeWebSocket(graphql.
  WSProtocolHandler(sch, subsByName))` (T8); senão se a requisição carrega um token
  (header/query, spec.md AC2), delega pra `graphql.SSESingleConnectHandler(reg)` (T12);
  senão (GraphQL over HTTP com `Accept: text/event-stream`, ou fallback), delega pra
  `graphql.SSEDistinctHandler(sch, subsByName)` (T10) -- este é o edge case do spec.md
  ("roteamento do adapter Fiber SHALL despachar cada um corretamente por MÉTODO HTTP")
  RESOLVIDO por design: os 3 GETs convivem porque agora são um `RegisterRoute` só,
  com um dispatcher interno olhando headers, não 3 registros concorrentes de rota Fiber.
- `RegisterRoute(HttpDelete, graphqlPath, graphql.SSESingleCancelHandler(reg))` (T14).
`ReservationRegistry` (T11) é construído uma vez em `registerGraphql` (só se houver ao
menos 1 Subscription registrada, mesmo guard que hoje existe pra WS/SSE -- um app sem
NENHUM resolver GraphQL continua sem registrar rota nenhuma, comportamento preservado).
**Where**: `internal/app/graphql.go`
**Depends on**: T8, T10, T14
**Reuses**: Toda a lógica de coleta de queries/mutations/subscriptions já existente em `registerGraphql` (inalterada), `graphql.Execute`/`WSProtocolHandler`/`SSEDistinctHandler`/`SSESingleXxxHandler`/`ReservationRegistry`
**Requirement**: GQLRT-01..07

**Done when**:
- [ ] `registerGraphql` registra exatamente 4 `RegisterRoute` (POST/PUT/GET/DELETE), todas no mesmo `graphqlPath`, nenhum `RegisterWebSocket`
- [ ] `GET graphqlPath` com upgrade WS funciona (dispatcher escolhe `WSProtocolHandler`)
- [ ] `GET graphqlPath` sem upgrade, sem token, com `Accept: text/event-stream` funciona (dispatcher escolhe `SSEDistinctHandler`)
- [ ] `GET graphqlPath` com token funciona (dispatcher escolhe `SSESingleConnectHandler`)
- [ ] `POST graphqlPath` sem token continua funcionando como antes (JSON simples)
- [ ] `POST graphqlPath` com token+`operationId` funciona (`SSESingleOperationHandler`)
- [ ] `go build ./...` passa (repo inteiro volta a compilar -- resolve a quebra transitória aceita em T4)
- [ ] `go test ./... -race` passa

**Tests**: integration — `internal/app/graphql_test.go` ganha testes novos exercitando os 4 métodos no mesmo path via dispatch real (`app.Test`/dial real conforme o transporte exigir -- WS precisa de dial real, ver T4's teste como referência)
**Gate**: full (`go test ./... -race`)
**Commit**: `refactor(app)!: registerGraphql wires 4 real-protocol routes on the same /graphql path`

---

### T16: Remover `ws.go`/`sse.go` antigos + testes; reescrever `cross_transport_test.go`

**What**: Deletar `internal/graphql/ws.go`, `internal/graphql/sse.go`,
`internal/graphql/ws_test.go`, `internal/graphql/sse_test.go` por inteiro (os
transportes ad-hoc do Milestone 17, substituídos pelos protocolos reais das tasks
anteriores -- D1/context.md). Reescrever `internal/graphql/cross_transport_test.go`
(hoje testa SSE+WS ad-hoc recebendo o mesmo evento emitido) pro equivalente usando os
transportes NOVOS: um client WS (via `WSProtocolHandler`, protocolo real) e um client
SSE Distinct (via `SSEDistinctHandler`) subscritos na MESMA Subscription, ambos
recebendo o mesmo evento de um `Emitter.Emit`, desconectar um não afeta o outro --
mesmo Independent Test de spec.md's P2 story, só que contra a implementação real.
Confirmar (grep) que nenhum símbolo `graphql.WSHandler`/`graphql.SSEHandler` (as
funções antigas) sobra em lugar nenhum do repo.
**Where**: `internal/graphql/{ws,sse,ws_test,sse_test}.go` (deletados), `internal/graphql/cross_transport_test.go` (reescrito)
**Depends on**: T15
**Requirement**: GQLRT-07

**Done when**:
- [ ] `ws.go`/`sse.go`/`ws_test.go`/`sse_test.go` não existem mais
- [ ] `grep -r "WSHandler\|SSEHandler\b" internal/ gonest.go --include=*.go` retorna vazio (fora de `SSEDistinctHandler`/`SSESingleXxxHandler`/`WSProtocolHandler`, nomes novos que não colidem com o grep exato acima)
- [ ] `cross_transport_test.go` prova WS real + SSE Distinct real recebendo o mesmo evento, desconexão isolada
- [ ] `go test ./... -race` passa

**Tests**: integration — `cross_transport_test.go` reescrito (descrito acima)
**Gate**: full (`go test ./... -race`)
**Commit**: `refactor(graphql)!: remove ad-hoc ws.go/sse.go transports (replaced by real protocols)`

---

### T17: `.examples/blog-graphql` — demonstrar os 3 transportes reais

**What**: Atualizar `.examples/blog-graphql` (criado no Milestone 17/AD-037, hoje
demonstra Query/Mutation/Subscription via SSE ad-hoc `GET /graphql/stream/:name`) pra
demonstrar os 3 transportes desta feature: (1) `POST /graphql` JSON simples
(inalterado, já funciona); (2) WS via `graphql-transport-ws` -- adicionar uma seção no
README/comentário do exemplo mostrando como uma IDE real (ex: Apollo Sandbox/GraphiQL)
conecta; (3) SSE Distinct -- substituir o uso antigo de `curl -N .../stream/onXxx` por
`curl -N` com `Accept: text/event-stream` contra `/graphql?query=...`; (4) SSE Single
connection -- adicionar um fluxo de exemplo (script ou README) demonstrando
`PUT`→`GET`(token)→`POST`(operationId)→`DELETE`. Rodar o exemplo de VERDADE (não só
`go build`) confirmando que os 3 transportes funcionam ponta a ponta -- mesma lição de
AD-037 (3 bugs reais só apareceram rodando de verdade, não só testes unitários).
**Where**: `.examples/blog-graphql/*`
**Depends on**: T16
**Requirement**: Success Criteria (spec.md)

**Done when**:
- [ ] `.examples/blog-graphql` builda (`go build ./...` dentro do módulo do exemplo)
- [ ] WS real (dial de verdade, não só unitário) completa handshake + Query + Subscription contra o exemplo rodando
- [ ] SSE Distinct funciona via `curl -N` com `Accept: text/event-stream` contra o exemplo rodando
- [ ] SSE Single connection funciona via um client de teste/script que implementa reserva+token+multiplexação contra o exemplo rodando
- [ ] README do exemplo (ou comentário equivalente) atualizado, sem menção aos endpoints ad-hoc antigos (`/graphql/stream/:name`, `/graphql/ws/:name`)

**Tests**: manual/scripted (rodar o exemplo de verdade, mesma convenção de AD-037/AD-036) -- não é `go test`, é verificação end-to-end real documentada no relatório do Implementer
**Gate**: manual (`go run` do exemplo + `curl`/dial real contra ele, evidência colada no relatório, não assumida)
**Commit**: `docs(examples): blog-graphql demonstrates graphql-transport-ws and graphql-sse (both modes)`

---

### T18: Gate final

**What**: Rodar a suite completa, confirmar zero símbolo legado (`RegisterWebSocket`,
`WSHandler`/`SSEHandler` antigos, endpoints `/graphql/stream/:name`/`/graphql/ws/:name`),
confirmar `.examples/*` inteiro builda (não só `blog-graphql`), atualizar
`STATE.md`/`ROADMAP.md` com o AD final desta feature (decisões tomadas DURANTE a
execução, se houver SPEC_DEVIATION -- mesmo padrão de AD-036/AD-037 pra
`graphql-support`), atualizar `spec.md`'s traceability table pra "Verified".
**Where**: raiz, `.specs/project/{STATE,ROADMAP}.md`, `.specs/features/graphql-realtime-protocols/spec.md`
**Depends on**: T17

**Done when**:
- [ ] `go test ./... -race` passa
- [ ] `go build ./...` passa (repo raiz + todo `.examples/*`)
- [ ] `grep -r "RegisterWebSocket\|graphql/stream/:name\|graphql/ws/:name" . --include=*.go` retorna vazio (fora de `.specs`/`STATE.md`, registro histórico)
- [ ] `STATE.md` tem novo AD documentando a execução (+ SPEC_DEVIATIONs, se houver)
- [ ] `ROADMAP.md`'s Milestone 18 → COMPLETE
- [ ] `spec.md`'s traceability table → todo GQLRT-0x → Verified

**Tests**: integration (suite completa)
**Gate**: full (`go test ./... -race`)
**Commit**: `chore: finalize graphql-realtime-protocols feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Execute helper | 1 função + 1 refactor de call site | ✅ |
| T2: WSConn move + CloseWithCode | 1 tipo movido + 1 método novo | ✅ |
| T3: IsWebSocketUpgrade/UpgradeWebSocket | 2 métodos de interface + ~12 stubs mecânicos | ✅ (grande em contagem de arquivo, mecânico) |
| T4: Fiber real impl + remove RegisterWebSocket | 1 arquivo, mudanças relacionadas | ✅ |
| T5: WS handshake | 1 arquivo novo, escopo fechado (handshake só) | ✅ |
| T6: WS Subscribe single-result | extensão do mesmo arquivo, 1 capability | ✅ |
| T7: WS Subscribe streaming | extensão do mesmo arquivo, 1 capability | ✅ |
| T8: WS multiplexação + 4409 | extensão + testes de fechamento | ✅ |
| T9: SSE Distinct Query/Mutation | 1 arquivo novo, escopo fechado | ✅ |
| T10: SSE Distinct Subscription | extensão do mesmo arquivo | ✅ |
| T11: reservation registry | 1 arquivo novo, tipo isolado | ✅ |
| T12: SSE Single PUT+GET | 1 arquivo novo, 2 handlers relacionados | ✅ |
| T13: SSE Single POST | extensão do mesmo arquivo | ✅ |
| T14: SSE Single DELETE | extensão do mesmo arquivo | ✅ |
| T15: registerGraphql rewrite | 1 arquivo, mudança central de wiring | ✅ |
| T16: remoção + cross_transport_test rewrite | mecânico + 1 teste reescrito | ✅ |
| T17: exemplo | escopo grande mas isolado (1 diretório) | ✅ |
| T18: gate final | verificação + docs | ✅ |
