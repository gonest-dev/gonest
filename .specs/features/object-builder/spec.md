# Object Builder Specification

## Problem Statement

Milestone 5's última peça: `Object()` cobre 2 casos de campo aninhado (`struct` Go direto, não `[]T`) -- (1) reusar um `*Metadata` já registrado via `NewMetadata[T]` como schema do campo (equivalente `$ref` do OpenAPI), e (2) schema livre/aberto (`map[string]any`, sem struct Go pra refletir, `AdditionalProperties`). Ao contrário de `Array()`, `Object()` NÃO tem conceito de "item" separado do campo -- o campo INTEIRO É o objeto aninhado, então não existe o estado duplo que `ArrayMetadata` precisou (AD-011, STATE.md). Ainda assim, o usuário fixou em INSIGHT.md que `Object()` sempre recebe CALLBACK (`func(om *gonest.ObjectMetadata)`), mesmo padrão de `Items()`, por consistência de API entre os dois branches estruturais.

## Goals

- [ ] `Property(&t.X)` (X `struct` Go, ex: `AddressEntity`) ganha `Object(fn func(om *gonest.ObjectMetadata))` -- seta `format = "object"` no campo, roda `fn(om)` passando o `*ObjectMetadata` do PRÓPRIO campo, devolve `om`
- [ ] `ObjectMetadata.Metadata(ref *gonest.Metadata)` -- reusa `*Metadata` já registrado (equivalente `$ref`), sem duplicar `Property`
- [ ] `ObjectMetadata.AdditionalProperties()` -- marca schema aberto (sem `ref`, campo tipicamente `map[string]any`)
- [ ] `Required()`/`Nullable()`/`Description()`/`Examples()` em `*ObjectMetadata` -- delegam ao MESMO `PropertyBuilder` do campo (sem ambiguidade de escopo, já que não existe "item" separado aqui)
- [ ] Reproduzir os 2 casos do INSIGHT.md: `Address AddressEntity` (via `Metadata(addressMetadata)`) e `Metadata map[string]any` (via `AdditionalProperties()`, com `.Nullable().Description(...)` encadeado FORA do callback, no retorno de `Object(fn)`)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Leitura/uso de `ref`/`AdditionalProperties` pra OpenAPI/validação runtime | Milestones 6-7 |
| Validação de que `ref` é compatível com o tipo Go do campo (`AddressEntity` bate com o `*Metadata` passado) | Nenhum exemplo do INSIGHT.md valida isso; mesma postura "trust the caller" de toda feature anterior |

---

## User Stories

### P1: Object reusando metadata já registrada ($ref) ⭐ MVP

**User Story**: Como usuário do gonest, quero `m.Property(&t.Address).Object(func(om *gonest.ObjectMetadata) { om.Metadata(addressMetadata); om.Required(); om.Description("Endereço principal") })` (INSIGHT.md) reusando `addressMetadata` (já criado via `NewMetadata[AddressEntity]`) como o schema do campo `Address`, sem duplicar `Property`.

**Acceptance Criteria**:

1. WHEN `Object(fn)` é chamado num `*PropertyBuilder` THEN sistema SHALL setar `format = "object"` e devolver `*ObjectMetadata` NOVO, embedando o MESMO `*PropertyBuilder` do campo (sem estado extra tipo "item" -- diferente de `Array()`)
2. WHEN `fn` é executado THEN sistema SHALL passar pra ele o MESMO `*ObjectMetadata` que `Object` devolve (identidade de ponteiro, mesmo padrão de `Items(fn)`)
3. WHEN `om.Metadata(ref)` é chamado DENTRO do callback THEN sistema SHALL armazenar `ref` (equivalente `$ref`), recuperável com identidade de ponteiro preservada
4. WHEN `om.Required()`/`om.Description(s)` é chamado (dentro OU fora do callback, encadeado no retorno de `Object(fn)`) THEN sistema SHALL mutar o `PropertyBuilder` do CAMPO (único estado que existe aqui -- sem ambiguidade possível)

**Independent Test**: reproduzir `Address AddressEntity` do INSIGHT.md verbatim; assertar `FormatValue() == "object"`, `Metadata()` (getter) devolve o MESMO ponteiro de `addressMetadata`, `Required`/`Description` armazenados no campo.

---

### P2: Object livre (schema aberto, `AdditionalProperties`)

**User Story**: Como usuário do gonest, quero `m.Property(&t.Metadata).Object(func (om *gonest.ObjectMetadata) { om.AdditionalProperties() }).Nullable().Description("Metadados abertos do usuário")` (INSIGHT.md) pra um campo `map[string]any` sem struct Go aninhada pra reusar -- `Nullable`/`Description` encadeados FORA do callback, no retorno de `Object(fn)`.

**Acceptance Criteria**:

1. WHEN `om.AdditionalProperties()` é chamado DENTRO do callback THEN sistema SHALL marcar o campo como schema aberto (flag booleano), sem `ref` associado
2. WHEN `Object(fn).Nullable().Description(s)` é encadeado FORA do callback (no retorno de `Object`) THEN sistema SHALL mutar o MESMO `PropertyBuilder` do campo que o callback já tinha acesso (nenhuma diferença de comportamento entre chamar dentro ou fora -- mesmo objeto)

**Independent Test**: reproduzir `Metadata map[string]any` do INSIGHT.md verbatim; assertar `IsAdditionalProperties() == true`, `Metadata()` (getter de ref) devolve `(nil, false)`, `Nullable`/`Description` armazenados corretamente mesmo chamados FORA do callback.

---

## Edge Cases

- WHEN `om.Metadata(ref)` E `om.AdditionalProperties()` são chamados no MESMO callback THEN sistema SHALL aceitar ambos sem panic -- último valor de cada um vale (nenhum exemplo do INSIGHT.md faz isso, mas não há razão pra proibir; consumidor futuro de OpenAPI decide qual prioridade, fora de escopo aqui)
- WHEN `Object(fn)` é chamado numa `*PropertyBuilder` que já teve outro branch (`.String().Object(fn)`) THEN sistema SHALL sobrescrever `format` pra `"object"` (last-write-wins, mesmo precedente de todo branch anterior)
- WHEN `fn` é `nil` THEN comportamento não especificado por nenhum exemplo do INSIGHT.md -- se `Items`/`Object` seguirem o mesmo padrão de Go idiomático (chamar `nil` func panica natural do runtime), aceitar esse comportamento sem tratamento especial (mesma postura "trust the caller")

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| OB-01 | P1: Object(fn) seta format=object, devolve *ObjectMetadata novo, roda fn(om) com identidade | T1 | Done |
| OB-02 | P1: Metadata(ref) armazena ref com identidade preservada | T1 | Done |
| OB-03 | P1/P2: Required/Nullable/Description/Examples em ObjectMetadata mutam o campo (dentro ou fora do callback) | T1 | Done |
| OB-04 | P2: AdditionalProperties() marca schema aberto | T1 | Done |

**ID format:** `OB-[NUMBER]`

**Coverage:** 4 total, 4 mapped.

---

## Success Criteria

- [x] Os 2 casos do INSIGHT.md (`Address`, `Metadata`) compilam e armazenam corretamente
- [x] `Required`/`Nullable`/`Description`/`Examples` funcionam idênticos chamados dentro ou fora do callback (mesmo objeto, sem estado duplo)
- [x] Zero regressões na suite existente (`go test ./... -race` verde, commits `6a602fb`/`1e734ba`)
