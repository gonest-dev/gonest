# Schema Value Support Specification

## Problem Statement

Hoje `gonest.NewSchema[T]` exige que `T` seja uma struct: cada campo é
identificado por OFFSET de ponteiro (`&t.Field` medido contra o endereço
base de `t`, via `unsafe.Pointer`+`reflect` -- `internal/schema.New`). Isso
cobre bem `UserEntity`/`UserIdParams`/etc, mas não cobre um caso real e
comum: validar um VALOR ÚNICO, sem sub-campos -- ex: um CPF (`string`)
isolado, sem estar dentro de uma struct maior.

Paralelamente, `gonest.Value[T]` já existe hoje (`internal/value`,
dirty-tracking field wrapper para PATCH-style handlers -- `Get()`/`Set()`/
`IsDirty()`) e ocupa o nome mais natural para o conceito novo acima
("Value" = "isto é o valor raiz sendo descrito"). O nome precisa ficar
livre antes do novo conceito poder usá-lo.

## Goals

- [ ] Renomear `gonest.Value[T]` (dirty-tracking atual) para
      `gonest.Accessor[T]` -- mesma API (`Get()`/`Set()`/`IsDirty()`/
      `OnDirty()`/`Apply()`), só o nome muda
- [ ] Introduzir `gonest.NewValue[T](func(m *gonest.Value) {...})` --
      construtor de `*schema.Schema` para um valor único (string/integer/
      number/boolean/etc), sem exigir `T` ser struct
- [ ] `gonest.Value` (o builder, tipo novo) expõe os MESMOS branches de
      tipo que `PropertyBuilder` já expõe (`String()`, `Integer()`,
      `Boolean()`, etc), reaproveitando a validação existente
      (`validateValue`/`validatePrimitive`) sem duplicar lógica
- [ ] `gonest.Parse[T]`/`gonest.MustParse[T]` (unified-parse-api) continuam
      funcionando sem mudança de assinatura para um schema de `Value`
- [ ] `gonest.Property(&t.X)` (dentro de `NewSchema[T]`, struct) permanece
      EXATAMENTE como está -- nenhuma mudança nesse caminho

## Out of Scope

| Feature                                              | Reason                                                                                     |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `Value` cobrindo Array/Object no nível raiz            | `Array()`/`Object()` já são dual-state builders mais ricos (`Items(fn)`) que `String()`/`Integer()` -- decidir separado, ver INSIGHT-SCHEMA.md's "O que fica em aberto" |
| Registro em `components.schemas` (OpenAPI) para um `Value` nomeado | Ainda não pensado se um `cpfSchema` desse tipo ganha `$ref` reusável ou fica sempre inline -- feature de geração de OpenAPI separada |
| GraphQL Custom Scalars usando `Value`                  | Mencionado como motivação em INSIGHT-GRAPHQL.md/INSIGHT-SCHEMA.md, mas GraphQL em si é feature própria, não implementada |

---

## Design Decisions (tomadas durante o brainstorming)

| #   | Decisão                                                                                                                                                                                  |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| D1  | `Property`/`m.Property(&t.X)` continua só dentro de `NewSchema[T]` (struct) -- não reformar por causa de um caso secundário |
| D2  | Novo conceito usa o nome `Value`/`NewValue[T]`, não `Primitive`/`NewPrimitive` nem `PrimitiveSchema`/`ValueSchema` (nomes compostos descartados por preferência explícita) |
| D3  | `gonest.Value[T]` atual (dirty-tracking) renomeia para `gonest.Accessor[T]` -- termo correto da literatura para "algo com get/set" (o "dirty" é um ESTADO do accessor, exposto via `IsDirty()`, não o nome do tipo -- `Dirty[T]` foi cogitado e descartado por essa razão) |
| D4  | `Scalar` foi cogitado e descartado para o novo conceito -- engessaria a leitura só pro contexto GraphQL, mesmo sendo genérico o bastante pra qualquer transporte |
| D5  | `NewValue[T]` NÃO recebe o parâmetro `t *T` que `NewSchema[T]` tem hoje -- esse `t` só existe para permitir `&t.Field`, e um valor solto não tem campo pra apontar |

---

## Architecture Note

Internamente, o builder `Value` reaproveita a MESMA infraestrutura de
`PropertyBuilder`/`validateValue`/`validatePrimitive` que já valida cada
campo individual hoje -- aplicada a uma property implícita única (offset
0, representando o valor raiz) em vez de iterar `m.OwnProperties()` de uma
struct real. `internal/schema.New` precisa de um segundo modo de
construção (hoje assume sempre struct via `reflect.TypeOf`/offset
scanning) -- ou um novo construtor-irmão que já monta essa property
implícita direto, sem exigir um `T` struct de entrada.

