# blog-graphql

The GraphQL-focused gonest example: one `GraphqlResolver` (`resolver.go`)
exposing `Query`/`Mutation`/`Subscription` over an in-memory `Post` store
(`service.go`), reusing the exact same `Schema`/`MustParse[T]`/`MustInject`
REST already uses -- see `.examples/blog-api` for the REST-heavy example
this one deliberately stays separate from.

## Schema

- `posts` (Query) -- returns every stored `Post`
- `post(id: Int!)` (Query) -- returns one `Post`, 404s (via
  `gonest.NewNotFoundException`) when `id` is not found
- `createPost(title: String!, body: String!)` (Mutation) -- stores a new
  `Post` and emits a `PostCreatedEvent` (`gonest.Emitter`)
- `postCreated` (Subscription) -- streams every `Post` created after the
  client subscribes, via `gonest.Subscribe[PostCreatedEvent]`

## Run

```
cd .examples/blog-graphql && go run .
```

Listens on `:3002`.

## The 3 real-protocol transports

Every transport below is served on the exact same path, `/graphql`,
dispatched purely by HTTP method + headers (see
`internal/app/graphql.go`'s `registerGraphql`) -- there is no separate
path per transport; every Subscription goes through one of the 2
streaming protocols below (WebSocket or SSE).

### 1. Plain GraphQL-over-HTTP -- `POST /graphql`

Unchanged: a single JSON `{query, variables, operationName}` body, answered
with a single `{data, errors}` JSON response. Used above for Query/Mutation
one-shot calls, and also the transport a `Subscription` operation can never
be dispatched through (it needs a streaming transport -- one of the 2
below).

### 2. WebSocket -- `graphql-transport-ws` (`GET /graphql`, `Upgrade: websocket`)

The [graphql-ws](https://github.com/enisdenjo/graphql-ws) protocol
(`Sec-WebSocket-Protocol: graphql-transport-ws`), implemented by
`internal/graphql.WSProtocolHandler`. Real GraphQL IDEs (Apollo Sandbox,
GraphiQL, Altair) speak this natively when you point them at
`ws://localhost:3002/graphql` -- no separate config needed beyond the
default WS endpoint. A minimal hand-rolled client, message by message:

1. Dial `ws://localhost:3002/graphql` (subprotocol `graphql-transport-ws`).
2. Send `{"type":"connection_init"}` -- server replies
   `{"type":"connection_ack"}` (a 3s timeout closes the connection with
   code `4408` if this never arrives).
3. Send a `subscribe` message per operation, each with its own `id`:
   ```json
   {"id":"1","type":"subscribe","payload":{"query":"{ posts { id title body } }"}}
   ```
   A Query/Mutation answers with one `next` (the `{data,errors}` result)
   followed immediately by one `complete`. A `Subscription` (e.g.
   `subscription { postCreated { id title body } }`) instead keeps sending
   `next` messages, one per emitted event, until the client sends
   `{"id":"<id>","type":"complete"}` or the connection drops.

Verified end-to-end with a real Go client (`gorilla/websocket`, dialed
against the example running on `:3002`) -- captured output:

```
HTTP status: 101 Switching Protocols
recv: map[type:connection_ack]
recv (query next): map[id:1 payload:map[data:map[posts:[]] errors:<nil>] type:next]
recv (query complete): map[id:1 type:complete]
recv (subscription next): map[id:2 payload:map[data:map[postCreated:map[body:via curl id:1 title:WS Trigger]]] type:next]
WS test finished successfully
```

(The `postCreated` event above arrived after a `createPost` Mutation was
fired via `curl` from a second terminal while the `id:2` subscription was
still open -- proving the streaming path, not just the handshake.)

### 3. Server-Sent Events -- `graphql-sse`, Distinct connections mode (`GET /graphql`, `Accept: text/event-stream`)

The [graphql-sse](https://github.com/enisdenjo/graphql-sse) protocol's
"Distinct connections" mode, implemented by
`internal/graphql.SSEDistinctHandler`: a normal `GET /graphql` whose
query/variables/operationName travel as query-string parameters, and whose
response body IS the SSE stream (`event: next` / `event: complete`
frames). One connection per operation.

```
# Query -- one `next` frame then `complete`, connection ends
curl -N --get localhost:3002/graphql \
  --data-urlencode 'query={ posts { id title body } }' \
  -H 'Accept: text/event-stream'
```

Real output:

```
event: next
data: {"data":{"posts":[{"body":"via curl","id":1,"title":"WS Trigger"}]},"errors":null}

event: complete
data:
```

```
# Subscription -- connection stays open, one `next` frame per event
curl -N --get localhost:3002/graphql \
  --data-urlencode 'query=subscription { postCreated { id title body } }' \
  -H 'Accept: text/event-stream'
```

Real output (captured while a `createPost` Mutation was fired from a
second terminal):

```
event: next
data: {"data":{"postCreated":{"id":2,"title":"SSE Distinct Trigger","body":"via curl2"}}}
```

### 4. Server-Sent Events -- `graphql-sse`, Single connection mode (`PUT`/`GET`/`POST`/`DELETE /graphql`)

The same [graphql-sse](https://github.com/enisdenjo/graphql-sse) protocol's
"Single connection mode": one long-lived SSE connection multiplexes every
operation (Query/Mutation/Subscription) for a client, each tagged by its
own `operationId`. 4 steps:

```
# 1. PUT -- reserve a token
curl -s -X PUT localhost:3002/graphql -i
# -> 201 Created, {"token":"<token>"}

# 2. GET -- open the one SSE connection for that token (run in the
#    background / a separate terminal, it stays open)
curl -N -H "X-GraphQL-Event-Stream-Token: <token>" localhost:3002/graphql

# 3. POST -- start an operation over that reservation (repeatable, one
#    call per operationId; the result streams over step 2's connection,
#    NOT in this response, which is always a bare 202)
curl -X POST localhost:3002/graphql \
  -H "X-GraphQL-Event-Stream-Token: <token>" \
  -d '{"query":"{ posts { id title } }","extensions":{"operationId":"op1"}}'

curl -X POST localhost:3002/graphql \
  -H "X-GraphQL-Event-Stream-Token: <token>" \
  -d '{"query":"subscription { postCreated { id title body } }","extensions":{"operationId":"op2"}}'

# 4. DELETE -- cancel one still-streaming operation (Subscriptions only;
#    a Query/Mutation's `next`+`complete` pair already ends on its own)
curl -X DELETE "localhost:3002/graphql?operationId=op2" \
  -H "X-GraphQL-Event-Stream-Token: <token>"
```

Real output from a full PUT -> GET -> POST(x2) -> createPost Mutation ->
DELETE run:

```
--- PUT ---
HTTP/1.1 201 Created
{"token":"19dac5d4-97f2-4f25-812c-e5184a3cb1b8"}

--- POST (op1, Query) ---
HTTP/1.1 202 Accepted
{}

--- POST (op2, Subscription) ---
HTTP/1.1 202 Accepted
{}

--- createPost Mutation fired from a 3rd terminal ---
{"data":{"createPost":{"id":3,"title":"SSE Single Trigger"}}}

--- DELETE (cancel op2) ---
HTTP/1.1 200 OK
{}

--- what the GET connection (step 2) actually streamed ---
event: next
data: {"id":"op1","payload":{"data":{"posts":[{"id":1,"title":"WS Trigger"},{"id":2,"title":"SSE Distinct Trigger"}]},"errors":null}}

event: complete
data: {"id":"op1"}

event: next
data: {"id":"op2","payload":{"data":{"postCreated":{"id":3,"title":"SSE Single Trigger","body":"via curl3"}}}}
```

(`op1`, a Query, answers with its own `next`+`complete` pair and needs no
DELETE; `op2`, a Subscription, keeps streaming `next` frames until the
DELETE above cancels it -- note the connection never sees a `complete` for
`op2` in this run, since the DELETE terminates it from the outside rather
than the handler returning on its own.)
