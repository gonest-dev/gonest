# Redirect (Reply + Route) Tasks

**Design**: `.specs/features/redirect/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T2, QA PASS)

**Papéis desta feature (pedido explícito do usuário nesta sessão, substitui o
Planner/Implementer/Evaluator padrão só aqui):**
- **PO** — valida que a task, antes de começar, ainda reflete spec.md/design.md
  (aceite de escopo). Sem código.
- **DEV** — implementa a task + escreve os testes descritos em "Done when".
- **QA** — roda o Gate, confere cada item de "Done when" de forma independente,
  só então marca `[x]`.

Ciclo por task: **PO (aceite) → DEV (implementa) → QA (valida)**. QA reprova →
volta pro DEV com o motivo anotado na task; não pula etapa.

---

## Execution Plan

```
T1 (Reply.Redirect) → T2 (Route.Redirect)
```

Sequencial: T2 chama `Reply.Redirect` internamente (design.md's Architecture
Overview) — precisa de T1 pronto e testado primeiro. Feature pequena (2 tasks),
sem paralelismo real (mesmo raciocínio de L-003 já registrado em STATE.md).

---

## Task Breakdown

### T1: `Reply.Redirect` ✅ DONE (QA: PASS)

**What**: `internal/execution/reply.go` — novo método `Redirect(url string, status ...int) error`. Import novo: `net/http` (só `http.StatusFound`). Segue o padrão de `Text`/`Json`/`Html` já existentes no mesmo arquivo (doc-comment explicando o "porquê", não o "o quê").
**Where**: `internal/execution/reply.go`, `internal/execution/reply_test.go`
**Depends on**: None
**Reuses**: `Responder.SetHeaderValue`/`SetStatus`/`SendString` (já usados por `Text`)
**Requirement**: REQ-001

**PO — aceite**:
- [x] Assinatura bate com design.md (`func (res *Reply) Redirect(url string, status ...int) error`)
- [x] Default 302 (`http.StatusFound`) confirmado, não 308

**Done when (DEV implementa, QA valida)**:
- [x] `Redirect(url)` sem status: seta `Location: url`, status `302`, corpo vazio
- [x] `Redirect(url, 307)` / `Redirect(url, 301)`: status explícito sobrescreve o default
- [x] Segue exatamente o padrão de `TestReply_SetHeader_WritesToResponder` (fake `Responder`, assert em `SetHeaderValue`/`GetStatus`)
- [x] Gate check passa (`go test ./internal/execution/...`)
- [x] Test count: 2+ (default + override — table-driven aceitável)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(execution): add Reply.Redirect for dynamic HTTP redirects`

---

### T2: `Route.Redirect` ✅ DONE (QA: PASS)

**What**: `internal/route/route.go` — novo método `Redirect(url string, status ...int) *Route`. Import novo: `net/http` (só `http.StatusFound`; `route.go` hoje não importa `net/http`). Chama `r.Response(code)` (documenta status no OpenAPI) e popula `r.handler` com uma closure que chama `c.Response().Redirect(url, code)`.
**Where**: `internal/route/route.go`, `internal/route/route_test.go`
**Depends on**: T1 (chama `Reply.Redirect` internamente)
**Reuses**: `Route.Response`, `Reply.Redirect` (T1)
**Requirement**: REQ-002, REQ-003 (aliases já cobrem, sem código extra — QA confirma no smoke test)

**PO — aceite**:
- [x] Assinatura bate com design.md (`func (r *Route) Redirect(url string, status ...int) *Route`)
- [x] Confirma que NÃO duplica lógica de header/status (só chama `Reply.Redirect`)

**Done when (DEV implementa, QA valida)**:
- [x] `r.Redirect(url)` popula `r.handler` (via `r.HandlerFunc() != nil`) E documenta status 302 (via `r.Responses()[302]` existir)
- [x] `r.Redirect(url, 301)`: status custom sobrescreve o default, documentado e usado pelo handler
- [x] Rodar `r.HandlerFunc()` contra um fake `HttpContext`/`Responder` produz `Location`/status idênticos a chamar `Reply.Redirect` isolado (confirma composição, não duplica bateria de T1)
- [x] Smoke test em `gonest_test.go` (nível raiz): `gonest.Route.Redirect` resolve e funciona via alias público (REQ-003)
- [x] Gate check passa (`go test ./...`)
- [x] Test count: 3+ (default, override, composição-com-Reply — smoke test à parte)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(route): add Route.Redirect for static HTTP redirects`

---

## Parallel Execution Map

```
Fully sequential: T1 → T2
```

**Papéis por task:** PO aceita escopo antes de começar → DEV implementa código + testes → QA roda Gate e confere "Done when" item a item, só então marca `[x]` e libera a próxima task.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Reply.Redirect | 1 método novo, 1 arquivo, sem dependência externa nova além de `net/http` | ✅ Granular |
| T2: Route.Redirect | 1 método novo, 1 arquivo, depende só de T1 | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Método em `Reply` (write-side HTTP, sem I/O real — fake `Responder`) | unit | unit | ✅ OK |
| T2 | Método em `Route` (declarativo, sem I/O real — fake `HttpContext`) | unit | unit | ✅ OK |

Nenhuma violação. Nenhum teste de integração em `internal/app` necessário — dispatch não muda (spec.md's Out of Scope).
