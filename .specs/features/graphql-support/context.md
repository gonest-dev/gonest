# GraphQL Support Context

**Gathered:** 2026-07-17 (brainstorming em conversa, evoluindo o rascunho `INSIGHT-GRAPHQL.md`)
**Spec:** `.specs/features/graphql-support/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Nova ponta de exposição (`Resolver`) análoga a `Controller`, cobrindo
Query/Mutation/Subscription GraphQL, reaproveitando 100% de
`Schema`/`PropertyBuilder`/`Parse[T]`/`MustParse[T]`/`MustInject`/
`Emitter` já existentes. Sem mudança em REST/OpenAPI.

---

## Implementation Decisions

### Handler signature: 1 parâmetro, retorno = data (não Response separado)

Diferente de REST (`Handler(req, res)`, motivado pelo público-alvo
Express/NestJS e por `Response` ter status/headers reais pra justificar
um write-side rico), GraphQL resolvers no NestJS literalmente fazem
`return valor` e o framework trata como o `data` -- não existe um
`res`/write-side separado porque não há status code nem headers na
semântica de um resolver individual. Usuário confirmou essa observação
diretamente ("inclusive no nestjs ele apenas pega o retorno e já trata
como data"), o que motivou a assinatura final `func(ctx *GraphqlContext) any`.

Nome do método também foi ajustado durante a conversa: `query.Resolve(fn)`
→ `query.Handler(fn)`, por consistência com `Route.Handler`/`Guard.
Handler`/`Middleware.Handler`/etc -- usuário já tinha feito essa mudança
sozinho no arquivo antes de eu revisar, só precisei corrigir comentários
órfãos da versão anterior (que ainda citavam `res.Data`/`GraphRequest`).

### Subscription não reusa `Handler(ctx) any`

Query/Mutation são request-response (1 valor, acaba). Subscription é um
STREAM -- o resolver fica vivo emitindo N valores enquanto o client segue
conectado. Por isso ganhou assinatura própria: `Handler(func(ctx
*GraphqlContext, emit func(any)))`, onde `emit` publica um valor por vez e
o Handler só retorna quando a subscription termina (`<-ctx.Done()`).

### Subscription reaproveita `gonest.Emitter` (não um callback ad-hoc)

Primeira versão do sketch usava um `orderService.OnCreated(callback)`
fictício -- usuário apontou a inconsistência: o app já tem UM barramento
de eventos (`gonest.Emitter`, Event Emitter feature, já implementado,
usado hoje via `Emit`/`MustOn`), e a conclusão do próprio doc promete
"Uma Única Fonte de Verdade". Reescrito para `emitter.Subscribe[T
](ctx.Done())`, um método NOVO complementar a `Emit`/`MustOn`:

- `MustOn[T](listener, handler)` -- estático, registrado no bootstrap via
  `module.Listeners`, dura a vida toda do app (caso "fire-and-forget,
  handler fixo").
- `Subscribe[T](done <-chan struct{}) <-chan T` -- dinâmico, vivo só
  enquanto a conexão GraphQL específica durar, cancelado automaticamente
  quando `done` fecha (mesmo padrão de `context.Context` do Go stdlib).

Detalhe técnico confirmado em conversa: `range` sobre o canal devolvido só
sai sozinho quando o canal é FECHADO (não quando `done` dispara -- são
coisas diferentes). A responsabilidade de fechar o canal ao observar
`done` é da IMPLEMENTAÇÃO de `Subscribe` (precisa rodar uma goroutine
interna que aguarda `done` e chama `close(ch)`), não é automático do Go --
sem isso, a goroutine da subscription vazaria.

### Nomenclatura: `Value` (schema-value-support) e conexão com `GraphqlScalar`

Durante a mesma sessão de brainstorming, uma reflexão PARALELA sobre
suportar `gonest.NewSchema` para valores primitivos isolados (sem struct)
virou sua própria feature (`schema-value-support`, Milestone 15) -- essa
feature é mencionada aqui só como MOTIVAÇÃO futura (`Value` nomeado
reusável entre múltiplos `GraphqlScalar(name)`), não implementada como
parte desta spec.

### Custom Scalars: dois níveis

1. **Formato nativo** (`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/
   `Password`/`Byte`/`Binary`/`DateTime`/`Date`) -- já tem `format`
   OpenAPI conhecido, o gerador de SDL sabe automaticamente que nome de
   scalar usar (`Email`→`scalar Email`, etc).
