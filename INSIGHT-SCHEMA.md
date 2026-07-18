# Schema para valores primitivos (sem struct) -- implementado

`gonest.NewSchema[T]` continua exigindo que `T` seja uma struct (cada campo
identificado por OFFSET de ponteiro -- `unsafe.Pointer`+`reflect`, ver
`internal/schema.New`'s doc comment), **inalterado**. Para um valor único
sem sub-campos (ex: um CPF `string`, sozinho, sem struct em volta), existe
agora um construtor PARALELO: `gonest.NewValue[T]`.

```go
cpfSchema := gonest.NewValue[string](func(m *gonest.Value) {
  m.String().Min(11).Max(11).Pattern(`^\d{11}$`).Required()
})

ageSchema := gonest.NewValue[int64](func(m *gonest.Value) {
  m.Integer().Min(0).Max(130)
})

// gonest.Parse[T]/MustParse[T] (unified-parse-api) funcionam sem
// nenhuma mudança de assinatura -- dst é *string/*int64 em vez de *struct.
cpf := gonest.MustParse[string](req.Body(), cpfSchema)
```

`gonest.Value` é um alias direto de `PropertyBuilder`
(`internal/schema.Value = internal/schema.PropertyBuilder`) -- reaproveita
100% dos branches de tipo+formato já existentes (`String`/`Integer`/
`Boolean`/`Array`/`Object`, `Min`/`Max`/`Pattern`/`Custom`), zero lógica de
validação nova.

## Pré-requisito cumprido: `gonest.Value[T]` (dirty-tracking) → `gonest.Accessor[T]`

O nome `Value` estava ocupado pelo wrapper dirty-tracking de PATCH-style
handlers (`Get()`/`Set()`/`IsDirty()`). Renomeado por inteiro para
`gonest.Accessor[T]` -- termo correto da literatura para "algo com
get/set" ("dirty" é um ESTADO do accessor, não o nome do tipo).
`gonest.NewValue`/`gonest.ValueToDirtyMap` (construtor/helper do dirty-
tracking) também renomearam junto, para `gonest.NewAccessor`/
`gonest.AccessorsToDirtyMap` -- liberando `NewValue` por inteiro para o
construtor de schema acima:

```go
type UpdateUserDTO struct {
  Name  gonest.Accessor[string] `json:"name"`
  Email gonest.Accessor[string] `json:"email"`
}

body := gonest.MustParse[*UpdateUserDTO](req, updateUserSchema)
body.Name.OnDirty(func(name string) { user.Name = name })
changes := gonest.AccessorsToDirtyMap(body)
```

## Como foi implementado

- `internal/schema.NewValue(valueType reflect.Type) (*Schema, *PropertyBuilder)`
  -- constrói um `*Schema` com exatamente 1 `PropertyBuilder` implícito
  (offset 0), registrado no MESMO registry process-wide que `New` usa
  (`Register`) -- `MustParse[T]`'s `resolveSchema` (`internal/validate`)
  segue funcionando sem mudança, já que a comparação é por `reflect.Type`,
  agnóstica a Kind.
- `Schema.IsValue()`/`Schema.ValueProperty()` -- dois métodos novos em
  `internal/schema/schema.go`, nenhuma linha da struct-shaped path
  (`New`/`Property`) tocada.
- **SPEC_DEVIATION real, não antecipada no design original**: o caminho
  existente de validação/populate (`validateStruct`/`populate`, em
  `internal/validate/validate.go`) assume um STRUCT ao redor do valor --
  lê `p.Field()`'s struct tag (`json:"..."`) e usa `dest.FieldByIndex(...)`
  para escrever o campo. Um Value-schema não tem struct nem tag: o corpo
  JSON decodificado inteiro JÁ É o valor. Resolvido com um caminho
  PARALELO (`populateValue`, roteado via `Schema.IsValue()`), reaproveitando
  os MESMOS primitivos por-valor que `validateStruct`/`populate` já chamam
  por campo (`validateValue`/`setField`), só aplicados directly ao valor
  inteiro em vez de um campo dele.

## Onde isso se conecta

- **`Custom(fn)` ganha um uso mais natural para valor único** -- antes só
  fazia sentido dentro de um `Property(&t.X)` de uma struct maior; agora
  `cpfSchema.Custom(normalizeCpf)` valida/transforma o valor solto direto.
- **`INSIGHT-GRAPHQL.md`'s Custom Scalars/`GraphqlScalar(name)`** ganham um
  caminho mais direto -- um scalar customizado (ex: `ObjectID`) pode ter
  seu PRÓPRIO `NewValue[ObjectIDString]` nomeado e reusável entre múltiplas
  structs, em vez de repetir `Custom(decodeObjectID).GraphqlScalar(...)`
  campo por campo (ainda não conectado de fato -- Milestone 16 continua
  fora de escopo desta feature).

## O que continua em aberto (não resolvido nesta feature)

- **`Value` cobre só string/integer/number/boolean/etc, ou também
  array/object no nível raiz?** -- `internal/schema.NewValue` aceita
  qualquer `reflect.Type` sem checar Kind, e `Value` (alias de
  `PropertyBuilder`) já expõe `.Array()`/`.Object(fn)` -- tecnicamente
  chamável, mas NENHUM teste desta feature cobre esse caso (só
  string/integer, spec.md's Out of Scope). Continua genuinamente aberto.
- **Registro/reuso em `components.schemas`**: um `cpfSchema` desse tipo
  ainda não tem identidade própria no gerador OpenAPI (`internal/openapi`)
  -- não pensado nesta feature.
