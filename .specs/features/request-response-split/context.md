# Request/Response Split Context

**Gathered:** 2026-07-17 (via `superpowers:brainstorming`, conduzido só na conversa — usuário pediu explicitamente para NÃO gerar `docs/superpowers/specs/*.md`)
**Spec:** `.specs/features/request-response-split/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Substituir `*RestContext` único (`internal/execution.Context`) por dois
tipos concretos — `*Request` (leitura) e `*Response` (escrita) — em toda a
superfície pública (`Handler`, `Guard`, `Middleware`, `Interceptor`,
`Filter`), espelhando o padrão `(req, res)` de Express/NestJS.

---

## Motivação de Produto (não só técnica)

gonest mira devusers vindos do ecossistema Node — a maior parte das libs
Express-like da comunidade Node usa `(req, res)` como padrão, e NestJS
(criado em cima do Express) herdou essa convenção. gonest busca trazer esse
tipo de devuser pro Go trazendo performance/economia de recursos em produção,
mas SEM a dor de Go ser estranho pra quem vem de lá. Esse é o motivo de
fundo, não só resolver a colisão de nome `ctx.Json()` (leitura vs escrita).

---

## Implementation Decisions

### Abordagem escolhida: split real, não façade

Duas abordagens foram apresentadas:
- **A (escolhida)**: `Request`/`Response` são tipos de verdade, sem
  `Context` compartilhado por baixo. Handler/dispatch trabalham com os dois
  ponteiros diretamente.
- **B (descartada)**: `Context` único continuaria existindo internamente
  (mesmo wiring de `WithRoute`/`WithSources`), `Request`/`Response` seriam
  wrappers finos por cima do mesmo objeto — "split de fachada", isolamento
  só aparente.

Usuário escolheu A explicitamente, mesmo sabendo do custo de refactor maior
(seção "Fluxo de dados" do brainstorming), porque o motivo de fundo é
adoção/familiaridade — um split de fachada entrega a sintaxe mas não a
testabilidade/composição reais que a decisão promete.

### Escopo do split: TUDO migra

Perguntado se só o `Handler` final migraria (Guards/Interceptors/Middlewares
continuando com `ctx` único, já que majoritariamente só leem/decidem),
usuário respondeu que TODOS migram — consistência total na API pública é
mais importante que minimizar a superfície de mudança.

### `next` de Middleware/Interceptor

Duas opções: `func(req, res, next func(req, res))` (espelha Express de
verdade) vs `func(req, res, next func())` (mais raiz Go, já que req/res são
ponteiros compartilhados na mesma call chain). Usuário escolheu a primeira —
`next(req, res)` explícito, mesmo padrão em toda a cadeia.

### `Response` conhece `Request`

Perguntado se `Request`/`Response` deveriam ser totalmente independentes
(sem referência cruzada, espelhando Express de verdade) ou se `Response`
guardaria um ponteiro pro `Request` (útil pra logging/decisões baseadas em
dados do request). Usuário respondeu que não vê problema em `Response`
conhecer `Request` internamente — decisão tomada sem alternativa forte
descartada, só validação de que o acoplamento é aceitável.

### `Request.Body()` consolidado com `Raw()`/`Text()`

Usuário perguntou diretamente se `Request.RawBody()` (que sobrou solto da
feature `unified-parse-api`) não deveria virar `Request.Body().Raw()`,
seguindo o mesmo padrão de `.Json()`/`.Form(onFile)` já estabelecido — e
junto, adicionar `Request.Body().Text()`. Ambos aceitos: `BodySource` ganha
`Raw() []byte` e `Text() string`, os dois retornando valor direto (não
`Parseable` — bytes/texto crus não têm schema pra validar contra, diferente
de `Json()`/`Form()`).

### `Response.HTML` → `Response.Html`

Usuário apontou que `HTML` (all-caps) destoa de `Json`/`Text` (PascalCase
normal) — o projeto já usa essa convenção em outros lugares (`BodyJsonSchema`,
não `BodyJSONSchema`). Renomeado por consistência.

### `Response.Text` força Content-Type

Usuário perguntou diretamente: "a ideia do Html/Text/Json é forçar os
content types relacionados" — confirmado que `Text(s)` deve setar
`Content-Type: text/plain` explicitamente, IGUAL `Html()` força `text/html`
e `Json()` força `application/json`. Isso é uma MUDANÇA DE COMPORTAMENTO
real vs o `SendString` atual (que não seta Content-Type nenhum) — não é só
rename. `SendString` deixa de existir como método público.

### `gonest.Value[T]` descartado pra Get/Set de Request/Response

Usuário questionou se valeria usar `gonest.Value[T]` (tipo genérico já
existente, `internal/value`) nos pares com prefixo Get/Set do
Request/Response, pra sinalizar por tipo quando algo é bidirecional.
Análise conjunta revelou dois problemas:
1. `GetHeader`/`SetHeaderValue` NÃO são um par real — `GetHeader` lê o
   header do REQUEST, `SetHeaderValue` escreve o header da RESPONSE, dados
   diferentes que só têm nome parecido. Só `GetStatus`/`SetStatus` é um par
   de verdade (os dois mexem no mesmo dado: status code da resposta).
2. O `Value[T]` existente tem semântica de DIRTY-TRACKING (pensado pra
   decodificar bodies PATCH-style) — `Get()` retorna o valor armazenado
   independente de estado real vindo de baixo. `Response.StatusCode()`
   precisa refletir o estado real do fasthttp (que já default pra 200 antes
   de qualquer `Set` explícito) — usar `Value[T]` quebraria esse default.

Usuário optou por NÃO usar `Value[T]` aqui, manter `Status(code)`/
`StatusCode()` simples — ver spec.md's Out of Scope.

---

## Specific References

- `.specs/features/unified-parse-api/` — feature anterior que introduziu
  `Parseable`/`BodySource`/`ctx.Params()`/`ctx.Query()`/`ctx.Headers()`/
  `ctx.Body()`. Esta feature reusa TODOS esses tipos inalterados — só migra
  o objeto que os expõe (`Context` → `Request`).
- AD-025 (STATE.md) — decisões arquiteturais de `Parseable`/`BodySource`,
  ainda válidas, só o "dono" (Context → Request) muda.
- 17 arquivos internos hoje dependem de `func(ctx *execution.Context)`
  (confirmado via grep durante o brainstorming): `internal/app`,
  `internal/adapter/fiber`, `internal/route`, `internal/guard`,
  `internal/interceptor`, `internal/filter`, `internal/middleware`,
  `internal/openapi`, `internal/controller`, e seus respectivos `_test.go`.

## Deferred Ideas

- `gonest.Value[T]` genérico pra outros pares Get/Set futuros, SE algum
  candidato real (mesmo dado, bidirecional, sem depender de estado externo
  tipo fasthttp) aparecer depois — não descartado como padrão geral, só não
  se aplica a nada existente hoje.
