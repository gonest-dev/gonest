# Object Builder Tasks

**Design**: `.specs/features/object-builder/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T2, todos evaluator PASS)

---

## Execution Plan

```
T1 (internal/metadata: Object() + ObjectMetadata single-state builder) → T2 (root re-exports)
```

Sequencial (T2 depende do tipo `ObjectMetadata` existir).

---

## Task Breakdown

### T1: `ObjectMetadata` novo + `Object()` no `PropertyBuilder` ✅ DONE (evaluator: PASS, commit `6a602fb`)

**What**: estende `internal/metadata/metadata.go`'s `PropertyBuilder` com:
- `func (p *PropertyBuilder) Object(fn func(om *ObjectMetadata)) *ObjectMetadata` -- `p.format = "object"`, constrói `om := &ObjectMetadata{PropertyBuilder: p}`, roda `fn(om)`, devolve `om`

Cria arquivo NOVO `internal/metadata/object.go` (MESMO pacote):
- `type ObjectMetadata struct { *PropertyBuilder; ref *Metadata; additionalProperties bool }`
- `func (om *ObjectMetadata) Metadata(ref *Metadata) *ObjectMetadata` -- `om.ref = ref`, devolve `om`
- `func (om *ObjectMetadata) MetadataRef() (*Metadata, bool)` -- getter
- `func (om *ObjectMetadata) AdditionalProperties() *ObjectMetadata` -- `om.additionalProperties = true`, devolve `om`
- `func (om *ObjectMetadata) IsAdditionalProperties() bool` -- getter
- `Required()`/`Nullable()`/`Description(s)`/`Examples(...)` REDECLARADOS em `*ObjectMetadata`, delegando DIRETO ao `om.PropertyBuilder` (mesmo objeto do campo -- SEM estado duplo, ao contrário de `ArrayMetadata`)

**Where**: `internal/metadata/metadata.go` (existente, estendido), `internal/metadata/object.go` (novo), `internal/metadata/object_test.go` (novo)

**Depends on**: nenhuma

**Reuses**: `PropertyBuilder`'s `Required`/`Nullable`/`Description`/`Examples` (mesmo objeto, sem roteamento)

**Requirement**: OB-01 até OB-04

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator) -- ver AD-001/AD-003 em STATE.md

**Done when**:
- [x] `Object(fn)` seta `format == "object"` no campo, devolve `*ObjectMetadata` NOVO com `om.PropertyBuilder == p` (identidade de ponteiro, MESMO objeto, sem synthetic separado)
- [x] `fn` recebe o MESMO `*ObjectMetadata` que `Object` devolve (identidade de ponteiro, mesmo teste-padrão de `Items(fn)`)
- [x] `om.Metadata(ref)` armazena `ref` com identidade preservada, recuperável via `MetadataRef()` (`got == ref`)
- [x] `om.AdditionalProperties()` seta `IsAdditionalProperties() == true`
- [x] `Required`/`Nullable`/`Description`/`Examples` chamados DENTRO do callback E chamados FORA (encadeados no retorno de `Object(fn)`) produzem EXATAMENTE o mesmo resultado -- teste explícito comparando os dois casos (prova que não há distinção dentro/fora, ao contrário de `ArrayMetadata`)
- [x] Reproduzir os 2 casos verbatim do INSIGHT.md: `Address AddressEntity` (via `Metadata(addressMetadata)` + `Required`/`Description` dentro do callback) e `Metadata map[string]any` (via `AdditionalProperties()` dentro + `.Nullable().Description(...)` encadeado FORA)
- [x] Chamar `Object(fn)` duas vezes na mesma `*PropertyBuilder` não panica -- segunda `*ObjectMetadata` não carrega `ref`/`additionalProperties` da primeira
- [x] Gate check passa (evaluator rodou `go test ./... -race -count=1` fresh + `go vet` + `gofmt -l` nos 3 arquivos, tudo limpo)
- [x] Test count: 13 entregues (acima do alvo 10+)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add ObjectMetadata builder (ref + additionalProperties)`

---

### T2: Root re-exports ✅ DONE (evaluator: PASS, commit `1e734ba`)

**What**: pacote raiz `gonest` ganha `type ObjectMetadata = metadata.ObjectMetadata`. Per AD-009, ADICIONE à seção `// Metadata` já existente em `gonest.go`/`gonest_test.go`.
**Where**: `gonest.go` (existente), `gonest_test.go` (existente)
**Depends on**: T1
**Reuses**: `Metadata`/`PropertyBuilder`/`ArrayMetadata`/`NewMetadata[T]` já re-exportados
**Requirement**: OB-01 até OB-04 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.ObjectMetadata` resolve na raiz
- [x] Reprodução do exemplo `UserEntity.Address`/`Metadata` do INSIGHT.md através dos aliases raiz -- confirma correto (usando `AddressEntity` já registrada via `gonest.NewMetadata`)
- [x] Gate check passa (evaluator rodou `go test ./... -race -count=1` fresh + `go vet` + `gofmt -l`, tudo limpo)
- [x] Test count: 2 entregues

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): re-export ObjectMetadata at root`

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
| T1: ObjectMetadata + Object() | 2 arquivos, padrão mecânico SIMPLES (single-state, mais simples que ArrayMetadata) | ✅ Granular |
| T2: Root re-exports | Adiciona à seção existente (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, reflection puro | unit | unit | ✅ OK |
| T2 | Re-export + reprodução de exemplo | unit | unit | ✅ OK |

Nenhuma violação.
