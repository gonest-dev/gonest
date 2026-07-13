# Controller & Route Registration Specification

## Problem Statement

Dev NestJS espera declarar `@Controller('/user')` + `@Get(':id')` e ter rota HTTP funcionando, com params tipados e body parseado. gonest precisa do equivalente sem decorators: `Controller.Path`/`Route`/`Handler`/`HttpCode`, um `Context` que encapsula request/response, e `MustParam[T]` com coerção de tipo. É a primeira feature que liga o grafo de DI (já pronto) a HTTP de verdade — decide o adapter (Fiber, único no v1 por PROJECT.md) e o contrato de `Context`.

## Goals

- [ ] `Controller`/`Route` registram rotas que respondem via Fiber de verdade (`go run` funcional servindo `/user/:id`, meta do Milestone 1 no ROADMAP.md)
- [ ] `MustParam[T]` converte param de string (Fiber sempre devolve string) pro tipo pedido, com erro claro se inválido
- [ ] `Context` nasce com API pública mínima (`Json`/`Status`/`Header`/`MustParam`) mas com hooks stub (`Use`/`Guards`/`Interceptors`/`Filters`, no-op) prontos pra feature seguinte (Request Pipeline) preencher sem quebrar assinatura

## Out of Scope

| Feature | Reason |
| --- | --- |
| Middleware/Guard/Interceptor/Filter (comportamento real) | Feature separada "Request Pipeline" (Milestone 3), decidida junto com o usuário — só os stubs no-op de `Context` entram aqui |
| Contrato de erro estruturado `{name,message,details}` (`HttpException`) | Feature separada "Exceptions & Response Contract" (Milestone 2) — panic não tratado aqui vira 500 genérico sem detalhe |
| `AppOptions`/`MustListen`/`OnListen` completos | Feature separada "App Bootstrap & Listen" — aqui só o suficiente pra rota resolver via um `NewApp`/`Listen` mínimo (reusa o que "Provider & DI Graph" T9 já entregou) |
| Multi-adapter HTTP (net/http, Echo, Gin) | Fora de escopo do v1 inteiro (PROJECT.md) |

---

## User Stories

### P1: Registro e dispatch de rota via Fiber ⭐ MVP

**User Story**: Como dev gonest, quero declarar `controller.Path("/user")` + `controller.Route(gonest.HttpGet, "/:user_id", func(route *gonest.Route) {...})` e ter essa rota respondendo via um servidor Fiber real.

**Why P1**: sem isso a feature não existe — é o alicerce de qualquer app HTTP no framework.

**Acceptance Criteria**:

1. WHEN `Controller.Path(prefix)` + `Controller.Route(method, path, fn)` são declarados dentro do `fn` adiado do Controller (mesmo padrão Stage 2 de Provider) THEN o bootstrap SHALL registrar essa rota no adapter Fiber com o path completo (`prefix + path`)
2. WHEN uma request HTTP bate numa rota registrada THEN o `Handler` daquela `Route` SHALL rodar recebendo um `*gonest.Context` que envolve o `*fiber.Ctx` da request
3. WHEN `route.HttpCode(status)` é declarado THEN a resposta SHALL usar esse status por padrão (a menos que o Handler chame `ctx.Status(...)` explicitamente, que sobrescreve)
4. WHEN `ctx.Json(value)` é chamado THEN a resposta SHALL serializar `value` como JSON no body com `Content-Type: application/json`
5. WHEN o `Handler` de uma rota panica com algo que NÃO é uma Exception estruturada (nil pointer, index out of range etc — Exceptions estruturadas são Milestone 2) THEN a resposta SHALL ser 500 genérico sem vazar detalhe interno, e o processo NÃO SHALL cair

**Independent Test**: subir app com `UserController` do INSIGHT.md (rotas List/Get/Create/Update/Delete adaptadas, sem MustResolve[*UserService] complexo — service simples em memória), bater as 5 rotas via HTTP real, confirmar status/body corretos.

---

### P2: MustParam[T] com coerção via Pipe

**User Story**: Como dev gonest, quero `gonest.MustParam[int64](ctx, "user_id")` convertendo o param string da URL pro tipo pedido, com panic claro se inválido — e poder customizar essa conversão declarando um `Pipe` (`route.Param("user_id", ParseIntPipe)`).

**Why P2**: sem coerção tipada, todo Handler teria que fazer `strconv` manual — quebra a promessa de DX do projeto.

**Acceptance Criteria**:

