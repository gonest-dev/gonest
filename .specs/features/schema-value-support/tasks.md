# Schema Value Support — Tasks

**Spec**: `.specs/features/schema-value-support/spec.md`
**Status**: Complete

---

## Subagent Roles (ver .specs/project/STATE.md's "Subagent workflow convention")

Mesmo padrão de 3 papéis já validado na feature `request-response-split`:

- **Planner** — já rodou (esta sessão) pra produzir este `tasks.md`.
- **Implementer** — 1 subagente por task abaixo (ou por grupo `[P]` em
  paralelo). Recebe SÓ a definição da task, não as outras tasks nem o
  histórico da conversa.
- **Evaluator** — roda depois de CADA Implementer, antes da task virar
  `completed`. Roda o `Gate` de verdade (não confia só no relatório do
  Implementer), confere `Done when` item a item. Aprova ou devolve com
  motivo específico — nunca corrige código ele mesmo.

Todo prompt de Implementer deve incluir: a task inteira (What/Where/
Depends on/Reuses/Done when/Tests/Gate/Commit), o trecho relevante de
`design.md`/`spec.md` referenciado, e — como esta feature mexe em código
Go real já existente (`internal/value`) — os trechos reais lidos de
`internal/schema/schema.go`/`internal/value/value.go` (não deixar o
Implementer adivinhar a shape atual, ele deve ler o arquivo real primeiro).

---

## Execution Plan

### Phase 1: Rename Value[T] → Accessor[T] (Sequential — bloqueante)

```
T1 -> T2
```

### Phase 2: `internal/schema`'s Value builder (Sequential, depende de T2 só pelo nome livre)

```
T2 -> T3 -> T4
```

### Phase 3: Integração + validação (Sequential)

```
T4 -> T5 -> T6
```

### Phase 4: Gate final

```
T6 -> T7
```

---

## Task Breakdown

### T1: Renomear `internal/value.Value[T]` → `Accessor[T]`

**What**: Ler `internal/value/value.go` por inteiro primeiro. Renomear o tipo `Value[T]` para `Accessor[T]` em todo o arquivo (struct, `New[T]`, `Get`/`Set`/`IsDirty`/`OnDirty`/`Apply`/`GetAny`/`MarshalJSON`/`UnmarshalJSON`, e a interface interna `valueable` se fizer sentido renomear também para `accessible` — avaliar ao ler o código real). `ToDirtyMap`/`fieldKey` (funções livres, não o tipo) só precisam continuar reconhecendo o tipo renomeado via type-assertion. Doc comments do arquivo (topo do pacote, cada método) atualizados para dizer "Accessor" em vez de "Value" onde descrevem O TIPO — mas cuidado para não confundir com o parâmetro `value T`/variável local `val` que já existem no código com nome genérico, esses continuam como estão.
**Where**: `internal/value/value.go`, `internal/value/value_test.go`
**Depends on**: None
**Reuses**: Toda a implementação existente, 100% mecânico — nenhuma linha de LÓGICA muda, só o identificador do tipo
**Requirement**: SVAL-01

**Done when**:
- [ ] `grep -rn "type Value\[T" internal/value/` retorna vazio
- [ ] `type Accessor[T any] struct` existe com os mesmos 2 campos (`dirty`, `value`) que `Value[T]` tinha
- [ ] Todo método (`Get`/`Set`/`IsDirty`/`OnDirty`/`Apply`/`GetAny`/`MarshalJSON`/`UnmarshalJSON`) existe em `*Accessor[T]` com corpo IDÊNTICO ao que `*Value[T]` tinha
- [ ] `internal/value/value_test.go` migrado (todo `Value[T]`/`value.New[T]` vira `Accessor[T]`/`value.New[T]` — `New` em si NÃO muda de nome, só o tipo que ela constrói)
- [ ] `go test ./internal/value/...` passa

**Tests**: unit — suite existente, migrada
**Gate**: quick (`go test ./internal/value/...`)
**Commit**: `refactor(value)!: rename Value[T] to Accessor[T]`

---

### T2: Atualizar `gonest.go`'s re-export de `Value[T]`

**What**: Ler a seção `// Value (dirty-tracking field wrapper)` em `gonest.go` primeiro (contém o alias público + `gonest.MustNewValue`/equivalente, se existir — confirmar lendo o arquivo real, não assumir). Atualizar o alias público de `gonest.Value[T] = value.Value[T]` para `gonest.Accessor[T] = value.Accessor[T]`. Atualizar `ToDirtyMap`'s doc comment/re-export se ele mencionar "Value" no texto. Migrar `gonest_test.go` e qualquer `.examples/*` que referencie `gonest.Value[` (grep confirmou: nenhum uso em `.examples/*` hoje, só em `gonest.go`/`gonest_test.go` — checar `gonest_test.go` de qualquer forma antes de considerar terminado).
**Where**: `gonest.go`, `gonest_test.go`
**Depends on**: T1
**Reuses**: Padrão de type alias já usado para `Schema`/`Parseable`/etc
**Requirement**: SVAL-01

