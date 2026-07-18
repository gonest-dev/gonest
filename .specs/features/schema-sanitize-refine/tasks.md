# Schema Sanitize/Refine — Tasks

**Spec**: `.specs/features/schema-sanitize-refine/spec.md`
**Design**: `.specs/features/schema-sanitize-refine/design.md`
**Status**: Draft

---

## Subagent Roles (ver .specs/project/STATE.md's "Subagent workflow convention")

Feature pequena (Large-ish, mas contida em 1 pacote real + 1 pacote de
integração) -- executada diretamente pelo Planner/Implementer/Evaluator
combinados na mesma sessão (mesmo padrão já usado em
`schema-value-support`), sem necessidade de subagentes separados por
task dado o tamanho contido.

---

## Execution Plan

```
T1 -> T2 -> T3 -> T4 -> T5
```

Sequencial: T2 (Refine) depende de T1 só por ordem de commit, não de
verdade; T3/T4 (integração em internal/validate) dependem de ambos T1/T2
existirem primeiro.

---

## Task Breakdown

### T1: `PropertyBuilder.Sanitize`/`SanitizeFunc`

**What**: Adicionar campo `sanitize func(raw any) any` ao `PropertyBuilder` (`internal/schema/schema.go`), método `Sanitize(fn func(raw any) any) *PropertyBuilder` (bare return, last-call-wins, mesmo padrão de `Custom`) e getter `SanitizeFunc() (func(raw any) any, bool)` (mesmo padrão bool-return de `CustomFunc`).
**Where**: `internal/schema/schema.go`, `internal/schema/schema_test.go`
**Depends on**: None
**Reuses**: `Custom`/`CustomFunc`'s exact shape
**Requirement**: SANR-01

**Done when**:
- [ ] `p.Sanitize(fn)` compila, retorna `p`, `p.SanitizeFunc()` retorna `(fn, true)` depois
- [ ] `p.SanitizeFunc()` retorna `(nil, false)` antes de `Sanitize` ser chamado
- [ ] `go test ./internal/schema/...` passa (suite existente sem regressão + teste novo)

**Tests**: unit
**Gate**: quick (`go test ./internal/schema/...`)
**Commit**: `feat(schema): add Sanitize/SanitizeFunc pre-processing hook`

---

### T2: `Schema.Refine`/`OwnRefines`

**What**: Adicionar campo `refines []func(dst any) (string, error)` ao `Schema`, método `Refine(fn func(dst any) (field string, err error)) *Schema` (append, chain-return) e getter `OwnRefines() []func(dst any) (string, error)` (defensive copy, mesmo padrão de `OwnProperties`).
**Where**: `internal/schema/schema.go`, `internal/schema/schema_test.go`
**Depends on**: T1 (ordem de commit)
**Reuses**: `OwnProperties()`'s defensive-copy shape
**Requirement**: SANR-02

**Done when**:
- [ ] `m.Refine(fn1).Refine(fn2)` acumula 2 entradas em `OwnRefines()`, em ordem de registro
- [ ] `OwnRefines()` retorna cópia (mutar o slice retornado não afeta `m`)
- [ ] `go test ./internal/schema/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/schema/...`)
**Commit**: `feat(schema): add Refine/OwnRefines cross-field post-processing hook`

---

### T3: Aplicar `Sanitize` em `validateValue`/`populate`/`populateValue`

**What**: Ler `internal/validate/validate.go`'s `validateValue` (topo da função), `populate` (loop por campo), `populateValue` (schema-value-support feature) por inteiro primeiro. Em cada um, logo no início (ANTES do check de `Custom`), aplicar: `if fn, ok := p.SanitizeFunc(); ok { raw = fn(raw) }`.
**Where**: `internal/validate/validate.go`, `internal/validate/validate_test.go`
**Depends on**: T1
**Reuses**: `validateValue`/`populate`/`populateValue`'s existing structure, zero linha removida
**Requirement**: SANR-01

