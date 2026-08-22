# Redirect (Reply + Route) Design

**Spec**: `.specs/features/redirect/spec.md`

## Architecture Overview

```
internal/execution/reply.go            internal/route/route.go
        │                                       │
        ├── Reply.Redirect(url, status...)      ├── Route.Redirect(url, status...) *Route
        │      SetHeader("Location", url)       │      r.Response(code)   -- docs (OpenAPI)
        │      SetStatus(code)                  │      r.handler = func(c) {
        │      SendString("")                   │        c.Response().Redirect(url, code)
        │                                       │      }
        │                                       │
        └── mesmo padrão de Json/Html/Text      └── sugar sobre o MESMO campo `handler`
            (força header/status próprios)          que Handler(fn) já popula -- zero
                                                     mudança em internal/app (dispatch)

root gonest package: nenhuma mudança -- Reply/Route já são type alias
(`type Reply = execution.Reply`, `type Route = route.Route`), método novo
fica visível em gonest.Reply/gonest.Route automaticamente (mesmo mecanismo
descrito em AD-061/http-context-unify).
```

`Route.Redirect` chama `Reply.Redirect` internamente (não duplica a lógica de
header/status) — garante que os dois caminhos (estático via Route, dinâmico via
Reply direto no Handler) produzem exatamente a mesma resposta HTTP.

---

## Components

### `Reply.Redirect` (novo método)

- **Purpose**: caso dinâmico — redirect com URL calculada em runtime (ex.: URL de
  provider OAuth só existe depois do usecase rodar). Chamado direto de dentro de
  um `Handler`.
- **Location**: `internal/execution/reply.go`, logo após `Text` (mesmo agrupamento
  de métodos "escreve corpo + força headers").
- **Signature**: `func (res *Reply) Redirect(url string, status ...int) error`
- **Dependencies**: `net/http` (só a constante `http.StatusFound` — precisa de
  novo import, `reply.go` hoje só importa `"bufio"`)
- **Reuses**: `res.res.SetHeaderValue`/`SetStatus`/`SendString` — os mesmos 3
  métodos do `Responder` que `Json`/`Html`/`Text` já usam, nenhuma capability
  nova exigida do adapter Fiber.

### `Route.Redirect` (novo método)

- **Purpose**: caso estático — redirect fixo declarado na Route, sem escrever um
  `Handler` manualmente (ex.: `/docs` → `/docs/`).
- **Location**: `internal/route/route.go`, logo após `Handler`/`HandlerFunc`
  (mesmo agrupamento — ambos populam `r.handler`).
- **Signature**: `func (r *Route) Redirect(url string, status ...int) *Route`
- **Dependencies**: `net/http` (mesma constante `http.StatusFound` — `route.go`
  hoje não importa `net/http`, precisa de novo import), `internal/execution`
  (já importado — `*execution.HttpContext` já é tipo do parâmetro de `Handler`)
- **Reuses**: `r.Response(code)` (já existe, documenta status no OpenAPI) e
  `Reply.Redirect` (chamado de dentro do handler interno que `Route.Redirect`
  monta) — nenhuma lógica de header/status duplicada aqui.

---

## Data Models

Nenhum novo tipo — os dois métodos são funções puras sobre tipos já existentes
(`*Reply`, `*Route`). Nenhuma mudança em `Responder` (interface do adapter) —
`SetHeaderValue`/`SetStatus`/`SendString` já existem e já são usados por
`Text`.

---

## Error Handling Strategy

`Reply.Redirect` retorna `error` (mesma assinatura de `Json`/`Html`/`Text` —
erro de escrita de rede propagado de `SendString`, não um erro de validação).
`Route.Redirect` não retorna `error` — é fluente (`*Route`, mesmo padrão de
`Summary`/`Tags`/`BearerAuth`), qualquer erro de escrita acontece só em
runtime dentro do handler interno e seria idêntico a um erro de `Text("")`
hoje (não tratado explicitamente em lugar nenhum do código atual — comportamento
inalterado).

---

## Tech Decisions (só as não óbvias)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Default de status | `http.StatusFound` (302) | Decisão explícita do usuário nesta sessão: paridade com o default do `@Redirect()` do NestJS, não com o `308` usado hoje no código consumidor (`erc`) — premissa geral do projeto é espelhar Nest. |
| `Route.Redirect` chama `Reply.Redirect` (não duplica header/status) | Composição, não duplicação | Garante caminho estático e dinâmico produzem a MESMA resposta HTTP; qualquer ajuste futuro (ex. Location relativo vs absoluto) muda em um lugar só. |
| `Route.Redirect` não aceita `Handler` simultâneo | Redirect e Handler são mutuamente exclusivos (última chamada vence, mesmo campo `r.handler`) | Mesma semântica que já existe hoje pra `Handler(fn)` chamado 2x — sem guarda nova, sem código extra; documentado no doc-comment do método. |
| Sem header `Location` documentado no OpenAPI | Fora de escopo (ver spec.md) | `Response(code)` hoje só documenta status, não headers — adicionar isso é uma feature própria de "response headers na doc", não faz parte do escopo de redirect. |

---

## Testing Strategy

- `internal/execution/reply_test.go`: `TestReply_Redirect_*` — segue o padrão de
  `TestReply_SetHeader_WritesToResponder`/`reply_test.go` já existente (fake
  `Responder`, assert em `SetHeaderValue("Location", ...)` + `GetStatus()` +
  corpo vazio). Casos: default 302 sem status explícito; status custom
  (`307`, `301`) sobrescrevendo o default.
- `internal/route/route_test.go`: `TestRoute_Redirect_*` — segue padrão de
  `TestHttpCode_StoresStatus`. Casos: `r.Redirect(url)` popula `r.handler` E
  `r.Response(302)` (checar via `r.Responses()`); status custom override;
  `HandlerFunc()` executado contra um fake `HttpContext`/`Responder` produz o
  mesmo resultado que `Reply.Redirect` isolado (evita duplicar toda a bateria
  de casos de `Reply.Redirect` — só confirma que a composição funciona).
- Nenhum teste de integração novo em `internal/app` necessário — dispatch não
  muda (spec.md's Out of Scope).

---

## Open Questions pra Tasks

- Nenhuma — spec.md fechou default de status e escopo; este design.md cobre
  os dois métodos, os imports novos (`net/http` em `reply.go` e `route.go`) e
  a estratégia de teste.
