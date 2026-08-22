# Default response description Tasks

**Design**: `.specs/features/response-default-description/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1, QA PASS)

**Papéis (mesmo padrão da feature `redirect`, PO/DEV/QA):**
- **PO** — valida que a task ainda reflete spec.md/design.md antes de começar.
- **DEV** — implementa + escreve os testes descritos em "Done when".
- **QA** — roda o Gate, confere "Done when" item a item, só então marca `[x]`.

---

## Execution Plan

```
T1 (buildResponses default description)
```

Única task — fix de 2 linhas numa função já existente, sem dependências
internas novas.

---

## Task Breakdown

### T1: `buildResponses` — default description = `http.StatusText(status)` ✅ DONE (QA: PASS)

**What**: `internal/openapi/generate.go` — troca `map[string]any{"description": ""}` por `map[string]any{"description": http.StatusText(status)}` no branch de zero-`Response()` (usa `r.Code()`) E no branch de status documentado sem schema-de-erro (usa `status`). `defaultErrorResponse` fica inalterado (já correto). `resp.DescriptionText()` override continua rodando por último, sem mudança de ordem.
**Where**: `internal/openapi/generate.go`, teste em `internal/openapi/generate_test.go` (ou o arquivo que já cobre `buildResponses`/`Generate` hoje — DEV confirma o nome real antes de escrever)
**Depends on**: None
**Reuses**: `http.StatusText` (já importado, já usado em `defaultErrorResponse`)
**Requirement**: REQ-001, REQ-002, REQ-003, REQ-004

**PO — aceite**:
- [x] Confirma que a mudança é só nos 2 pontos mapeados em design.md, nada em `route.Response`/`route.Route`
- [x] Confirma que `.Description(...)` explícito continua ganhando (REQ-003 não regride)

**Done when (DEV implementa, QA valida)**:
- [x] `Response(200)` sem `.Description()` → `description == "OK"`
- [x] `Response(201, func(r){ r.Schema(...) })` sem `.Description()` → `description == "Created"`
- [x] Rota sem nenhum `Response()` chamado → `description == "OK"` (default `r.Code()` 200)
- [x] `Response(200, func(r){ r.Description("custom") })` → `description == "custom"` (override intacto)
- [x] `Response(404)` sem schema → `description == "Not Found"` (caminho de erro, regressão — já passava, confirma que não quebrou)
- [x] Gate check passa (`go test ./internal/openapi/...`)
- [x] Test count: 5+ (os 5 casos acima, table-driven aceitável)

**Tests**: unit
**Gate**: quick

**Commit**: `fix(openapi): default response description to http.StatusText for every status`

---

## Parallel Execution Map

```
Única task: T1
```

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: buildResponses default description | 2 linhas em 1 função existente, 1 arquivo | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Gerador OpenAPI, função pura (sem I/O) | unit | unit | ✅ OK |

Nenhuma violação.
