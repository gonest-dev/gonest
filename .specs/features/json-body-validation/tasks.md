# JSON Body Validation Tasks

**Design**: `.specs/features/json-body-validation/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T0-T4, todos evaluator PASS)

---

## Execution Plan

```
T0 (PropertyBuilder storage relocation + kind field -- HIGH REGRESSION RISK)
  → T1 (global metadata registry)
  → T2 (Context/Responder body access)
  → T3 (internal/validate recursive core + MustJsonBody)
  → T4 (root re-export + INSIGHT.md end-to-end reproduction)
```

Sequencial estrito. T0 é a única task que toca código de 4 features já fechadas e aprovadas (String-family, Numeric & Boolean, Array Builder, Object Builder) -- roda com seu PRÓPRIO evaluator pass ANTES de qualquer task seguinte começar, mesmo que isso pareça redundante com o padrão normal (AD-001 já exige evaluator por task; aqui a barreira é ainda mais estrita: NADA de T1-T4 começa até T0 estar 100% verde).

---

## Task Breakdown

### T0: Relocar storage de String/Numeric/Array/Object pro `PropertyBuilder` compartilhado + campo `kind` ✅ DONE (evaluator: PASS, commit `d012c7e` -- SPEC_DEVIATION: `ArrayMetadata.item` mantido como snapshot próprio em vez de deletado, verificado são pelo evaluator)

**What**: (ver design.md's Components/Data Models pra detalhe completo)

1. `internal/metadata/metadata.go`'s `PropertyBuilder` struct ganha 8 campos novos: `kind string`, `min, max *int`, `pattern string`, `item *PropertyBuilder`, `itemRef *Metadata`, `ref *Metadata`, `additionalProperties bool`
2. Todo método de branch existente em `metadata.go` (`String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`/`Integer`/`Int32`/`Float`/`Double`/`Boolean`/`DateTime`/`Date`/`Array`/`Object`) ganha UMA linha nova setando `p.kind` (valores exatos: ver design.md's Components table)
3. `PropertyBuilder.Array()` muda de `return &ArrayMetadata{PropertyBuilder: p, item: &PropertyBuilder{}}` pra primeiro fazer `p.item = &PropertyBuilder{}` e DEPOIS `return &ArrayMetadata{PropertyBuilder: p}`
4. `internal/metadata/array.go`'s TODOS os 18 métodos de branch de item (mesma lista acima, mas `am.item.format = X` / `am.item.kind = Y`) ganham a mesma linha `kind`
5. `internal/metadata/string.go`: DELETA os campos `min, max *int; pattern string` do `StringMetadata` struct -- NÃO toca em NENHUM method body (compilam via promoted field access)
6. `internal/metadata/numeric.go`: DELETA os campos `min, max *int` do `NumericMetadata` struct -- idem, sem tocar method bodies
7. `internal/metadata/array.go`: DELETA os campos `item *PropertyBuilder; itemRef *Metadata; min, max *int` do `ArrayMetadata` struct -- idem, sem tocar method bodies (exceto o `Array()` no item 3 acima)
8. `internal/metadata/object.go`: DELETA os campos `ref *Metadata; additionalProperties bool` do `ObjectMetadata` struct -- idem, sem tocar method bodies
9. Adiciona getters NOVOS diretamente em `PropertyBuilder` (`metadata.go`): `MinValue() (int, bool)`, `MaxValue() (int, bool)`, `PatternValue() string`, `ItemBuilder() *PropertyBuilder`, `ItemRef() (*Metadata, bool)`, `MetadataRef() (*Metadata, bool)`, `IsAdditionalProperties() bool`, `KindValue() string` -- MESMA lógica de nil-handling que cada wrapper já tinha (`(0, false)` se nunca setado)

**Where**: `internal/metadata/metadata.go`, `internal/metadata/string.go`, `internal/metadata/numeric.go`, `internal/metadata/array.go`, `internal/metadata/object.go` (todos existentes, SEM arquivo novo)

**Depends on**: nenhuma

**Reuses**: nada quebra -- ver design.md's Tech Decisions pra por que o método promoted-field-access torna isso um diff mínimo

**Requirement**: JV-00

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer -- mas aqui o "RED" inicial é o SUITE EXISTENTE, não testes novos: rode a suite ANTES de mexer em nada, confirme verde, faça as mudanças, confirme verde de novo, DEPOIS escreva testes novos provando que a leitura funciona sem o wrapper original), `verification-before-completion` (evaluator)

**Done when**:
- [x] TODOS os testes EXISTENTES de `internal/metadata` passam SEM NENHUMA MODIFICAÇÃO no código de teste (confirmado por evaluator via `git diff --stat`, zero output)
- [x] TODOS os testes EXISTENTES em `gonest_test.go` que tocam `StringMetadata`/`NumericMetadata`/`ArrayMetadata`/`ObjectMetadata` continuam passando sem modificação
- [x] Novo teste (evaluator escreveu o SEU próprio, independente do `kind_test.go` do dev): item Min/Max sobrevive sem NENHUMA referência de wrapper, lido só via `PropertyBuilder.ItemBuilder()`
- [x] Novo teste: `Boolean()` e `String()` têm `KindValue()` DIFERENTE (`"boolean"` vs `"string"`)
- [x] Novo teste: `KindValue()` correto pra CADA branch (20 casos)
- [x] Gate check passa (evaluator rodou `-race -count=1` fresh, 18 pacotes, 65 testes de `internal/metadata`)
- [x] `gofmt -l` limpo nos arquivos tocados

**Tests**: unit
**Gate**: quick

**Commit**: `refactor(metadata): relocate branch storage onto shared PropertyBuilder, add kind field`

---

### T1: Registro global de metadata ✅ DONE (evaluator: PASS, commits `74c31bc`/`ebe3f2e` -- SPEC_DEVIATION: `Deregister` test-only adicionado + 7 arquivos de teste tocados só pra cleanup, escopo confirmado limpo; débito leve: 4 testes vs "5+" sugerido, `Deregister` sem teste unitário direto)

**What**: cria `internal/metadata/registry.go` (NOVO arquivo, mesmo pacote): `var registry map[reflect.Type]*Metadata` guardado por `sync.RWMutex`; `Register(t reflect.Type, m *Metadata)` (panic em duplicata); `Lookup(t reflect.Type) (*Metadata, bool)`. `internal/metadata.New(structType, baseAddr)` (já existente) ganha uma chamada a `Register(structType, m)` antes de devolver `m`.

**Where**: `internal/metadata/registry.go` (novo), `internal/metadata/metadata.go` (existente, `New` estendido), `internal/metadata/registry_test.go` (novo)

**Depends on**: T0 (evaluator PASS confirmado antes de começar)

**Reuses**: `Metadata` já existente

**Requirement**: JV-01, JV-02

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] `NewMetadata[SomeType]` registra automaticamente, recuperável via `Lookup`
- [x] Chamar `NewMetadata[SomeType]` DUAS vezes pro MESMO tipo panica
- [x] `Lookup` de um tipo nunca registrado devolve `(nil, false)`, sem panic
- [x] Gate check passa (`-race`, 18 pacotes)
- [x] Test count: 4 entregues (abaixo do "5+" sugerido, mas todo cenário coberto -- `Deregister` sem teste unitário direto, débito leve registrado)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(metadata): add global type registry, auto-populated by NewMetadata`