**Done when**:
- [ ] `grep -n "gonest\.Value\[" gonest.go gonest_test.go` retorna vazio (fora de comentário histórico, se houver)
- [ ] `type Accessor[T any] = value.Accessor[T]` (ou wrapper equivalente se `Value[T]` não era um alias direto — confirmar ao ler) existe em `gonest.go`
- [ ] `go build ./... && go test . ` passa

**Tests**: unit — `gonest_test.go` migrado
**Gate**: quick (`go build ./... && go test .`)
**Commit**: `refactor(gonest)!: re-export Accessor[T] (was Value[T])`

---

### T3: `internal/schema` ganha o builder de valor único

**What**: Ler `internal/schema/schema.go` por inteiro primeiro (`New`, `Schema` struct, `Property`, `PropertyBuilder` struct — já parcialmente lido em design.md, mas o Implementer deve ler de novo, o arquivo pode ter mudado). Decidir (e documentar a decisão tomada, já que design.md deixou em aberto) se o tipo `Value` é um alias direto de `PropertyBuilder` (`type Value = PropertyBuilder`) ou um tipo novo que embeda `*PropertyBuilder`. Adicionar um construtor que produz um `*Schema` com exatamente 1 `PropertyBuilder` implícito representando o valor raiz (offset 0), SEM passar pela validação `structType.Kind() != reflect.Struct` que `New` já tem (essa validação continua intacta para `New`, o valor único usa um caminho PARALELO, não uma exceção dentro do mesmo `New`).
**Where**: `internal/schema/value.go` (novo arquivo)
**Depends on**: T2 (nome `Value` só fica livre depois do rename)
**Reuses**: `PropertyBuilder` (branches String/Integer/Boolean/Array/Object, Min/Max/Pattern/Custom) — reuso total, zero lógica de validação nova
**Requirement**: SVAL-02, SVAL-03

**Done when**:
- [ ] Novo construtor interno (nome exato a critério do Implementer, documentado no commit) produz um `*Schema` válido para um `reflect.Type` não-struct
- [ ] `Schema.Property`/`New` (caminho de struct) permanecem BYTE-A-BYTE inalterados — nenhuma linha de `schema.go` editada além de, no máximo, expor um campo/método novo se estritamente necessário (ex: um getter que o `Value` builder precise)
- [ ] `go test ./internal/schema/...` passa (suite existente, zero regressão) + testes novos para o construtor de valor único cobrindo pelo menos: `String().Min().Max().Pattern()`, `Integer().Min().Max()`, `Required()`

**Tests**: unit — novo `internal/schema/value_test.go`
**Gate**: quick (`go test ./internal/schema/...`)
**Commit**: `feat(schema): add value-only schema construction (no struct required)`

---

### T4: `gonest.NewValue[T]`/`gonest.Value` público

