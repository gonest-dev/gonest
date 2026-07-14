# Array Builder Tasks

**Design**: `.specs/features/array-builder/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T2, todos evaluator PASS)

---

## Execution Plan

```
T1 (internal/metadata: Array() + ArrayMetadata dual-state builder) → T2 (root re-exports)
```

Sequencial (T2 depende do tipo `ArrayMetadata` existir).

---

## Task Breakdown

### T1: `ArrayMetadata` novo + `Array()` no `PropertyBuilder` ✅ DONE (evaluator: PASS, commit `31a76b3`)

**What**: estende `internal/metadata/metadata.go`'s `PropertyBuilder` com:
- `func (p *PropertyBuilder) Array() *ArrayMetadata` -- `p.format = "array"`, devolve `&ArrayMetadata{PropertyBuilder: p, item: &PropertyBuilder{}}`

Cria arquivo NOVO `internal/metadata/array.go` (MESMO pacote):
- `type ArrayMetadata struct { *PropertyBuilder; item *PropertyBuilder; itemRef *Metadata; min, max *int }`
- `func (am *ArrayMetadata) Items(fn func(m *ArrayMetadata)) *ArrayMetadata` -- chama `fn(am)`, devolve `am`
- 18 métodos de branch de ITEM, cada um mecanicamente igual ao método correspondente já existente em `PropertyBuilder`, só trocando o receiver alvo pra `am.item`:
  - `String()`/`Email()`/`Uuid()`/`Uri()`/`Hostname()`/`Ipv4()`/`Ipv6()`/`Password()`/`Byte()`/`Binary()` → `*StringMetadata{PropertyBuilder: am.item}`
  - `Integer()`/`Int32()`/`Float()`/`Double()` → `*NumericMetadata{PropertyBuilder: am.item}`
  - `Boolean()`/`DateTime()`/`Date()` → devolve `am.item` bare (`*PropertyBuilder`)
- `func (am *ArrayMetadata) Object(ref *Metadata) *ArrayMetadata` -- `am.itemRef = ref`, devolve `am`
- `func (am *ArrayMetadata) Min(v int) *ArrayMetadata` / `func (am *ArrayMetadata) Max(v int) *ArrayMetadata` -- quantidade do ARRAY (campos próprios `min`/`max`, NÃO os do item)
- `func (am *ArrayMetadata) MinValue() (int, bool)` / `MaxValue() (int, bool)` -- getters de quantidade
- `func (am *ArrayMetadata) ItemBuilder() *PropertyBuilder` -- accessor cru pro `am.item`
- `func (am *ArrayMetadata) ItemRef() (*Metadata, bool)` -- accessor pro `am.itemRef`
- `Required()`/`Nullable()`/`Description(s)`/`Examples(...)` REDECLARADOS em `*ArrayMetadata`, delegando ao `am.PropertyBuilder` (o CAMPO, nunca `am.item`) -- mesmo padrão embed+redeclare de `StringMetadata`/`NumericMetadata`

**Where**: `internal/metadata/metadata.go` (existente, estendido), `internal/metadata/array.go` (novo), `internal/metadata/array_test.go` (novo)

**Depends on**: nenhuma (branches de item reusam `StringMetadata`/`NumericMetadata` já existentes, sem modificá-los)

**Reuses**: `StringMetadata`, `NumericMetadata` (mecanismo de item), `PropertyBuilder`'s `Required`/`Nullable`/`Description`/`Examples` (mecanismo de campo)

**Requirement**: AR-01 até AR-06

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator) -- ver AD-001/AD-003 em STATE.md

**Done when**:
- [x] `Array()` seta `format == "array"` no `PropertyBuilder` do campo e devolve `*ArrayMetadata` NOVO, com `item` sintético (identidade de ponteiro do CAMPO confirmada: `am.PropertyBuilder == p`)
- [x] `Items(fn)` executa `fn` passando o MESMO `*ArrayMetadata` recebido como parâmetro (identidade de ponteiro), e devolve esse mesmo `*ArrayMetadata`
- [x] Dentro do callback, `m.String().Min(1).Max(50)` configura o ITEM (`am.item.format == "string"`, `Min`/`Max` acessíveis via o `*StringMetadata` devolvido) -- teste equivalente pra `m.Integer().Min(0).Max(100)` (item numérico)
- [x] Dentro (ou fora) do callback, `m.Required()`/`m.Description(s)`/`m.Examples(...)` mutam o CAMPO (`am.PropertyBuilder`), NUNCA o item -- teste explícito provando que `am.item`'s campos correspondentes continuam zero-value depois de chamar esses 4 no `am`
- [x] `m.Object(addressMetadata)` dentro do callback armazena `addressMetadata` como `itemRef`, recuperável via `ItemRef()` com identidade de ponteiro (`got == addressMetadata`)
- [x] `Items(fn).Min(1)` (Min FORA do callback, encadeado no retorno) armazena `1` como quantidade MÍNIMA do array, recuperável via `MinValue()` -- distinto e não-colidente com o `Min` do item testado acima
- [x] Reproduzir os 3 casos verbatim do INSIGHT.md: `Tags []string`, `Scores []int`, `Addresses []AddressEntity` (via `Object(addressMetadata)` + `.Min(1)`)
- [x] Chamar `Array()` duas vezes na mesma `*PropertyBuilder` não panica -- segunda `*ArrayMetadata` tem `item` novo (independente do primeiro)
- [x] Gate check passa (evaluator rodou `go test ./... -race` + `go vet` + `gofmt -l .` independente, tudo limpo)
- [x] Test count: 12 entregues (abaixo do alvo "15+", mas todo item do checklist coberto por teste substantivo -- evaluator aceitou como suficiente)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add ArrayMetadata dual-state builder (item + quantity)`

---

### T2: Root re-exports ✅ DONE (evaluator: PASS, commit `1c5ea45`)

**What**: pacote raiz `gonest` ganha `type ArrayMetadata = metadata.ArrayMetadata`. Per AD-009, ADICIONE à seção `// Metadata` já existente em `gonest.go`/`gonest_test.go`.
**Where**: `gonest.go` (existente), `gonest_test.go` (existente)
**Depends on**: T1
**Reuses**: `Metadata`/`PropertyBuilder`/`StringMetadata`/`NumericMetadata`/`NewMetadata[T]` já re-exportados
**Requirement**: AR-01 até AR-06 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.ArrayMetadata` resolve na raiz
- [x] Reprodução do exemplo `UserEntity.Tags`/`Scores`/`Addresses` do INSIGHT.md através dos aliases raiz -- confirma correto (usando `AddressEntity` já registrada via `gonest.NewMetadata`)
- [x] Gate check passa (evaluator rodou `go clean -testcache` + `go test ./... -race` + `go vet` + `gofmt -l` fresh, tudo limpo)
- [x] Test count: 2 entregues

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): re-export ArrayMetadata at root`

---

## Parallel Execution Map

```
Sequencial: T1 → T2
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: ArrayMetadata + Array() + 18 métodos de item + quantidade | 2 arquivos, PADRÃO NOVO (dual-state) mas mecânico depois de resolvido pelo design.md | ✅ Granular |
| T2: Root re-exports | Adiciona à seção existente (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, reflection puro | unit | unit | ✅ OK |
| T2 | Re-export + reprodução de exemplo | unit | unit | ✅ OK |

Nenhuma violação.