1. WHEN `MustParam[T](ctx, name)` é chamado sem nenhum `Pipe` customizado registrado pro param THEN SHALL usar coerção default via reflect+strconv pros tipos básicos (`string`, `int`, `int64`, `bool`, `float64`)
2. WHEN o valor do param não converte pro tipo pedido (default ou via Pipe) THEN SHALL panicar com uma exception clara (formato provisório, refinado em Milestone 2) — não deixar o erro vazar como panic genérico
3. WHEN `route.Param(name, somePipe)` é declarado THEN `MustParam[T](ctx, name)` SHALL rodar o `Handler` do Pipe em vez da coerção default pra esse param específico
4. WHEN `gonest.NewPipe(func(pipe *gonest.Pipe) { pipe.Handler(func(ctx *gonest.Context, raw string) T {...}) })` é declarado THEN o `Handler` SHALL receber a string bruta do param e devolver o valor tipado (ou panicar)

**Independent Test**: rota com `:user_id` sem Pipe custom resolvendo `int64` via default; rota separada com `ParseIntPipe` customizado fazendo a mesma coerção explicitamente; rota com param inválido (`"abc"` onde espera `int64`) confirma panic claro, não crash.

---

### P3: Context com stubs pro Request Pipeline futuro

**User Story**: Como mantenedor do framework, quero que `Context`/`Controller` já exponham `Use`/`Guards`/`Interceptors`/`Filters` (assinatura final, comportamento no-op) pra que a feature "Request Pipeline" só precise implementar o corpo, sem quebrar a API pública que já existir em apps reais.

**Why P3**: decisão explícita do usuário — antecipa a forma da API sem antecipar o comportamento, reduz retrabalho de breaking change depois.

**Acceptance Criteria**:

1. WHEN `controller.Use(...)`/`controller.Guards(...)`/`controller.Interceptors(...)`/`controller.Filters(...)` são chamados THEN SHALL aceitar os tipos que a feature "Request Pipeline" vai definir (`Middleware`/`Guard`/`Interceptor`/`Filter` — placeholders mínimos aqui, sem lógica) e apenas registrar/no-op, sem afetar o dispatch da rota nessa feature
2. WHEN nenhum desses métodos é chamado THEN comportamento SHALL ser idêntico a uma versão sem eles (dispatch direto Handler)

**Independent Test**: chamar `controller.Use(SomeMiddlewarePlaceholder)` num controller de teste, confirmar que a rota responde exatamente igual a um controller sem essa chamada (prova de no-op).

---

## Edge Cases

- WHEN duas rotas colidem (mesmo método + path, dois `Route` diferentes, considerando o prefixo do `Controller`) THEN `NewApp` SHALL detectar antes de registrar no Fiber e retornar erro claro (`"duplicate route: GET /user/:id"`) — mesmo padrão de "erro no bootstrap" já usado pra provider duplicado indevido na feature DI Graph
- WHEN `Controller` não declara `Path` (fica `""`) THEN prefixo SHALL ser vazio, rota registrada só com o path do `Route`
- WHEN `MustParam[T]` é chamado com um `name` que não existe na rota (typo) THEN SHALL panicar com mensagem clara identificando o param ausente, não devolver zero-value silenciosamente
- WHEN app não tem nenhum Controller registrado THEN bootstrap SHALL funcionar normalmente (servidor sobe sem rota nenhuma, não é erro)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| CTRL-01 | P1: Registro de rota via Fiber | Design | Pending |
| CTRL-02 | P1: Context.Json/Status | Design | Pending |
| CTRL-03 | P1: Panic genérico → 500 sem detalhe | Design | Pending |
| CTRL-04 | P2: MustParam default coercion | Design | Pending |
| CTRL-05 | P2: Pipe customizado via route.Param | Design | Pending |
| CTRL-06 | P2: Panic claro em coerção inválida | Design | Pending |
| CTRL-07 | P3: Stubs Use/Guards/Interceptors/Filters no-op | Design | Pending |
| CTRL-08 | Edge: colisão de rota → erro no bootstrap | Design | Pending |

**Coverage:** 8 total, 0 mapped to tasks, 8 unmapped ⚠️ (aguardando fase Design/Tasks)

---

## Success Criteria

- [ ] Exemplo `UserController` adaptado do INSIGHT.md compila e responde via HTTP real (Fiber)
- [ ] `MustParam[int64]` funciona com e sem Pipe customizado
- [ ] Panic não tratado nunca derruba o processo, sempre vira 500
