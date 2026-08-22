# Spec: Redirect nativo (Reply + Route)

## Summary

Gonest existe pra refletir a ergonomia do NestJS em Go. NestJS resolve redirect via
`@Redirect(url, statusCode)` estático (decorator) e dinâmico (handler retorna
`{ url, statusCode }` sobrescrevendo o decorator). Gonest não tem retorno de handler
(tudo imperativo via `ctx.Response()`), então o equivalente vira dois métodos sugar:
`Reply.Redirect` (dinâmico, caso mais comum — URL só existe em runtime, ex.: OAuth
callback) e `Route.Redirect` (estático, redirect fixo sem precisar de `Handler`).
Elimina o padrão manual repetido `SetHeader("Location", ...)` + `Status(...)` +
`Text("")` visto hoje em código consumidor (`erc/ctrl/api/.../sso/controller.go`).

Origem: `.specs/insight/REDIRECT.md` (brainstorm desta sessão).

## Requirements

- REQ-001: `Reply.Redirect(url string, status ...int) error` — seta header
  `Location: url`, seta status (default `http.StatusFound`, 302 — paridade com
  default do NestJS `@Redirect()`), escreve corpo vazio. `status[0]` sobrescreve
  o default quando passado.
- REQ-002: `Route.Redirect(url string, status ...int) *Route` — registra a Route
  como redirect estático: documenta o status via `Response(code)` (mesmo
  mecanismo de doc que `HttpCode`/`Response` já usam) e popula `r.handler`
  internamente chamando `Reply.Redirect`. Mesma regra de default/override de
  status que REQ-001.
- REQ-003: `gonest.Reply`/`gonest.Route` (aliases públicos em `gonest.go`) expõem
  os dois métodos automaticamente — são type alias (`type Reply = execution.Reply`,
  `type Route = route.Route`), zero código adicional na raiz do módulo.

## Affected Components (from graph)

- `Reply` [`internal/execution/reply.go:17`] — recebe o novo método `Redirect`,
  mesmo padrão de `Json`/`Html`/`Text` (força headers/status, um método por
  comportamento).
- `Route` [`internal/route/route.go`] — recebe o novo método `Redirect`, sugar
  sobre o campo `handler` já existente (mesmo campo que `Handler(fn)` popula).
- `.Response()` [`internal/route/route.go:344`] — reusado por `Route.Redirect`
  pra documentar o status no OpenAPI, sem mudança de assinatura.
- `HttpContext.Response()` [`internal/execution/httpcontext.go:38`] — ponto de
  entrada existente, não muda; só passa a expor um método a mais no `*Reply`
  retornado.
- `gonest.go` — nenhuma mudança de código (aliases já cobrem `Reply`/`Route`),
  só documentação (comentário/exemplo, se aplicável).
- Nenhum God Node tocado — `Reply`/`Route` são componentes de borda (community
  `Reply`/`Route`), sem fan-in alto que exija cautela extra de blast radius.

## Out of Scope

- `RedirectException` ou qualquer redirect disparado de dentro de
  usecase/filter/camada interna — decisão do brainstorm (YAGNI, NestJS também
  não tem isso; só o controller/handler decide redirecionar).
- Redirect estático documentado com header `Location` explícito no OpenAPI
  (`Response(code)` hoje só documenta status, não headers) — mencionado como
  aberto no insight, fica de fora do v1 desta feature.
- Qualquer mudança em `internal/app` (dispatch/pipeline) — `Route.Redirect` é
  só açúcar que popula o mesmo campo `handler`, zero risco pro pipeline
  existente de middleware/guard/interceptor/filter.

## Open Questions

Nenhuma — default de status (302, `http.StatusFound`) e escopo já validados no
brainstorm (`.specs/insight/REDIRECT.md`).
