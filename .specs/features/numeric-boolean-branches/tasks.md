# Numeric & Boolean Branches Tasks

**Design**: `.specs/features/numeric-boolean-branches/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T2, todos evaluator PASS)

---

## Execution Plan

```
T1 (internal/metadata: 4 branches numéricos + NumericMetadata + Boolean sem wrapper) → T2 (root re-exports)
```

Sequencial.

---

## Task Breakdown

### T1: `NumericMetadata` novo + 4 branches numéricos + `Boolean()` ✅ DONE (evaluator: PASS, commit `45e5d22` — identidade de ponteiro do Boolean confirmada genuína, teste crítico de chain confirmado)

**What**: estende `internal/metadata/metadata.go`'s `PropertyBuilder` (já existente) com:
- `func (p *PropertyBuilder) Integer() *NumericMetadata` — `p.format = "int64"`
- `func (p *PropertyBuilder) Int32() *NumericMetadata` — `p.format = "int32"`
- `func (p *PropertyBuilder) Float() *NumericMetadata` — `p.format = "float"`
- `func (p *PropertyBuilder) Double() *NumericMetadata` — `p.format = "double"`
- `func (p *PropertyBuilder) Boolean() *PropertyBuilder` — `p.format = ""`, devolve `p` DIRETO (SEM wrapper novo — primeiro branch que não precisa de tipo próprio, ver design.md's Tech Decisions)

Cria arquivo NOVO `internal/metadata/numeric.go` (MESMO pacote):
- `type NumericMetadata struct { *PropertyBuilder; min, max *int }` (SEM campo `pattern` — família numérica não tem validador tipo regex)
- `func (n *NumericMetadata) Min(v int) *NumericMetadata` / `func (n *NumericMetadata) Max(v int) *NumericMetadata`
- Getters: `MinValue() (int, bool)`, `MaxValue() (int, bool)`
- `Required()`/`Nullable()`/`Description(s)`/`Examples(...)` REDECLARADOS em `*NumericMetadata` — MESMO padrão exato de `StringMetadata` (mecânico, já resolvido, só repita)

**Where**: `internal/metadata/metadata.go` (existente, estendido), `internal/metadata/numeric.go` (novo), `internal/metadata/numeric_test.go` (novo)

**Done when**:
- [x] Cada um dos 4 branches numéricos devolve `*NumericMetadata` com `FormatValue()` correto (`"int64"`/`"int32"`/`"float"`/`"double"`), confirmado individualmente
- [x] `Min`/`Max` armazenam corretamente, cada um devolve o MESMO `*NumericMetadata` (chain continua)
- [x] `Required`/`Nullable`/`Description`/`Examples` chamados em `*NumericMetadata` mutam o `PropertyBuilder` compartilhado E continuam devolvendo `*NumericMetadata` — teste `.Required().Min(5)` numa chamada só (mesmo teste crítico já usado em `StringMetadata`, repetido aqui)
- [x] `Boolean()` devolve o MESMO `*PropertyBuilder` (não um tipo novo) — confirme via identidade de ponteiro (`got == p`), com `format == ""`, e que `Required()`/`Nullable()`/`Description()`/`Examples()` funcionam nele normalmente (já funcionam, são métodos do próprio `PropertyBuilder` — só confirme que não quebrou nada)
- [x] Chain completo reproduzindo INSIGHT.md: `m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))` e `m.Property(&t.IsActive).Boolean().Required().Description("Status do usuário").Examples(true)` — ambos funcionam ponta a ponta
- [x] Chamar `.Integer()` e depois `.Boolean()` (ou vice-versa) no MESMO `*PropertyBuilder` não panica — `format` reflete a última chamada (`""` se `Boolean()` foi por último)
- [x] Gate check passa
- [x] Test count: 12+ (4 branches numéricos individualmente + Min/Max chain + 4 comuns via NumericMetadata + Boolean sem wrapper + chain INSIGHT.md + cross-branch last-write-wins)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add NumericMetadata (Integer/Int32/Float/Double) and wrapper-less Boolean branch`

---

### T2: Root re-exports ✅ DONE (evaluator: PASS, commit `3b32728`)

**What**: pacote raiz `gonest` ganha `type NumericMetadata = metadata.NumericMetadata`. Per AD-009, ADICIONE à seção `// Metadata` já existente em `gonest.go`/`gonest_test.go`. `Boolean()` não precisa de re-export próprio (é só um método a mais em `PropertyBuilder`, já re-exportado).
**Where**: `gonest.go` (existente), `gonest_test.go` (existente)
**Depends on**: T1
**Reuses**: `Metadata`/`PropertyBuilder`/`StringMetadata`/`NewMetadata[T]` já re-exportados
**Requirement**: NUM-01 até NUM-04 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.NumericMetadata` resolve na raiz
- [x] Reprodução do exemplo `UserEntity.Id`/`UserEntity.IsActive` do INSIGHT.md através dos aliases raiz — confirma correto
- [x] `Int32()`/`Float()`/`Double()` (não mostrados no INSIGHT.md) também exercitados via aliases raiz, `FormatValue()` conferido pra cada
- [x] Gate check passa
- [x] Test count: 3+ (NumericMetadata resolve, reprodução INSIGHT.md Integer+Boolean, os 3 branches numéricos restantes)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): re-export NumericMetadata at root`

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
| T1: NumericMetadata + branches numéricos + Boolean | 2 arquivos, padrão mecânico já resolvido + 1 caso novo simples (Boolean) | ✅ Granular |
| T2: Root re-exports | Adiciona à seção existente (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, reflection puro | unit | unit | ✅ OK |
| T2 | Re-export + reprodução de exemplo | unit | unit | ✅ OK |

Nenhuma violação.
