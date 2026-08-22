# INSIGHT-REDIRECT — Redirect nativo (Reply + Route) (rascunho evoluído via brainstorm)

Motivado pelo `sso/controller.go` (projeto externo `erc`), que reimplementa manualmente, rota a rota,
o que deveria ser um primitivo do framework:

```go
func redirectTo(ctx *gonest.HttpContext, location string) {
	ctx.Response().SetHeader("Location", location)
	ctx.Response().Status(gonest.HttpStatusPermanentRedirect).Text("")
}
```

Premissa geral do gonest (reafirmada nesta sessão): o framework existe pra refletir a ergonomia do
NestJS em Go. NestJS resolve redirect de dois jeitos — `@Redirect(url, statusCode)` **estático** no
decorator, e **dinâmico**, handler retornando `{ url, statusCode }` que sobrescreve o decorator. Gonest
não tem retorno de handler (tudo é imperativo via `ctx.Response()`), então o equivalente dos dois casos
do Nest vira dois métodos separados — um em `Reply` (dinâmico), um em `Route` (estático) — em vez de um
único decorator com override.

Escopo fechado nesta sessão: SEM `RedirectException`/redirect disparado de dentro de usecase/filter/
camada interna — YAGNI, o próprio NestJS também não tem isso, é sempre o controller (a camada HTTP) quem
decide redirecionar.

---

## 1. `Reply.Redirect` — caso dinâmico

`internal/execution/reply.go`, mesmo padrão de `Json`/`Html`/`Text` (força os próprios headers, um
método por content/comportamento):

```go
// Redirect writes a redirect response -- sets the Location header, the
// status code (302 Found by default, matching NestJS's own @Redirect()
// default), and an empty body. status, when given, overrides the default --
// same optional-trailing-arg shape as Route.Redirect.
func (res *Reply) Redirect(url string, status ...int) error {
	code := http.StatusFound // 302 -- NestJS @Redirect() default
	if len(status) > 0 {
		code = status[0]
	}
	res.res.SetHeaderValue("Location", url)
	res.res.SetStatus(code)
	return res.res.SendString("")
}
```

Substitui `redirectTo` diretamente: `ctx.Response().Redirect(authURL)` no lugar de
`SetHeader("Location", ...)` + `Status(...)` + `Text("")`. Cobre o caso do SSO — URL do provider OAuth só
existe depois do usecase rodar, então só o handler (camada HTTP) pode decidir o redirect final.

## 2. `Route.Redirect` — caso estático

`internal/route/route.go`, açúcar declarativo pra redirect fixo (ex.: `/docs` → `/docs/`), sem precisar
escrever um `Handler` manualmente:

```go
// Redirect registers this Route as a static redirect to url -- sugar over
// Handler that also documents the response status (Response(code)) for
// OpenAPI. status, when given, overrides the default 302 Found.
func (r *Route) Redirect(url string, status ...int) *Route {
	code := http.StatusFound
	if len(status) > 0 {
		code = status[0]
	}
	r.Response(code)
	r.handler = func(c *execution.HttpContext) {
		c.Response().Redirect(url, code)
	}
	return r
}
```

Não muda dispatch (`internal/app`) — internamente é só `r.handler = ...`, o mesmo campo que `Handler(fn)`
já preenche. Zero risco pro pipeline de middleware/guard/interceptor/filter existente: `Route.Redirect` é
só uma forma alternativa de popular o mesmo campo que `Handler` sempre populou.

---

## Uso alvo (sso/controller.go, pós-feature)

```go
r.Handler(func(ctx *gonest.HttpContext) {
	provider := ctx.Request().Param("provider")
	callbackURL := ctx.Request().Queries()["callback_url"]
	authURL, err := useCase.Execute(provider, callbackURL)
	if err != nil {
		panic(err)
	}
	ctx.Response().Redirect(authURL) // 302 default -- redirectTo() inteiro some
})
```

Aberto pra Design/Tasks quando formalizado: se `Route.Redirect` deve também aparecer documentado no
OpenAPI com um header `Location` explícito na resposta (hoje `Response(code)` só documenta o status, não
headers) -- decisão de Design, não decidida aqui.