---

### T2: Acesso ao body cru no Context ✅ DONE (evaluator: PASS, commit `5207af2`)

**What**: `internal/execution/context.go`'s `Responder` interface ganha `Body() []byte`; `Context` ganha `func (ctx *Context) Body() []byte { return ctx.res.Body() }`. `internal/adapter/fiber`'s `fiberResponder` ganha `func (r *fiberResponder) Body() []byte { return r.c.Body() }` (sem cópia defensiva -- ver design.md's Tech Decisions pra justificativa, documentar a mesma constraint no doc comment de `Body()`).

**Where**: `internal/execution/context.go` (existente, estendido), `internal/execution/context_test.go` (existente ou novo), `internal/adapter/fiber/fiber.go` (existente, estendido), teste de dispatch real HTTP em `internal/adapter/fiber` ou `internal/app` confirmando bytes reais chegam

**Depends on**: nenhuma (independente de T0/T1, pode rodar em paralelo com elas SE quiser -- mas mantendo sequencial por simplicidade de dispatch nesta sessão)

**Reuses**: `Responder` interface já existente, mesmo padrão de toda outra delegação em `Context`

**Requirement**: JV-03

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] `Context.Body()` devolve exatamente os bytes que o `Responder` fake devolve, em teste isolado
- [x] Dispatch HTTP real (`app.Test`) postando um body JSON confirma que o `fiberResponder.Body()` devolve os bytes REAIS postados
- [x] Gate check passa
- [x] Test count: 3 entregues

