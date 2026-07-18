# GraphQL Realtime Protocols Context

**Gathered:** 2026-07-18 (brainstorming em conversa, motivado por um bug real -- testar Subscription
via uma IDE GraphQL real, screenshot em `C:\dev\gonest-dev\gonest\image.png`, mostrou a IDE tentando
conectar via WebSocket direto em `/graphql`, esperando o protocolo padrão `graphql-transport-ws`, e
falhando com `{"errors":[{"message":"Unknown GraphQL error"}]}`)
**Spec:** `.specs/features/graphql-realtime-protocols/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Substituir os DOIS transportes ad-hoc de Subscription que o gonest construiu por conta própria
(Milestone 17/T9-T10 -- `GET /graphql/stream/:name` e `GET /graphql/ws/:name`, nenhum seguindo
protocolo padrão nenhum) pelos protocolos REAIS e amplamente adotados que a comunidade GraphQL já usa:

1. **`graphql-transport-ws`** (github.com/enisdenjo/graphql-ws) -- WebSocket, direto no `/graphql`
   já existente (mesma URL que `POST /graphql` usa para Query/Mutation).
2. **`graphql-sse`** (github.com/enisdenjo/graphql-sse) -- Server-Sent Events, TAMBÉM no `/graphql`
   existente, cobrindo os DOIS modos que o protocolo define (Distinct connections E Single
   connection -- decisão do usuário: cobertura completa, não só o mais simples).

Ambos os ad-hoc anteriores são REMOVIDOS por inteiro -- nenhum teve consumidor real (Milestone 17 foi
lançado na mesma sessão).

---

## Implementation Decisions

### Remover o endpoint WS ad-hoc por inteiro, não manter os dois

`GET /graphql/ws/:name` nunca foi integrado por ninguém ainda (feature nova, lançada nesta mesma
sessão) -- sem motivo para manter compatibilidade com um formato que nunca teve consumidor real.
Substituição direta: o WS de Subscription passa a viver em `/graphql` (mesmo endpoint de Query/
Mutation), falando o protocolo padrão.

### Só o subprotocolo moderno `graphql-transport-ws`, não o legado `graphql-ws`

Existem dois subprotocolos WS na história do ecossistema GraphQL:

1. **`graphql-transport-ws`** (github.com/enisdenjo/graphql-ws) -- o atual, mantido, usado por
   GraphiQL/Apollo Sandbox/urql modernos.
2. **`graphql-ws`** (github.com/apollographql/subscriptions-transport-ws) -- o legado, descontinuado
   pelo próprio autor original, ainda usado por ferramentas mais antigas.

Decisão: implementar só o moderno. Suportar os dois dobraria a complexidade (2 máquinas de estado de
protocolo distintas) sem benefício líquido -- IDEs modernas (o caso real que motivou esta feature) já
usam o moderno.

### Multiplexação suportada desde a v1

O protocolo `graphql-transport-ws` já é multiplexado por natureza -- cada operação (Query, Mutation,
OU Subscription) tem seu próprio `id` único dentro da MESMA conexão WS, e várias podem estar ativas
simultaneamente. NÃO suportar isso exigiria um caminho especial ("só 1 operação por vez", rejeitando
uma segunda `Subscribe` enquanto a primeira está ativa) -- mais trabalho de exceção, não menos. V1
já suporta N operações concorrentes por conexão, cada uma rastreada pelo seu próprio `id`.

### Query/Mutation TAMBÉM passam a ser executáveis via WS (não só Subscription)

O protocolo não distingue "isso é uma Subscription" de "isso é uma Query/Mutation" na mensagem
`Subscribe` em si -- ambos usam a mesma forma (`{id, type: "subscribe", payload: {query, variables,
operationName}}"`), a única diferença observável é quantas mensagens `Next` o servidor manda antes do
`Complete` (Subscription: N. Query/Mutation: exatamente 1, chamado de "single-result operation" no
PROTOCOL.md). Isso significa que o gonest precisa, ao receber um `Subscribe`, primeiro DESCOBRIR se a
operação é uma `subscription` (nome do campo bate com uma `Subscription` registrada) ou uma
`query`/`mutation` (nesse caso, delega pro MESMO `graphql.Build`'s `*gql.Schema`/`gql.Do` que
`POST /graphql` já usa, só que devolvendo o resultado como `Next`+`Complete` ao invés de um corpo HTTP
único).

### Pesquisa real feita (não assumida)

