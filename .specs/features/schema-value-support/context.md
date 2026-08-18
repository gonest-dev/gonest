# Schema Value Support Context

**Gathered:** 2026-07-17 (brainstorming em conversa, evoluindo o rascunho de `INSIGHT-SCHEMA.md`)
**Spec:** `.specs/features/schema-value-support/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Permitir `gonest.NewSchema`-like para um valor primitivo isolado (sem
struct em volta), e renomear `gonest.Value[T]` (dirty-tracking existente)
para `gonest.Accessor[T]`, liberando o nome `Value` para o conceito novo.

---

## Implementation Decisions

### Por que `Property(&t.X)` não serve pra valor único

`Property` mede a DISTÂNCIA entre `&t.Field` e `t` (o struct inteiro) --
essa distância é o offset usado depois para localizar o campo em
`populate`/`validateStruct`. Um valor primitivo sozinho não tem "campos"
para apontar -- `t` já É o valor inteiro, offset 0, sem sub-estrutura.
Tecnicamente dava pra forçar `Property(&t)` com offset sempre 0, mas o
VOCABULÁRIO da API (`Property`, pensado para "um campo dentre vários")
ficava estranho para "o valor inteiro é ele mesmo".

### Nomes descartados e por quê

- **`PrimitiveSchema`/`ValueSchema`** -- usuário tem preferência explícita
  contra nomes compostos (`Value + Schema`), soam "gambiarra".
- **`NewPrimitive`/`Primitive`** -- primeira escolha alternativa, funcional
  e sem colisão, mas descartada em favor de `Value` (ver abaixo) depois de
  mais uma rodada de reflexão.
- **`NewProperty`/`Property`** (reaproveitando o `PropertyBuilder` já
  existente, sem tipo novo) -- cogitado por reduzir superfície de API
  nova, mas descartado: "Property" carrega a ideia de "parte de um todo"
  (natural pro campo de struct, caso mais comum e já estabelecido); usar
  o MESMO nome pro valor raiz (que não é parte de nada) criaria
  ambiguidade de leitura ("por que às vezes é `s.Property(&t.X)` e às
  vezes só `s.String()` direto?"). Decisão final: `Property` fica
  RESERVADO só para dentro de `NewSchema[T]` (struct) -- D1 no spec.md.
- **`Scalar`** -- cogitado por já ser vocabulário reconhecível de
  GraphQL/JSON Schema, descartado porque engessaria a leitura da API pro
  contexto GraphQL especificamente, mesmo sendo genérico o bastante pra
  qualquer transporte (usuário: "não gosto porque senão engessaria no
  graphql e poderia causar confusão").
- **`Standalone`** -- descritivo mas mais verboso, não escolhido.

### Nome final: `Value`, e o que isso exige do `Value[T]` atual

`Value` foi escolhido como o nome mais coerente para "isto é o valor
raiz sendo descrito" -- mas colide com `gonest.Value[T]`, que já existe
HOJE como código real (`internal/value`, dirty-tracking field wrapper
para PATCH-style handlers, usado via `Get()`/`Set()`/`IsDirty()`/
`OnDirty()`/`Apply()`/`MarshalJSON`/`UnmarshalJSON`/`ToDirtyMap`).

Usuário confirmou estar aberto a renomear o `Value[T]` atual para abrir
espaço. Investigação de nomenclatura ("qual nome certo na literatura pra
algo que tem get/set"):

- **`Dirty[T]`** -- cogitado primeiro (o padrão em si tem nome próprio na
  literatura, "Dirty Flag pattern", Nystrom's *Game Programming
  Patterns*), mas descartado: "dirty" é um ESTADO do objeto (exposto via
  `IsDirty()`), não o que o objeto É -- nomear o TIPO pelo seu estado
  transitório é estranho (usuário: "mas dirty é um estado do acessor
  não").
- **`Accessor[T]`** -- termo correto da literatura de OO para "algo que
  expõe get/set" (accessor methods/accessor pattern). O "dirty" vira só
  um ESTADO que o `Accessor` possui (`IsDirty()`), não o nome do tipo.
  Escolha final, confirmada pelo usuário ("acho que acessor fica
  plausível então").

### `NewValue[T]` não recebe `t *T`

Diferente de `NewSchema[T](func(t *T, s *Schema))`, `NewValue[T]` recebe
só `func(v *Value)` -- o parâmetro `t` em `NewSchema` existe unicamente
para permitir `&t.Field` dentro do callback; um valor primitivo solto não
tem campo nenhum pra apontar (é ele mesmo o valor), então não precisa
desse parâmetro.

---

## Specific References

- `INSIGHT-SCHEMA.md` (repo root) -- rascunho original desta reflexão,
  incluindo as seções "Onde isso se conectaria" (Custom(fn), unified-parse-
  api, GraphQL Custom Scalars) e "O que fica em aberto" (Array/Object no
  nível raiz, registro em `components.schemas`) -- não repetidas aqui,
  ainda válidas como motivação/conexão.
- `INSIGHT-GRAPHQL.md`'s seção "Tipos verdadeiramente customizados" -- menciona
  `GraphqlScalar(name)` sobre `Custom(fn)`; um `Value` nomeado reusável
  entre múltiplas structs é uma conexão futura, fora de escopo aqui (ver
  Out of Scope no spec.md).
- AD-016 em STATE.md -- rename anterior `Metadata`→`Schema`, mesmo
  tratamento de "nome composto descartado" (`Shape` too casual, `Spec`
  colide com a própria convenção `.specs/spec.md` do projeto) -- precedente
  de como este projeto decide nomenclatura por eliminação.

---

## Deferred Ideas

- `Value` cobrindo Array/Object no nível raiz (schema.md's "O que fica em
  aberto") -- decidir separado, já que `Array()`/`Object()` são
  dual-state builders mais ricos que `String()`/`Integer()`.
- Registro de um `Value` nomeado em `components.schemas` do OpenAPI (`$ref`
  reusável) -- não pensado ainda.
- GraphQL Custom Scalars usando `Value` como base -- depende da feature
  GraphQL em si (INSIGHT-GRAPHQL.md), não implementada.