`gonest.Parse[T]`/`MustParse[T]` (unified-parse-api) não mudam: `Parseable.
ParseInto(dst any, schema any)` já recebe `dst` como ponteiro genérico --
para um `Value`, `dst` é `*string`/`*int64`/etc em vez de `*SomeStruct`,
sem exigir mudança de assinatura.

## API Sketch

```go
package ex

import "gonest.dev/gonest"

var cpfSchema = gonest.NewValue[string](func(m *gonest.Value) {
  m.String().Min(11).Max(11).Pattern(`^\d{11}$`)
})

var ageSchema = gonest.NewValue[int64](func(m *gonest.Value) {
  m.Integer().Min(0).Max(130)
})

// Accessor[T] (era Value[T]) -- API idêntica, só renomeado.
type UpdateUserDTO struct {
  Name gonest.Accessor[string]
  Age  gonest.Accessor[int64]
}
```

---

## User Stories

### P1: Renomear `Value[T]` → `Accessor[T]` ⭐ MVP

**User Story**: Como mantenedor do gonest, quero renomear `gonest.Value[T]`
para `gonest.Accessor[T]` (mesma API, novo nome), para liberar o nome
`Value` pro conceito de schema de valor único, sem quebrar o comportamento
de dirty-tracking existente.

**Why P1**: Bloqueante -- `NewValue[T]`/`Value` (P2) não pode existir com
esse nome enquanto `gonest.Value[T]` continuar ocupando o símbolo.

**Acceptance Criteria**:

1. WHEN qualquer código chama `gonest.Accessor[T]` THEN SHALL ter EXATAMENTE o mesmo comportamento que `gonest.Value[T]` tinha (Get/Set/IsDirty/OnDirty/Apply/MarshalJSON/UnmarshalJSON)
2. WHEN o pacote é `go build`ado THEN `gonest.Value[T]` (símbolo antigo) SHALL não existir mais
3. WHEN `internal/value.ToDirtyMap` é chamado THEN SHALL continuar funcionando idêntico, só reconhecendo o novo tipo

**Independent Test**: `go test ./... -race` passa; nenhuma asserção de teste pré-existente muda de expectativa, só nome de tipo referenciado.

---

### P1: `gonest.NewValue[T]`/`gonest.Value` para schema de valor único ⭐ MVP

**User Story**: Como desenvolvedor, quero declarar um `*Schema` para um
valor primitivo isolado (ex: CPF, e-mail solto), sem precisar embrulhá-lo
numa struct só para ter `Property(&t.X)`.

**Why P1**: Núcleo da feature -- sem isso não há caso de uso novo, só o
rename do P1 anterior.

**Acceptance Criteria**:

1. WHEN `gonest.NewValue[string](func(m *gonest.Value) { m.String()... })` é chamado THEN SHALL retornar um `*Schema` válido, sem exigir que `string` seja struct
2. WHEN esse `*Schema` é passado para `gonest.Parse[T]`/`gonest.MustParse[T]` THEN SHALL validar/popular o valor raiz corretamente, reusando `validateValue`/`validatePrimitive` sem duplicar lógica
3. WHEN `m.String()`/`m.Integer()`/`m.Boolean()`/etc é chamado dentro do callback de `NewValue` THEN SHALL aceitar os MESMOS modificadores que `PropertyBuilder` já aceita (`Min`/`Max`/`Pattern`/`Required`/`Nullable`/`Custom(fn)`)
4. WHEN `gonest.Property(&t.X)` é usado dentro de `NewSchema[T]` (struct) THEN SHALL continuar funcionando exatamente como hoje, sem nenhuma mudança de comportamento

**Independent Test**: um `cpfSchema` construído via `NewValue[string]` valida um CPF via `gonest.MustParse[string](someParseable, cpfSchema)` (ou uso direto do schema, a definir no design), rejeitando um valor fora do padrão `Pattern`.

---

## Requirement Traceability

| Requirement ID | Story                                    | Phase   | Status  |
| -------------- | ------------------------------------------ | ------- | ------- |
| SVAL-01        | P1: Rename Value[T] → Accessor[T]           | Specify | Pending |
| SVAL-02        | P1: NewValue[T]/Value construtor            | Specify | Pending |
| SVAL-03        | P1: Value reaproveita PropertyBuilder engine| Specify | Pending |
| SVAL-04        | P1: Property/NewSchema[T] struct inalterado | Specify | Pending |

---

## Success Criteria

- [ ] `go test ./... -race` passa após a migração completa
- [ ] Zero ocorrência de `gonest.Value[T]` (símbolo antigo) fora de `.specs`/STATE.md
- [ ] `gonest.NewValue[T]`/`gonest.Value` existem e validam corretamente um valor primitivo isolado
- [ ] `gonest.Accessor[T]` existe com comportamento idêntico ao `Value[T]` anterior