**What**: Wrapper genérico em `gonest.go` (Go não re-exporta função genérica via `var`, AD-004) espelhando `NewSchema[T]`, mas sem o parâmetro `t *T` (ver design.md's Components). `type Value = schema.Value` (ou o nome real escolhido em T3) alias público.
**Where**: `gonest.go`, seção `// Schema`
**Depends on**: T3
**Reuses**: Padrão de wrapper de `NewSchema[T]` já existente
**Requirement**: SVAL-02

**Done when**:
- [ ] `gonest.NewValue[T any](fn func(m *gonest.Value)) *gonest.Schema` existe e compila
- [ ] `gonest.NewValue[string](func(m *gonest.Value) { m.String().Min(11).Max(11) })` funciona (exemplo do spec.md's API Sketch)
- [ ] `go build ./...` passa

**Tests**: unit — `gonest_test.go`, reproduzindo o exemplo `cpfSchema` do spec.md
**Gate**: quick (`go test . -run TestNewValue`)
**Commit**: `feat(gonest): expose NewValue[T]/Value for standalone value schemas`

---

### T5: Confirmar `gonest.Parse[T]`/`MustParse[T]` funcionam com um `Value`-schema

**What**: Prova de integração — construir um `Parseable` de teste (fake, mesmo padrão de `internal/validate`'s `newCtx`/testhelpers) e chamar `gonest.MustParse[string](fakeParseable, cpfSchema)`, confirmando que `resolveSchema`'s `m.StructType() != structType` check continua funcionando quando `StructType()` é um tipo primitivo (`reflect.TypeOf("")`) em vez de struct — `reflect.Type` comparison já deveria funcionar para qualquer Kind, mas PROVAR isso com um teste real em vez de assumir.
**Where**: `internal/validate/validate_test.go` (ou novo arquivo, a critério do Implementer) + `gonest_test.go`
**Depends on**: T4
**Reuses**: `Parseable`/`Parse[T]`/`MustParse[T]` (unified-parse-api) sem NENHUMA mudança de código — só prova que já funciona
**Requirement**: SVAL-02

**Done when**:
- [ ] Um teste real prova `gonest.MustParse[string](someParseable, cpfSchema)` valida corretamente (aceita CPF válido, rejeita fora do `Pattern`)
- [ ] Um teste real prova `resolveSchema`'s mismatch panic ainda dispara corretamente se o schema passado foi construído para um tipo primitivo DIFERENTE (ex: `int64` vs `string`)
- [ ] Se algum ajuste mínimo em `internal/validate` for necessário para esses testes passarem, documentar no commit exatamente o que mudou e por quê (SPEC_DEVIATION se divergir do design.md)

**Tests**: integration
**Gate**: quick (`go test ./internal/validate/... .`)
**Commit**: `test: verify gonest.Parse[T]/MustParse[T] work against value-only schemas`

---

### T6: Migrar `.examples/*` (se necessário) + `INSIGHT-SCHEMA.md`

**What**: Grep `.examples/*` por `gonest.Value[` (confirmado vazio nesta sessão de planejamento, mas re-confirmar — pode ter mudado). Atualizar `INSIGHT-SCHEMA.md` (repo root) substituindo a reflexão especulativa pelo estado REAL implementado (ex: remover "fica em aberto" para itens que a implementação real já resolveu, manter só o que genuinamente continua em aberto — Array/Object no nível raiz, registro em `components.schemas`).
**Where**: `.examples/*` (se aplicável), `INSIGHT-SCHEMA.md`
**Depends on**: T5
**Requirement**: (documentação, sem REQRES próprio)

**Done when**:
- [ ] `grep -rln "gonest\.Value\[" .examples --include=*.go` retorna vazio
- [ ] `INSIGHT-SCHEMA.md` reflete a implementação real (nomes finais confirmados, exemplo de código compilando de verdade — mesmo processo de verificação da feature `unified-parse-api`/`request-response-split`, scratch build real, não só revisão visual)

**Tests**: nenhum (documentação + exemplos)
**Gate**: manual (build real do snippet, `go build ./...` nos `.examples/*` se algum foi tocado)
**Commit**: `docs: update INSIGHT-SCHEMA.md to reflect implemented Value/Accessor API`

---

### T7: Gate final + STATE.md/ROADMAP.md

**What**: Rodar suite completa, confirmar zero símbolo antigo (`gonest.Value[T]` fora de histórico), atualizar `STATE.md` com o AD final (decisões tomadas DURANTE a execução, se houver SPEC_DEVIATION) e `ROADMAP.md`'s Milestone correspondente para COMPLETE.
**Where**: raiz, `.specs/project/{STATE,ROADMAP}.md`, `.specs/features/schema-value-support/spec.md`
**Depends on**: T6

**Done when**:
- [ ] `go test ./... -race` passa — 23 pacotes (ou mais, se T3 adicionou um novo _test.go que conta separado — confirmar contagem real), sem falha nova
- [ ] `go build ./...` passa, `.examples/*` buildam
- [ ] `STATE.md` tem novo AD documentando a execução
- [ ] `ROADMAP.md`'s Milestone → COMPLETE
- [ ] `spec.md`'s traceability table → todo SVAL-0x → Verified

**Tests**: integration (suite completa)
**Gate**: full (`go test ./... -race`)
**Commit**: `chore: finalize schema-value-support feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Rename Value[T]→Accessor[T] | 1 arquivo + 1 teste, mecânico | ✅ |
| T2: gonest.go re-export | 1 alias + call sites | ✅ |
| T3: schema.Value builder | 1 arquivo novo + testes | ✅ |
| T4: gonest.NewValue[T] público | 1 wrapper + 1 alias | ✅ |
| T5: Integração com Parse[T] | testes de integração | ✅ |
| T6: Migrar examples + INSIGHT doc | mecânico + doc | ✅ |
| T7: Gate final | verificação + docs | ✅ |
