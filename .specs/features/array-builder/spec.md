# Array Builder Specification

## Problem Statement

Milestone 4 fechou os branches primitivos (String/Numeric/Boolean/DateTime family), todos achatados direto em `*PropertyBuilder`. `Array()` é o primeiro branch que precisa de um ESTADO DUPLO: o campo container (`Tags []string`, format `"array"`, com seu próprio `Required`/`Nullable`/`Description`/`Examples`) e o ITEM dentro dele (formato/validadores do que cada elemento do slice é -- string com `Min/Max` de tamanho, número com `Min/Max` de faixa, ou objeto aninhado via `Object(ref)`). Um builder linear puro (padrão das features anteriores) não separa os dois escopos sem ambiguidade -- INSIGHT.md foi atualizado pelo usuário nesta sessão pra resolver isso com CALLBACK: `Items(func(m *gonest.ArrayMetadata) {...})`.

## Goals

- [ ] `Property(&t.X)` (X `[]T`) ganha `Array()` -- seta `format = "array"` no `PropertyBuilder` do campo, devolve `*ArrayMetadata` novo
- [ ] `ArrayMetadata.Items(fn func(m *ArrayMetadata))` -- chama `fn(am)` passando o PRÓPRIO `*ArrayMetadata` (não um tipo novo)-- dentro do callback, `m.Required()`/`Nullable()`/`Description()`/`Examples()` SEMPRE mutam o campo container (mesmo objeto `am`); `m.String()`/`m.Integer()`/`m.Int32()`/`m.Float()`/`m.Double()`/`m.Boolean()`/`m.DateTime()`/`m.Date()`/`m.Object(ref)` configuram o ITEM (delegam pra um `*PropertyBuilder` sintético interno, reusando `StringMetadata`/`NumericMetadata` já existentes pra ganhar `Min`/`Max` do item de graça)
- [ ] `Items(fn)` devolve o `*ArrayMetadata` (`am`) -- permite encadear `Min`/`Max` DEPOIS do callback pra quantidade de itens do array (`Items(fn).Min(1)`)
- [ ] `ArrayMetadata.Min(v int)`/`Max(v int)` -- quantidade de itens do array (distinto do `Min`/`Max` do item, que vive no wrapper devolvido por `m.String()`/`m.Integer()` dentro do callback)
- [ ] Reproduzir os 3 casos do INSIGHT.md (`Tags []string`, `Scores []int`, `Addresses []AddressEntity` via `Object(addressMetadata)`)

## Out of Scope

| Feature | Reason |
| --- | --- |
| `Object()` builder standalone (não-array) | Feature separada "Object Builder", mesmo Milestone 5 |
| `Unique` (INSIGHT.md menciona no comentário de branches previstos, mas nenhum exemplo usa) | Não especificado em nenhum exemplo concreto -- adicionar depois se aparecer caso de uso real |
| Leitura/uso de format+validators pra OpenAPI/validação runtime | Milestones 6-7 |

---

## User Stories

### P1: Array de tipo primitivo (String/Numeric item) ⭐ MVP

**User Story**: Como usuário do gonest, quero `m.Property(&t.Tags).Array().Items(func(m *gonest.ArrayMetadata) { m.String().Min(1).Max(50); m.Required(); m.Description("Tags do usuário"); m.Examples("admin", "beta") })` (exemplo verbatim do INSIGHT.md) compilando e armazenando corretamente -- tanto pra item String quanto item Integer (`Scores []int`).

**Acceptance Criteria**:

1. WHEN `Array()` é chamado num `*PropertyBuilder` THEN sistema SHALL setar `format = "array"` e devolver `*ArrayMetadata` NOVO (não o `*PropertyBuilder` base -- ao contrário de `Boolean()`/`DateTime()`, `Array()` PRECISA de estado extra: item)
2. WHEN `Items(fn)` é chamado num `*ArrayMetadata` THEN sistema SHALL executar `fn(am)` passando o MESMO `*ArrayMetadata` (identidade de ponteiro) e devolver `am`
3. WHEN `m.String()`/`m.Integer()`/`m.Int32()`/`m.Float()`/`m.Double()`/`m.Boolean()`/`m.DateTime()`/`m.Date()` é chamado DENTRO do callback (no `m` recebido) THEN sistema SHALL configurar um `*PropertyBuilder` INTERNO sintético (item, sem `field` real associado) e devolver o wrapper correspondente (`*StringMetadata`/`*NumericMetadata`/bare item builder pra Boolean/DateTime/Date) -- reusa os branches JÁ EXISTENTES sem duplicar código
4. WHEN `m.Required()`/`m.Nullable()`/`m.Description(s)`/`m.Examples(...)` é chamado no `*ArrayMetadata` (dentro OU fora do callback) THEN sistema SHALL mutar o `PropertyBuilder` do CAMPO CONTAINER (nunca o item)
5. WHEN `m.Min(v)`/`m.Max(v)` é chamado no `*ArrayMetadata` DIRETO (fora do callback, encadeado após `Items(fn)`) THEN sistema SHALL armazenar quantidade MIN/MAX de itens do array (campo próprio de `ArrayMetadata`, distinto do `Min`/`Max` do item)