**Tests**: unit + integration (dispatch real, per L-012's precedent -- wiring bug só aparece com request de verdade)
**Gate**: quick

**Commit**: `feat(execution): add Context.Body() raw request body access`

---

### T3: `internal/validate` -- núcleo recursivo + `MustJsonBody` ✅ DONE (evaluator: PASS, commits `25ab1e3`/`36924bf` -- collect-all rigorosamente provado; gap de teste de fractional-integer fechado em follow-up)

**What**: (ver design.md's Components/Data Models pra assinatura completa)

Cria pacote NOVO `internal/validate`:
- `type violation struct { Field, Message string }`
- `func MustJsonBody[T any](ctx *execution.Context) T` -- lookup registry (panic se não achado), unmarshal body 2x (`any`/`map[string]any` pra presença+tipo, `T` pro valor final), `validateStruct` recursivo coletando TODAS violações, panic `exception.NewBadRequestException(violations)` se houver alguma, senão devolve o `T` populado
- `validateStruct(presence map[string]any, m *metadata.Metadata, pathPrefix string) []violation` -- itera `m.OwnProperties()`, checa presença (Required), delega pra `validateValue`
- `validateValue(raw any, p *metadata.PropertyBuilder, path string) []violation` -- trata `null` (Nullable), dispatcha por `p.KindValue()` pra `validatePrimitive`/`validateArray`/`validateObject`
- `validatePrimitive` -- confere tipo Go do valor decodificado (`string`/`float64`/`bool`) bate com `kind`, aplica `MinValue`/`MaxValue`/`PatternValue` quando aplicável
- `validateArray` -- confere `raw` é `[]any`, aplica quantidade `MinValue`/`MaxValue` do PRÓPRIO campo, itera cada item aplicando `validateValue` contra `p.ItemBuilder()` (ou recursa em `p.ItemRef()` se setado), path inclui índice (`"tags[2]"`)
- `validateObject` -- se `p.MetadataRef()` setado, recursa `validateStruct` com o `*Metadata` referenciado, path prefixado (`"address."`); se `p.IsAdditionalProperties()`, pula validação estrutural (spec.md's Out of Scope)

**Where**: `internal/validate/validate.go` (novo), `internal/validate/validate_test.go` (novo)

**Depends on**: T0, T1, T2 (todas precisam estar verdes)

**Reuses**: `Metadata.OwnProperties()`, todos os getters de `PropertyBuilder` (velhos + novos de T0), `exception.NewBadRequestException`

**Requirement**: JV-04 até JV-09

**Tools**:
- MCP: NONE
- Skill: `test-driven-development` (developer), `verification-before-completion` (evaluator)

**Done when**:
- [x] Happy path: body válido, devolve `*T` populado, sem panic (JV-04)
- [x] Body com JSON malformado panica `BadRequestException` com 1 violation (JV-05)
- [x] Campo `Required` ausente gera violation mesmo com outros campos válidos (JV-05)
- [x] Campo fora de `Min`/`Max`/`Pattern` gera violation (JV-05)
- [x] 2+ violações simultâneas: `details` contém TODAS -- rigorosamente provado pelo evaluator (não só reivindicado), inclusive via dispatch HTTP real decodificando a resposta (JV-06)
- [x] `Nullable` + `null` aceito mesmo sendo `Required` (JV-07)
- [x] Array com item inválido identifica índice (JV-08)
- [x] Array com quantidade fora do range identifica o CAMPO, não item (JV-08)
- [x] Object com `ref` aninhado inválido identifica path aninhado (`"address.zip"`) (JV-09)
- [x] `AdditionalProperties()` pula validação estrutural (JV-09)
- [x] Tipo nunca registrado panica ANTES de tocar no body (ordem de código confirmada pelo evaluator)
- [x] Gate check passa
- [x] Test count: 19 entregues (18 do dev inicial + 1 de follow-up cobrindo fractional-integer-rejected, gap apontado pelo evaluator e fechado)

**Tests**: unit + integration (pelo menos os casos P3-P5 via dispatch HTTP real, `app.Test` -- mesmo precedente L-012, wiring entre Context/registry/validator só prova via request de verdade)
**Gate**: full

**Commit**: `feat(validate): add recursive MustJsonBody validator`

---

### T4: Root re-export + reprodução INSIGHT.md end-to-end ✅ DONE (evaluator: PASS, commit `a9bbda9`)

**What**: `gonest.go` ganha `func MustJsonBody[T any](ctx *Context) T { return validate.MustJsonBody[T](ctx) }` (wrapper real, Go não reexporta função genérica via `var` -- mesmo padrão de `MustInject`/`MustParam`, AD-004). Teste em `gonest_test.go` reproduzindo `UserEntity`-shaped body (INSIGHT.md) end-to-end via dispatch HTTP real, incluindo pelo menos 1 caso feliz e 1 caso com múltiplas violações (incluindo array+object aninhado).

**Where**: `gonest.go` (existente, estendido), `gonest_test.go` (existente, estendido)

**Depends on**: T3

**Reuses**: `Context`, `NewMetadata`, todo o resto já re-exportado

**Requirement**: JV-04 até JV-09 (superfície pública)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.MustJsonBody[T](ctx)` resolve e funciona idêntico à versão interna (usa `*execution.Context` direto -- `gonest.Context` alias não existe ainda, gap pré-existente confirmado pelo evaluator, mesmo padrão de `MustParam`)
- [x] Reprodução completa do `UserEntity` do INSIGHT.md via dispatch HTTP real, caso feliz E caso com violações combinadas (array item + object aninhado na MESMA request)
- [x] Gate check passa (evaluator rodou suite inteira fresh, feature completa T0-T4 regression-free)
- [x] Test count: 1 teste (2 subtests) entregue

**Tests**: integration (dispatch real)
**Gate**: quick

**Commit**: `feat(validate): re-export MustJsonBody at root`

---

## Parallel Execution Map

```
Sequencial estrito: T0 → T1 → T2 → T3 → T4
```

T2 é tecnicamente independente de T0/T1 (poderia rodar em paralelo), mas por segurança nesta sessão o dispatch continua sequencial -- T0 é grande demais/arriscado demais pra dividir atenção com outra task simultânea.

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`. T0 em particular: evaluator DEVE rodar a suite completa (`-race`, todos os 18 pacotes), não só `internal/metadata`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T0: Storage relocation + kind | 5 arquivos existentes, ALTO RISCO (toca 4 features fechadas), mas mecânico (Go promoted-field access) | ✅ Granular (mas isolado com barreira própria) |
| T1: Registro global | 2 arquivos (1 novo) | ✅ Granular |
| T2: Body access | 2 arquivos existentes | ✅ Granular |
| T3: Validador recursivo | 1 pacote novo, maior escopo (núcleo real da feature) | ✅ Granular (mas maior que o normal -- aceitável, é o coração da feature) |
| T4: Root re-export | Mecânico, mesmo padrão AD-009/AD-004 | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T0 | Refactor interno, reflection puro | unit | unit | ✅ OK |
| T1 | Registro global, reflection puro | unit | unit | ✅ OK |
| T2 | Infra HTTP (Responder/Context) | unit + integration | unit + integration | ✅ OK |
| T3 | Núcleo de validação + dispatch HTTP | unit + integration | unit + integration | ✅ OK |
| T4 | Re-export + reprodução E2E | integration | integration | ✅ OK |

Nenhuma violação.
