# Metadata Registration Core Tasks

**Design**: `.specs/features/metadata-registration-core/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (internal/metadata: Metadata/PropertyBuilder core + field-offset identification) → T2 (root re-exports)
```

Sequencial — T2 depende inteiramente de T1.

---

## Task Breakdown

### T1: `internal/metadata` — `Metadata`/`PropertyBuilder` core ✅ DONE (evaluator: PASS, commit `f60415a` — técnica de offset via ponteiro confirmada empiricamente sem ajustes, incluindo campos `time.Time`/`*time.Time`)

**What**: pacote novo `internal/metadata/metadata.go`:
- `type Metadata struct { structType reflect.Type; baseAddr uintptr; description string; properties map[uintptr]*PropertyBuilder }` (todos não-exportados)
- `func New(structType reflect.Type, baseAddr uintptr) *Metadata` — construtor interno, type-erased (recebe `reflect.Type`/`uintptr`, NÃO `T` genérico — ver design.md's Tech Decisions sobre por que métodos não podem ser genéricos)
- `func (m *Metadata) Description(s string) *Metadata` / `func (m *Metadata) DescriptionText() string`
- `func (m *Metadata) Property(fieldPtr any) *PropertyBuilder` — identifica o campo via `reflect.ValueOf(fieldPtr).Pointer()` menos `m.baseAddr` = offset, busca em `reflect.VisibleFields(m.structType)` o campo com esse `.Offset`. Panica com mensagem clara se: offset não bate com nenhum campo (ponteiro estranho), OU offset já foi registrado antes (double-registration).
- `func (m *Metadata) OwnProperties() []*PropertyBuilder` — accessor de cópia defensiva
- `type PropertyBuilder struct { field reflect.StructField; required, nullable bool; description string; examples []any }` (não-exportados)
- `func (p *PropertyBuilder) Required() *PropertyBuilder` / `IsRequired() bool`
- `func (p *PropertyBuilder) Nullable() *PropertyBuilder` / `IsNullable() bool`
- `func (p *PropertyBuilder) Description(s string) *PropertyBuilder` / `DescriptionText() string`
- `func (p *PropertyBuilder) Examples(examples ...any) *PropertyBuilder` / `ExamplesList() []any` (cópia defensiva)
- `func (p *PropertyBuilder) Field() reflect.StructField`

**IMPORTANTE — risco técnico central desta task**: a identificação de campo via offset de ponteiro (`unsafe.Pointer` arithmetic) é uma técnica CONHECIDA em Go (usada por libs de mapeamento de struct), mas NÃO foi verificada empiricamente nesta sessão — é implementada a partir de conhecimento geral de Go, não confirmada contra doc externo autoritativo. SEU TRABALHO nesta task inclui confirmar empiricamente que funciona pros tipos de campo que o exemplo do INSIGHT.md usa: `int64`, `string`, `bool`, `time.Time`, `*time.Time`. Se a técnica NÃO funcionar como esperado pra algum desses tipos, isso é um ACHADO REAL a reportar claramente (não workaround silencioso) — pode significar que o design.md precisa de ajuste.

**Where**: `internal/metadata/metadata.go`, `internal/metadata/metadata_test.go`

**Done when**:
- [x] `Property(&t.X)` identifica corretamente CADA campo de uma struct multi-campo (reproduza a struct `UserEntity` de 7 campos do INSIGHT.md — `Id int64`, `Name string`, `Email string`, `IsActive bool`, `CreatedAt time.Time`, `UpdatedAt time.Time`, `DeletedAt *time.Time` — chame `Property` pra CADA um, confirme via `Field()` que cada `PropertyBuilder` aponta pro `reflect.StructField` CERTO, não trocado com o vizinho)
- [x] `Property` com um ponteiro que NÃO pertence ao tipo passado a `New` panica com mensagem clara
- [x] `Property` chamado DUAS VEZES pro MESMO campo panica com mensagem clara ("already registered")
- [x] `Required()`/`Nullable()`/`Description(s)`/`Examples(...)` armazenam corretamente, cada um retorna o MESMO `*PropertyBuilder` (chain continua), getters (`IsRequired`/`IsNullable`/`DescriptionText`/`ExamplesList`) confirmam o que foi armazenado
- [x] `Metadata.Description(s)`/`DescriptionText()` funciona no nível do tipo inteiro, distinto de qualquer `Description` de campo individual
- [x] `OwnProperties()` devolve cópia defensiva — mutar o slice devolvido não afeta estado interno
- [x] `New` com um `structType` que NÃO é struct (ex: `reflect.TypeOf(42)`) panica com mensagem clara
- [x] Gate check passa
- [x] Test count: 12+ (identificação correta de 7 campos individualmente, ponteiro estranho panica, double-registration panica, round-trip dos 4 métodos comuns com getters, Description de nível-tipo distinto de nível-campo, OwnProperties cópia defensiva, non-struct panica)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add Metadata/PropertyBuilder core with pointer-offset field identification`

---

### T2: Root re-exports

**What**: pacote raiz `gonest` ganha `NewMetadata[T any](fn func(t *T, m *Metadata)) *Metadata` (wrapper genérico real — Go não consegue re-exportar função genérica via `var`, mesmo padrão de `MustInject`/`NewApp`), `type Metadata = metadata.Metadata`, `type PropertyBuilder = metadata.PropertyBuilder`. Per AD-009 (STATE.md), ADICIONE ao `gonest.go`/`gonest_test.go` EXISTENTES — não crie arquivo novo.
**Where**: `gonest.go` (existente, nova seção `// Metadata (Metadata Registration Core feature)`), `gonest_test.go` (existente, testes correspondentes)
**Depends on**: T1
**Reuses**: idioma exato `type X = pkg.X` + wrapper genérico já usado pra `NewApp`/`MustInject` na raiz
**Requirement**: MDR-01 até MDR-05 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.NewMetadata[T](fn)`, `gonest.Metadata`, `gonest.PropertyBuilder` resolvem e funcionam na raiz
- [ ] Reprodução do exemplo `UserEntity` do INSIGHT.md através dos aliases raiz (os 7 campos, `Required`/`Nullable`/`Description`/`Examples` — SEM as chamadas de branch tipo `.Integer()`/`.String()`, que não existem ainda, per Out of Scope) — confirma que cada campo foi registrado corretamente
- [ ] Gate check passa
- [ ] Test count: 3+ (smoke test raiz pros 3 tipos/função resolverem, reprodução do exemplo `UserEntity` completo)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): re-export NewMetadata/Metadata/PropertyBuilder at root`

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
| T1: internal/metadata core | 1 arquivo novo, pacote novo, técnica de reflect+unsafe não-trivial (denso em testes de verificação) | ✅ Granular |
| T2: Root re-exports | Adiciona seção em `gonest.go`/`gonest_test.go` existentes (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP/DI real — reflection puro | unit | unit | ✅ OK |
| T2 | Re-export + reprodução de exemplo | unit | unit | ✅ OK |

Nenhuma violação. **Nota:** `.specs/codebase/TESTING.md`'s Test Coverage Matrix não tem linha pra "metadata/reflection puro" ainda — T1 pode adicionar uma linha pequena como housekeeping.