**Independent Test**: reproduzir `Tags []string` (item String, `Min(1).Max(50)`) e `Scores []int` (item Integer, `Min(0).Max(100)`) do INSIGHT.md verbatim; assertar `FormatValue()` do campo (`"array"`), formato do item (`"string"`/`"int64"`), `Min`/`Max` do item, `Required`/`Description`/`Examples` do CAMPO (não do item).

---

### P2: Array de Object aninhado (reuso de metadata registrada)

**User Story**: Como usuário do gonest, quero `m.Property(&t.Addresses).Array().Items(func(m *gonest.ArrayMetadata) { m.Object(addressMetadata); m.Required(); m.Description("Endereços do usuário") }).Min(1)` (INSIGHT.md) reusando um `*Metadata` já criado via `NewMetadata[AddressEntity]` como o schema do item, sem duplicar `Property`.

**Acceptance Criteria**:

1. WHEN `m.Object(ref)` é chamado DENTRO do callback de `Items` (`ref` = `*Metadata` já registrado) THEN sistema SHALL armazenar `ref` como o schema do item (equivalente `$ref`), sem re-percorrer `Property` do tipo aninhado
2. WHEN `Items(fn).Min(1)` é chamado (Min FORA do callback, encadeado no retorno de `Items`) THEN sistema SHALL armazenar `1` como quantidade MÍNIMA de itens (não confundir com `Min`/`Max` de item, que não se aplica quando o item é `Object(ref)`)

**Independent Test**: reproduzir o caso `Addresses []AddressEntity` do INSIGHT.md verbatim; assertar que o item schema devolvido é o MESMO ponteiro de `addressMetadata` (identidade), quantidade mínima `1` armazenada no campo container, `Required`/`Description` no campo.

---

## Edge Cases

- WHEN `m.String()` (ou outro branch de item) é chamado MAIS DE UMA VEZ dentro do mesmo callback `Items(fn)` THEN sistema SHALL sobrescrever o formato do item (last-write-wins, mesmo precedente de todo branch anterior, sem panic)
- WHEN `Array()` é chamado numa `*PropertyBuilder` que já teve outro branch (`.String().Array()`) THEN sistema SHALL sobrescrever `format` pra `"array"` (last-write-wins, sem panic -- precedente já estabelecido)
- WHEN `Items(fn)` nunca é chamado (só `Array()`) THEN sistema SHALL deixar o item sem formato configurado (zero value), sem panic -- consumidor futuro (OpenAPI Milestone 7) decide como tratar item vazio, fora de escopo aqui

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| AR-01 | P1: Array() seta format=array, devolve *ArrayMetadata novo | T1 | Done |
| AR-02 | P1: Items(fn) executa fn(am) passando o mesmo *ArrayMetadata, devolve am | T1 | Done |
| AR-03 | P1: branches de item (String/Integer/etc) dentro do callback configuram item sintético, reusam StringMetadata/NumericMetadata | T1 | Done |
| AR-04 | P1: Required/Nullable/Description/Examples no ArrayMetadata sempre mutam o campo container | T1 | Done |
| AR-05 | P1: Min/Max no ArrayMetadata direto (fora do callback) = quantidade de itens | T1 | Done |
| AR-06 | P2: Object(ref) dentro do callback reusa *Metadata já registrado como schema do item | T1 | Done |

**ID format:** `AR-[NUMBER]`

**Coverage:** 6 total, 6 mapped.

---

## Success Criteria

- [x] Os 3 casos do INSIGHT.md (`Tags`, `Scores`, `Addresses`) compilam e armazenam corretamente
- [x] Item e campo container nunca se confundem (Required/Description sempre no campo; Min/Max do item vive só no wrapper devolvido dentro do callback)
- [x] Zero regressões na suite existente (`go test ./... -race` verde, commits `31a76b3`/`1c5ea45`)
