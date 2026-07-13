# Module Composition Specification

## Problem Statement

Feature "Module Composition" do ROADMAP.md (Milestone 1) — `NewModule`, `Imports`, `Providers`, `Controllers`, ordem topológica do import graph. Ao verificar o estado atual do código: **toda a capacidade central já foi entregue** como efeito colateral de construir "Provider & DI Graph" (T3 definiu `internal/module` completo, T7 adicionou os accessors e o auto-wiring de Stage 1). Único gap real encontrado: `Module` não tem nome legível — mensagens de erro caem em fallback `fmt.Sprintf("%p", m)` (endereço de ponteiro), ruim pra DX.

## Goals

- [x] `NewModule`/`Imports`/`Providers`/`Controllers`/`Exports` públicos e funcionais (já entregue)
- [x] Ordem topológica do import graph — BFS com dedup de diamante, já testado (já entregue)
- [ ] `Module` aceita um nome legível opcional, usado em mensagens de erro no lugar do fallback de endereço

## Out of Scope

| Feature | Reason |
| --- | --- |
| Route registration em Controller | Feature separada "Controller & Route Registration" |
| Listen/bootstrap completo | Feature separada "App Bootstrap & Listen" |

---

## User Stories

### P1: Module Naming pra mensagens de erro legíveis ⭐ MVP

**User Story**: Como dev gonest, quero dar um nome opcional ao meu `Module` (`module.Name("UserModule")`) pra que erros de bootstrap (ex: "module X exports provider *Y it does not declare", "type *X exists in module Y but is not exported") mostrem esse nome em vez de um endereço de ponteiro.

**Why P1**: é o único gap real da feature — resto já está entregue e testado.

**Acceptance Criteria**:

1. WHEN `module.Name("Foo")` é chamado THEN mensagens de erro que hoje usam o fallback de endereço SHALL usar `"Foo"` no lugar
2. WHEN `Name` nunca é chamado THEN o comportamento SHALL continuar exatamente como está hoje (fallback `%p`) — não quebra nada existente
3. WHEN `Name` é chamado mais de uma vez no mesmo módulo THEN a última chamada SHALL vencer (comportamento de setter simples, sem validação de duplicidade)

**Independent Test**: criar 2 módulos, nomear só um, provocar erro de export inválido em cada, confirmar que o nomeado mostra o nome e o outro continua com o fallback de endereço.

---

## Edge Cases

- WHEN `Name("")` (string vazia) é chamado THEN comportamento é o mesmo que nunca ter chamado — cai no fallback (evita nome legível virar string vazia nas mensagens)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| MODC-01 | P1: Module Naming | Execute | Pending |

**Coverage:** 1 total, 1 mapped (via Execute direto, sem tasks.md formal — escopo Medium)

---

## Success Criteria

- [ ] Mensagens de erro de módulo nomeado mostram o nome, não o endereço
- [ ] Zero regressão nos 95 testes existentes
