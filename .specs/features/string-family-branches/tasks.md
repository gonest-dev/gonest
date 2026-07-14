# String-family Branches Tasks

**Design**: `.specs/features/string-family-branches/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (internal/metadata: PropertyBuilder extended + StringMetadata novo) → T2 (root re-exports)
```

Sequencial.

---

## Task Breakdown

### T1: `PropertyBuilder` estendido + `StringMetadata` novo

**What**: estende `internal/metadata/metadata.go`'s `PropertyBuilder` existente (de "Metadata Registration Core", já commitado) com:
- Novo campo não-exportado `format string`
- `func (p *PropertyBuilder) FormatValue() string` — getter
- 10 métodos de branch, cada um seta `p.format` no objeto COMPARTILHADO e devolve `&StringMetadata{PropertyBuilder: p}`: `String()` (`format=""`), `Email()` (`"email"`), `Uuid()` (`"uuid"`), `Uri()` (`"uri"`), `Hostname()` (`"hostname"`), `Ipv4()` (`"ipv4"`), `Ipv6()` (`"ipv6"`), `Password()` (`"password"`), `Byte()` (`"byte"`), `Binary()` (`"binary"`)

Cria arquivo NOVO `internal/metadata/string.go` (MESMO pacote, não pacote novo — ver design.md's Tech Decisions) com:
- `type StringMetadata struct { *PropertyBuilder; min, max *int; pattern string }`
- `func (s *StringMetadata) Min(n int) *StringMetadata` / `func (s *StringMetadata) Max(n int) *StringMetadata` / `func (s *StringMetadata) Pattern(p string) *StringMetadata`
- Getters: `MinValue() (int, bool)`, `MaxValue() (int, bool)`, `PatternValue() string`
- `Required()`/`Nullable()`/`Description(s)`/`Examples(...)` REDECLARADOS em `*StringMetadata` (cada um delega pro `s.PropertyBuilder`'s próprio método, depois `return s`) — NÃO confie na promoção automática de método de embedding (isso devolveria `*PropertyBuilder`, quebrando a chain — ver design.md's Tech Decisions)

**Where**: `internal/metadata/metadata.go` (existente, estendido — só o campo `format`+`FormatValue()`+10 métodos de branch), `internal/metadata/string.go` (novo), `internal/metadata/string_test.go` (novo) ou testes de branch podem ir em `metadata_test.go` também — decisão do developer

**Done when**:
- [ ] Cada um dos 10 métodos de branch devolve `*StringMetadata` com `FormatValue()` correto (confirmado individualmente pros 10, não só uma amostra)
- [ ] `Min`/`Max`/`Pattern` armazenam corretamente, cada um devolve o MESMO `*StringMetadata` (chain continua)
- [ ] `Required`/`Nullable`/`Description`/`Examples` chamados em `*StringMetadata` mutam o `PropertyBuilder` COMPARTILHADO (confirme via `PropertyBuilder`'s próprios getters `IsRequired()`/etc, alcançados através do embedding) — E continuam devolvendo `*StringMetadata` (não `*PropertyBuilder`), provando que `.Min()` continua encadeável DEPOIS de `.Required()`
- [ ] Chain completa reproduzindo INSIGHT.md: `m.Property(&t.Email).Email().Required().Description("Email do usuário").Examples("[EMAIL_ADDRESS]")` e `m.Property(&t.Zip).String().Required().Pattern(...).Description("CEP").Examples("01310-100")` — ambos funcionam ponta a ponta
- [ ] Chamar `.String()` e depois `.Email()` no MESMO `*PropertyBuilder` (branch chamado duas vezes) não panica — `format` reflete a ÚLTIMA chamada (last-write-wins, determinístico)
- [ ] Gate check passa
- [ ] Test count: 15+ (10 branches individualmente + Min/Max/Pattern chain + 4 comuns através de StringMetadata + chain completo estilo INSIGHT.md + double-branch-call não panica)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add StringMetadata with 10 string-family branches (String/Email/Uuid/Uri/Hostname/Ipv4/Ipv6/Password/Byte/Binary)`

---

### T2: Root re-exports

**What**: pacote raiz `gonest` ganha `type StringMetadata = metadata.StringMetadata`. Per AD-009, ADICIONE à seção `// Metadata` já existente em `gonest.go`/`gonest_test.go` — não crie arquivo novo. Nenhuma função nova precisa de wrapper genérico aqui (os 10 métodos de branch já são métodos de `PropertyBuilder`/`StringMetadata`, que já são aliases raiz desde a feature anterior — só falta o alias do TIPO `StringMetadata` em si).
**Where**: `gonest.go` (existente, adiciona à seção Metadata), `gonest_test.go` (existente, testes correspondentes)
**Depends on**: T1
**Reuses**: `Metadata`/`PropertyBuilder`/`NewMetadata[T]` já re-exportados
**Requirement**: STR-01 até STR-04 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.StringMetadata` resolve na raiz
- [ ] Reprodução do exemplo `UserEntity`/`AddressEntity` do INSIGHT.md através dos aliases raiz usando pelo menos `.String()`/`.Email()`/`.Pattern()` (os únicos branches string mostrados explicitamente no INSIGHT.md) — confirma correto
- [ ] Os OUTROS 7 branches (`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`, não mostrados no exemplo do INSIGHT.md) também exercitados através dos aliases raiz pelo menos uma vez cada, confirmando `FormatValue()` correto
- [ ] Gate check passa
- [ ] Test count: 3+ (StringMetadata resolve, reprodução INSIGHT.md, os 7 branches restantes)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): re-export StringMetadata at root`

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
| T1: PropertyBuilder estendido + StringMetadata | 2 arquivos (1 existente estendido, 1 novo), 10 métodos mecânicos + 1 tipo novo | ✅ Granular |
| T2: Root re-exports | Adiciona à seção existente de `gonest.go`/`gonest_test.go` (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP/DI real — reflection puro | unit | unit | ✅ OK |
| T2 | Re-export + reprodução de exemplo | unit | unit | ✅ OK |

Nenhuma violação.