`PROTOCOL.md` de `github.com/enisdenjo/graphql-ws` (via `gh api repos/enisdenjo/graphql-ws/contents/
PROTOCOL.md`, conteúdo real lido nesta sessão, não resumo de terceiros) -- mensagens `ConnectionInit`/
`ConnectionAck`/`Ping`/`Pong`/`Subscribe`/`Next`/`Error`/`Complete`, cada uma com `type` (+ `id` quando
aplicável a uma operação específica, + `payload` conforme o tipo). Códigos de fechamento WS
específicos do protocolo: `4408` (sem `ConnectionInit` dentro do timeout), `4429` (`ConnectionInit`
duplicado), `4409` (`id` de `Subscribe` já em uso), `4401` (operação antes do `ConnectionAck`), `4400`
(mensagem inválida/tipo desconhecido).

---

### `graphql-sse`: os DOIS modos, cobertura completa (decisão do usuário)

O protocolo `graphql-sse` (`PROTOCOL.md` de `github.com/enisdenjo/graphql-sse`, lido nesta sessão via
`curl` direto no raw do GitHub -- conteúdo real, não resumo de terceiros) define dois modos
distintos, ambos a implementar:

1. **Distinct connections mode** -- uma conexão SSE por operação. O client faz uma requisição HTTP
   normal conforme o [GraphQL over HTTP spec](https://github.com/graphql/graphql-over-http), com
   `Content-Type`/`Accept: text/event-stream`; a resposta É a stream SSE (eventos `next`/`complete`,
   sem `id` -- só existe UMA operação por conexão, não precisa distinguir). Mais simples: sem
   reserva, sem token, sem endpoints extras.

2. **Single connection mode** -- pensado pra contornar o limite de conexões simultâneas do HTTP/1 nos
   browsers (6 por domínio, "Won't fix" no Chrome/Firefox). Fluxo: (a) client faz `PUT` pedindo uma
   "reserva", servidor responde `201` com um token; (b) client abre UMA conexão SSE carregando esse
   token (header `X-GraphQL-Event-Stream-Token` ou query param `token`); (c) cada operação subsequente
   é um `POST` separado (contendo o token + `extensions.operationId` no corpo da requisição GraphQL),
   respondido com `202` (Accepted) -- o RESULTADO de verdade chega pela conexão SSE já aberta, como
   eventos `next`/`complete` carregando `data: {id, payload}`; (d) `DELETE
   ?operationId=<id>` (+ token) encerra uma operação streaming (Subscription) antes dela terminar
   sozinha.

Como o usuário optou por cobertura completa dos dois modos, o gonest precisa manter um registro de
reservas ativas (token → conexão SSE correspondente) e rotear `POST`/`DELETE` de operação pro fluxo
`next`/`complete` da conexão SSE certa via esse token -- mecanismo novo, sem equivalente hoje
(SSE ad-hoc atual é 1 conexão = 1 Subscription, sem conceito de reserva/token/multiplexação).

## Specific References

- `image.png` (repo root, já removido do git -- era só um screenshot de debug) -- a IDE do usuário
  tentando WS em `/graphql` e recebendo `Unknown GraphQL error`, motivador direto desta feature.
- `.specs/features/graphql-support/` (Milestone 17) -- feature-mãe, esta é uma EXTENSÃO dela, mesmo
  `internal/graphql` package, mesmo `graphql.Build`/`*gql.Schema`.
- AD-036/AD-037 em STATE.md -- decisão de motor (`graphql-go/graphql`) e os 3 bugs reais achados via
  `.examples/blog-graphql`, contexto direto de por que "rodar de verdade" importa aqui também.
- `internal/adapter/fiber/fiber.go`'s `RegisterWebSocket` (já existe, Milestone 17 T10) -- mesma
  capability do `HttpAdapter`, reusada aqui; só a REGISTRAÇÃO muda (path `/graphql` em vez de
  `/graphql/ws/:name`, MAS a REGISTRAÇÃO de rota HTTP comum `POST /graphql` e a de WebSocket
  precisam coexistir no MESMO path -- a decidir em Design se isso é um problema real pro adapter
  Fiber ou não, já que `RegisterRoute`/`RegisterWebSocket` são chamadas separadas hoje).

## Deferred Ideas

- Subprotocolo legado `graphql-ws` (apollographql) -- fora de escopo, ver decisão acima.
- `Ping`/`Pong` keep-alive automático por parte do gonest (enviar `Ping` periódico não solicitado) --
  o protocolo permite mas não EXIGE isso do servidor; v1 só responde `Pong` a um `Ping` do client,
  não inicia por conta própria.
- Autenticação/payload de `ConnectionInit` -- v1 aceita qualquer `payload` (ou nenhum) sem validar,
  igual ao comportamento de auth de REST hoje (nenhum built-in, fica a cargo do dev via Guard
  equivalente -- que não existe pra WS ainda, gap reconhecido).