2. **Custom(fn) verdadeiramente customizado** (ex: `primitive.ObjectID`
   do MongoDB) -- sem `format` OpenAPI equivalente, `Custom(fn)` sozinho
   não diz ao gerador qual nome de scalar usar (é genérico por natureza).
   Resolvido com `.GraphqlScalar(name)` como modificador de `Custom(fn)`,
   só relevante quando o Schema é consumido por um Resolver -- REST/
   OpenAPI ignoram esse valor. Múltiplos campos com o MESMO `GraphqlScalar
   (name)` deduplicam pra uma única declaração `scalar X`, mesmo padrão
   de dedup por identidade que `internal/openapi.Generate` já faz pra
   `$ref`/`components.schemas`.

Chamar `Custom(fn)` sem `.GraphqlScalar(name)` num schema usado por
Resolver é um erro de CONFIGURAÇÃO (build-time da geração de SDL), mesma
categoria que `resolveSchema`'s panic de mismatch -- erro de programador,
não falha de request.

### Motor GraphQL: pesquisa real, não assumida

Investigação via WebSearch + `gh issue view` (não fabricado):

- `99designs/gqlgen` -- schema-first (`.graphql` manual + `go generate`).
  Subscriptions maduras (WebSocket via `graphql-ws`/`graphql-transport-ws`,
  SSE como alternativa via `graphql-sse`). Mas exige 2 camadas de geração
  se usado com o gonest (gonest gera `.graphql` a partir do `Schema`,
  gqlgen gera bindings Go a partir do `.graphql`) -- quebra a filosofia
  "tudo em runtime, sem etapa de build" que `Schema`/`PropertyBuilder` já
  seguem hoje.
- `graphql-go/graphql` -- code-first (schema montado via `graphql.
  NewSchema(...)` direto em Go, runtime). Repo confirmado ATIVO (`gh repo
  view`: push em 2026-06-23, 10k+ stars, não arquivado). Issue #49
  (`gh issue view 49 --repo graphql-go/graphql`) confirma: suporte a
  `subscription` foi resolvido em 2016 só na SINTAXE (parsear a operação,
  declarar `SubscriptionRoot`) -- NUNCA veio com motor de
  execução/streaming pronto (comentários subsequentes na própria issue,
  "Any working example?", confirmam que ninguém usou isso pronto pra
  produção sem construir a camada de execução por fora).

Decisão final: `graphql-go/graphql`. A limitação de subscription NÃO pesa
contra -- o gonest já ia construir sua PRÓPRIA camada de execução via
`Emitter.Subscribe` de qualquer forma (ver acima), então o que o `gqlgen`
daria de graça (motor de subscription pronto) não seria aproveitado do
mesmo jeito mesmo se disponível. `graphql-go/graphql` ganha a filosofia
code-first/runtime sem abrir mão de nada genuinamente útil.

**Consequência aceita explicitamente pelo usuário**: o gonest terá que
escrever sua própria solução de TRANSPORTE de Subscription cobrindo tanto
SSE quanto WebSocket -- nenhum dos dois vem pronto da lib escolhida. Os
protocolos de referência (`graphql-ws`, `graphql-transport-ws`, `graphql-
sse`) existem como especificação pública e servem de guia de
compatibilidade com clients existentes (Apollo Client, urql, etc), mesmo
implementando por conta própria.

---

## Specific References

- `INSIGHT-GRAPHQL.md` (repo root) -- o sketch vivo desta reflexão,
  atualizado ao longo de toda a conversa (Query/Mutation/Subscription,
  Custom Scalars, GraphqlScalar, decisão de lib). Fonte de verdade do
  "como fica o código", não duplicado neste context.md.
- `.specs/features/schema-value-support/` (Milestone 15) -- feature
  irmã, motivação futura pra `Value` nomeado reusável entre
  `GraphqlScalar(name)`s, fora de escopo aqui.
- AD-028/AD-030 em STATE.md -- padrão de 3 papéis de subagente
  (Planner/Implementer/Evaluator), a ser seguido quando esta feature
  chegar em Tasks/Execute.
- Event Emitter feature (Milestone 9, ROADMAP.md) -- `gonest.Emitter`/
  `MustOn`/`Emit` já implementados, `Subscribe[T]` é adição nova sobre
  essa base.
- `internal/openapi/generate.go` -- padrão de dedup por identidade de
  ponteiro (`$ref`/`components.schemas`) a espelhar para dedup de Custom
  Scalars por nome.

## Deferred Ideas

- Erro dentro de Subscription (canal de erro próprio tipo `emitError(err)`,
  fechamento gracioso só daquele client) -- gap reconhecido, não
  resolvido nesta spec (ver Edge Cases no spec.md).
- Federation/schema stitching, DataLoader/batching -- fora de escopo
  desta primeira versão.
- Registro de scalar em algo equivalente a `components.schemas` do
  OpenAPI -- não pensado.