**Done when**:
- [ ] Teste prova: `String().Sanitize(trim).Min(11).Max(11).Pattern(...)` aceita `"  12345678901  "` e rejeita `"  123  "` (spec.md's Independent Test, P1 story 1)
- [ ] Teste prova: `Sanitize` + `Custom` juntos -- `Custom` recebe o valor JÁ sanitizado (spec.md's Edge Cases)
- [ ] Suite existente de `internal/validate` passa sem regressão (nenhum teste anterior tocado)
- [ ] `go test ./internal/validate/...` passa

**Tests**: unit
**Gate**: quick (`go test ./internal/validate/...`)
**Commit**: `feat(validate): apply Sanitize before Custom/built-in dispatch (validateValue/populate/populateValue)`

---

### T4: Rodar `Refine` em `jsonBodySource.ParseInto`

**What**: Ler `jsonBodySource.ParseInto` por inteiro primeiro (já parcialmente lido em Design). Logo após `populate(dstVal, presence, m, "json")` retornar `nil` (sucesso) e ANTES do `return nil` final, iterar `m.OwnRefines()`, chamando cada `fn(dstVal.Addr().Interface())`. Coletar toda violação (`field`, `err.Error()`) num slice; se não-vazio, retornar `exception.NewBadRequestException(violations)`.
**Where**: `internal/validate/validate.go`, `internal/validate/validate_test.go`
**Depends on**: T2
**Reuses**: `violation`, `exception.NewBadRequestException`, `dstVal` (já computado na função)
**Requirement**: SANR-02

**Done when**:
- [ ] Teste prova: schema com `Refine` comparando 2 campos aceita payload onde batem, rejeita (violação em `"confirmPassword"`) onde diferem -- mesmo com cada campo individualmente válido (spec.md's Independent Test, P1 story 2)
- [ ] Teste prova: múltiplos `Refine` registrados, mais de um falhando, produzem TODAS as violações (D5)
- [ ] Teste prova: `Refine` nunca roda se `validateStruct` já produziu violação de campo individual (Edge Cases)
- [ ] Suite existente passa sem regressão
- [ ] `go test ./internal/validate/...` passa

**Tests**: unit + integration
**Gate**: quick (`go test ./internal/validate/...`)
**Commit**: `feat(validate): run Refine checks after populate succeeds (jsonBodySource)`

---

### T5: Gate final + STATE.md/ROADMAP.md + INSIGHT-SCHEMA.md

**What**: Rodar suite completa (`go test ./... -race`), confirmar `.examples/*` buildam. Atualizar `STATE.md` (novo AD), `ROADMAP.md` (nova Milestone, COMPLETE), `spec.md`'s traceability (SANR-0x → Verified), `INSIGHT-SCHEMA.md`'s seção "Pré/pós-processamento" substituindo a reflexão especulativa pelo estado REAL implementado (mesmo processo de `unified-parse-api`/`schema-value-support`, scratch build real).
**Where**: raiz, `.specs/project/{STATE,ROADMAP}.md`, `.specs/features/schema-sanitize-refine/spec.md`, `INSIGHT-SCHEMA.md`
**Depends on**: T3, T4

**Done when**:
- [ ] `go test ./... -race` passa, zero falha nova
- [ ] `go build ./...` passa, `.examples/*` buildam
- [ ] `STATE.md` tem novo AD documentando a execução
- [ ] `ROADMAP.md` ganha a Milestone (nova, entre 15 e GraphQL) → COMPLETE
- [ ] `spec.md`'s traceability table → todo SANR-0x → Verified
- [ ] `INSIGHT-SCHEMA.md` reflete a implementação real, confirmado compilando via scratch build

**Tests**: integration (suite completa)
**Gate**: full (`go test ./... -race`)
**Commit**: `chore: finalize schema-sanitize-refine feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: PropertyBuilder.Sanitize | 1 campo + 2 métodos + teste | ✅ |
| T2: Schema.Refine | 1 campo + 2 métodos + teste | ✅ |
| T3: Aplicar Sanitize no pipeline | 3 pontos de integração + testes | ✅ |
| T4: Rodar Refine no pipeline | 1 ponto de integração + testes | ✅ |
| T5: Gate final | verificação + docs | ✅ |
