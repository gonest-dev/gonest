# reflexão: Schema para valores primitivos (sem struct) (Futuro)

Hoje `gonest.NewSchema[T]` exige que `T` seja uma struct: cada campo é
identificado por OFFSET de ponteiro (`unsafe.Pointer` + `reflect`),
medindo a distância entre `&t.Field` e o endereço base de `t` (ver
`internal/schema.New`'s doc comment). Isso funciona perfeitamente para
`UserEntity`/`UserIdParams`/etc, mas não cobre um caso real: e se o dado
que eu quero validar é um valor único, sem sub-campos -- por exemplo um
CPF (`string`), sozinho, sem estar dentro de uma struct?

```go
// Não existe hoje -- Property(&t.X) não faz sentido quando T não tem "X".
cpfSchema := gonest.NewSchema[string](func(t *string, m *gonest.Schema) {
  // ???
})
```

## Por que `Property(&t.X)` não serve aqui

`Property` funciona porque mede a DISTÂNCIA entre o ponteiro do campo
(`&t.Field`) e o ponteiro do struct inteiro (`t`) -- essa distância É o
offset que depois localiza o campo dentro de qualquer instância de `T`
passada para `populate`/`validateStruct`. Um `string` sozinho não tem
"campos" para apontar: `t` já É o valor inteiro, offset 0, sem
sub-estrutura. Tentar encaixar isso em `Property(&t)` é possível
tecnicamente (offset sempre 0), mas o VOCABULÁRIO da API (`Property`,
pensado para "um campo dentre vários") fica estranho para "o valor
inteiro é ele mesmo".

## Proposta: `gonest.NewValue[T]`

Um construtor SEPARADO, sem o parâmetro `t *T` que `NewSchema[T]` tem hoje
(esse `t` só existe para permitir `&t.Field` -- um valor solto não tem
campo nenhum para apontar, então não precisa dele). O callback recebe
diretamente um builder cujos métodos de branch (`String`/`Integer`/
`Boolean`/etc -- os mesmos que `PropertyBuilder` já tem) descrevem o
valor raiz inteiro:

```go
var cpfSchema = gonest.NewValue[string](func(m *gonest.Value) {
  m.String().Min(11).Max(11).Pattern(`^\d{11}$`)
})

var ageSchema = gonest.NewValue[int64](func(m *gonest.Value) {
  m.Integer().Min(0).Max(130)
})
```

Internamente, `Value` reaproveitaria a MESMA infraestrutura de
`PropertyBuilder`/`validateValue`/`validatePrimitive` que já valida cada
campo individual hoje -- só que aplicada a uma única property implícita
(offset 0, representando o valor inteiro) em vez de iterar
`m.OwnProperties()` de uma struct real. Nenhuma lógica de validação nova,
só uma segunda porta de entrada para a MESMA engine.

### Pré-requisito: renomear o `gonest.Value[T]` atual para `gonest.Accessor[T]`

`gonest.Value[T]` já existe HOJE como código real (`internal/value`,
dirty-tracking field wrapper para PATCH-style handlers -- `Get()`/`Set()`/
`IsDirty()`). O nome `Value` precisa ficar livre para o conceito acima, e
"Accessor" é o termo correto da literatura para "algo que tem get/set"
(o "dirty" é um ESTADO do accessor, exposto via `IsDirty()` -- não faz
sentido nomear o TIPO pelo seu estado transitório, `Dirty[T]` foi
cogitado e descartado por esse motivo). Diferente do resto desta
reflexão, ESSE rename não é hipotético -- é uma mudança de código real,
pré-1.0, então de custo baixo agora e crescente depois.

## Onde isso se conectaria

- **`Custom(fn)` ganha um uso mais natural para valor único** -- hoje
  `Custom(fn)` só é chamado dentro de um `Property(&t.X)` de uma struct
  maior; um `Value` com `Custom(fn)` validaria/transformaria um valor
  solto direto (ex: `cpfSchema.Custom(normalizeCpf)`).
- **`gonest.Parse[T]`/`MustParse[T]` (unified-parse-api) continuariam
  funcionando sem mudança** -- `Parseable.ParseInto(dst any, schema any)`
  já recebe `dst` como um ponteiro genérico; para um `Value`, `dst`
  seria `*string`/`*int64`/etc em vez de `*SomeStruct`, sem exigir
  nenhuma mudança na assinatura de `Parseable` em si.
- **`INSIGHT-GRAPHQL.md`'s Custom Scalars/`GraphqlScalar(name)`** ganham
  um caminho mais direto -- um scalar customizado (ex: `ObjectID`) hoje
  precisa de um `Custom(fn)` DENTRO de uma struct maior; com
  `NewValue[ObjectIDString]`, o scalar poderia ter seu PRÓPRIO schema
  nomeado e reusável entre múltiplas structs, em vez de repetir
  `Custom(decodeObjectID).GraphqlScalar("ObjectID")` campo por campo.

## O que fica em aberto

- **`Value` cobre só string/integer/number/boolean/etc, ou também
  array/object no nível raiz?** -- em JSON Schema/OpenAPI, "primitive"
  (o conceito, não o nome) tecnicamente exclui `array`/`object`. Um
  `NewSchema[[]string]` (array no topo, sem struct) provavelmente merece
  pensar separado -- não necessariamente o mesmo `Value`, já que
  `Array()`/`Object()` no `PropertyBuilder` de hoje já tem um shape
  (dual-state builder, `Items(fn)`) bem mais rico que
  `String()`/`Integer()`.
- **Registro/reuso**: um `cpfSchema` desse tipo teria identidade própria
  o suficiente para aparecer em `components.schemas` do OpenAPI (como um
  schema nomeado reusável), ou seria sempre inline no campo que o
  referencia? Ainda não pensado.

Sem decisão tomada aqui -- fica registrado como reflexão para debater via
`superpowers:brainstorming` quando a feature entrar em pauta de verdade.
