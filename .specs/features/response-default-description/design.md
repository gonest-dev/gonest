# Default response description Design

**Spec**: `.specs/features/response-default-description/spec.md`

## Architecture Overview

```
buildResponses(r, doc, visiting)
        │
        ├── len(responses) == 0 (rota nunca chamou Response())
        │      antes: out[r.Code()] = {"description": ""}
        │      depois: out[r.Code()] = {"description": http.StatusText(r.Code())}
        │
        └── por status documentado:
               !hasSchema && status >= 400 → defaultErrorResponse (JÁ correto,
                   sem mudança: description = http.StatusText(status))
               senão (sucesso, ou erro COM schema) →
                   antes: entry = {"description": ""}
                   depois: entry = {"description": http.StatusText(status)}
               (resp.DescriptionText() explícito continua sobrescrevendo
                por último, comportamento já existente, sem mudança)
```

Fix mecânico: os 2 pontos que hoje hardcodeiam `""` passam a chamar a MESMA
`http.StatusText(status)` que `defaultErrorResponse` já usa — nenhuma
função nova, nenhum tipo novo.

---

## Components

### `buildResponses` (alterado, não novo)

- **Purpose**: cada `description` sintetizada (sem `.Description()`
  explícito) passa a ser o nome do status HTTP (`http.StatusText`), em vez
  de string vazia — cobre TODO caminho, não só erro.
- **Location**: `internal/openapi/generate.go:197`
- **Mudança**: 2 linhas — `map[string]any{"description": ""}` vira
  `map[string]any{"description": http.StatusText(status)}` (e
  `http.StatusText(r.Code())` no caminho de zero-responses).
- **Dependencies**: `net/http` (já importado no arquivo)
- **Reuses**: nada novo — mesma função stdlib que `defaultErrorResponse`
  já chama.

---

## Data Models

Nenhum novo tipo — mudança de valor literal dentro de uma função já
existente.

---

## Error Handling Strategy

N/A — `http.StatusText` nunca erra, retorna `""` pra status desconhecido
(REQ-004, comportamento inalterado pra esse caso extremo).

---

## Tech Decisions (só as não óbvias)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Aplicar `http.StatusText` em TODOS os status, não só ≥400 | Sim, incondicional | O bug relatado é justamente um `201` (sucesso) sem description — o caminho de erro já estava certo, o de sucesso que faltava. |
| Não criar uma constante/tabela própria de status→nome | Reusar `http.StatusText` (stdlib) | Já é a fonte usada em `defaultErrorResponse` — duas fontes de nomes de status divergiria em caso de status custom/edge, sem ganho. |
| `.Description(...)` continua sendo a última palavra | Sem mudança na ordem de resolução (`resp.DescriptionText()` aplicado por último) | REQ-003 — comportamento existente, só o DEFAULT (o que roda quando `Description` nunca foi chamado) muda. |

---

## Testing Strategy

- `internal/openapi/generate_test.go` (ou arquivo equivalente já existente
  pra `buildResponses`/`Generate`): casos novos —
  - Rota com `Response(200)` sem `.Description()` → `description == "OK"`.
  - Rota com `Response(201, func(r){ r.Schema(...) })` sem `.Description()`
    → `description == "Created"`.
  - Rota sem nenhum `Response()` chamado (`r.Code()` default 200) →
    `description == "OK"`.
  - Rota com `Response(200, func(r){ r.Description("custom") })` →
    `description == "custom"` (prova que override continua funcionando,
    regressão do comportamento existente).
  - Caminho de erro (`Response(404)` sem schema) → continua
    `"Not Found"` (regressão, já passava, confirma que não quebrou).

---

## Open Questions pra Tasks

Nenhuma — fix mecânico de 1 função, já mapeado linha a linha.
