# Env → Schema Binding Context

**Gathered:** 2026-07-18/19 (brainstorming em conversa, evoluindo `INSIGHT-CONFIG.md`, mesma sessão de
`dotenv-loading`)
**Spec:** `.specs/features/env-schema-binding/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Fechar o `ConfigModule`-like do gonest: validar/popular uma struct de config tipada a partir de env
vars já presentes no processo (carregadas ou não via `dotenv-loading`), reusando `Schema`/
`PropertyBuilder`/`Parse[T]`/`MustParse[T]` que REST já usa -- Uma Única Fonte de Verdade, mesmo
princípio de toda feature anterior deste framework (`GraphQL Support`, `OpenAPI Generation`, etc.).

## Implementation Decisions

### Rejeitado: `Schema.Validate(instance)`/`MustValidate(instance)`

Rascunho original de `INSIGHT-CONFIG.md` (antes desta sessão de brainstorming) tinha
`databaseConfigSchema.Validate(instance)` -- validar uma struct Go JÁ CONSTRUÍDA. Pesquisa confirmou
que isso não existe hoje: `*Schema` só é consumido via `Parse[T](src Parseable, s *Schema) (T, error)`/
`MustParse[T]`, que decodifica uma FONTE CRUA (não uma instância já populada) via `validateStruct`+
`populate` (`internal/validate/validate.go`). Criar `Validate(instance)` abriria um SEGUNDO caminho de
validação inteiro em `internal/schema`, com semântica diferente (valida instância populada vs. valida
fonte crua) -- rejeitado. Decisão: um `envSource` novo, MESMO nível que `paramsSource`/`querySource`/
etc. em `internal/validate`, implementando `execution.Parseable` -- zero API nova em `Schema`.

### `*Dotenv` implementa `Parseable`, não um tipo `Env` separado

Ver `dotenv-loading/context.md`'s decisão -- `gonest.Dotenv()` (mesmo singleton da feature irmã)
ganha `ParseInto(dst any, schema any) error`, satisfazendo `execution.Parseable`
(`internal/execution/request.go:43-45`, interface de UM método só). Usuário notou que ter `Dotenv`
(carregando) e `Env` (lendo) como dois conceitos pra descrever o MESMO espaço de variáveis era
redundante.

### `Default(value)` -- decisão via `AskUserQuestion`: entra no escopo

Confirmado explicitamente com o usuário (pergunta binária: "entra no escopo desta feature de
env-schema-binding?"). Resposta: sim, recomendado -- "faz sentido central pra config -- maioria das
vars de ambiente tem default razoável". `Default` fica ESCOPADO só pro caminho de `env` nesta feature
(não generalizado pra `param`/`query`/`headers`/`form` de cara) -- outra decisão confirmada via
`AskUserQuestion` na sessão anterior (`INSIGHT-CONFIG.md`'s nota "`Default` nasce só pra `envSource` ou
geral").

### `required:"true"` como tag -- rejeitado, decisão via `AskUserQuestion`

Pergunta feita: "pra env, qual seguir -- `.Required()` no builder ou `env:"X,required"` numa tag só?".
Resposta do usuário: `.Required()` no builder, consistente com toda outra branch do framework
(`String`/`Integer`/`Boolean`/etc. já fazem assim). `required:"true"` (a forma do rascunho original)
teria sido a ÚNICA exceção do framework inteiro.

### Coerção de tipo: reuso confirmado, zero mudança em `coerceParamString`

Pesquisa confirmou `coerceParamString(raw string, kind string) (any, error)`
(`internal/validate/params.go:132`) já é compartilhada por `params.go`/`query.go`/`headers.go`/
`form.go` -- todas fontes que recebem string crua e precisam converter pro tipo do campo. Env var
também é sempre string crua (`os.Getenv` retorna `string`), então `envSource` reusa a MESMA função sem
qualquer alteração nela.

## Specific References

- `internal/validate/params.go:132` -- `coerceParamString`, reuso confirmado
- `internal/validate/validate.go` -- `validateStruct`/`populate`, pipeline reusado sem mudança
- `internal/execution/request.go:43-45` -- `Parseable` interface (`ParseInto(dst any, schema any)
  error`), único método que `*Dotenv` precisa satisfazer
- `internal/resolver/stage3.go`'s `callConstructor` -- precedente de `Provider.Constructor` que
  panica/falha, propagado via `errgroup`+`context.Context` cancel, sem derrubar o processo; um
  `Constructor` chamando `gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)` segue o MESMO
  caminho sem nenhuma mudança em `internal/provider`/`internal/resolver`
- `.specs/features/provider-di-graph/design.md` -- tabela "Error Handling Strategy" confirma o
  comportamento acima
- `.specs/features/dotenv-loading/` -- feature PRÉ-REQUISITO, mesma sessão de brainstorming

## Deferred Ideas

- `Default` generalizado pra `param`/`query`/`headers`/`form` (não só `env`) -- feature futura
  separada, se surgir caso de uso real fora de config
- Semântica de env var setada-mas-vazia (`DB_HOST=`) pra fins de `Default` -- deixado em aberto pra
  Design (spec.md's Edge Cases)
- Erro de build/registro quando um `Schema` usado por `env-schema-binding` tem campo sem tag `env:"..."`
  -- deixado em aberto pra Design
