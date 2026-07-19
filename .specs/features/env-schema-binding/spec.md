# Env → Schema Binding Specification

## Problem Statement

Com `dotenv-loading` (feature irmã, `.specs/features/dotenv-loading/`) o processo já tem variáveis de
ambiente reais em `os.Environ()` -- mas nada no gonest hoje sabe VALIDAR/POPULAR uma struct de config
tipada (`DatabaseConfig{Host, Port, User, Password}`) a partir dessas variáveis, com o mesmo rigor que
`Parse[T]`/`MustParse[T]` já dão pra REST (`ctx.Params()`/`Query()`/`Body().Json()`). Motivado pelo
`ConfigModule` do NestJS.

## Goals

- [ ] `*Dotenv` (o mesmo singleton de `dotenv-loading`) satisfaz `execution.Parseable` -- `gonest.
      MustParse[DatabaseConfig](gonest.Dotenv(), databaseConfigSchema)` funciona igual a qualquer outro
      `Parse[T]`/`MustParse[T]` já existente
- [ ] Nova tag de struct `env:"NOME_DA_VAR"` (análoga a `param:"..."`/`query:"..."`) mapeia campo →
      variável de ambiente
- [ ] `PropertyBuilder.Default(value any) *PropertyBuilder` novo -- campo ausente da fonte usa o
      default em vez de disparar `Required`

## Out of Scope

| Feature | Reason |
| ------- | ------ |
| `Schema.Validate(instance T)`/`MustValidate(instance T)` (validar struct já construída) | Decisão de brainstorming (`INSIGHT-CONFIG.md`): criaria um SEGUNDO caminho de validação em `internal/schema`, quando `Parse[T]`/`MustParse[T]` contra `Parseable` já resolve o mesmo problema reusando 100% do pipeline existente |
| `required:"true"` como tag de struct | Toda outra branch do framework marca obrigatoriedade via `.Required()` no `PropertyBuilder`, nunca por tag -- `env` segue o mesmo padrão |
| `Default` disponível pra `params`/`query`/`headers`/`form` (só `env` nesta feature) | Decisão de Tasks (`INSIGHT-CONFIG.md`'s "O que fica em aberto"): `Default` pode nascer escopado só pro caminho de `env`, generalizar pros outros é trabalho futuro opcional, não bloqueia esta feature |
| Dotenv loading em si (`Load`/`MustLoad`, parser `.env`) | Feature SEPARADA e PRÉ-REQUISITO, `.specs/features/dotenv-loading/` |

## Design Decisions (herdadas do brainstorming em `INSIGHT-CONFIG.md`)

| # | Decisão |
| - | ------- |
| D1 | `envSource` novo em `internal/validate` (mesmo nível de `paramsSource`/`querySource`/`jsonBodySource`/`headersSource`/`formBodySource`), implementando `execution.Parseable` -- reusa `validateStruct`/`populate` sem tocar nenhum dos dois |
| D2 | `*Dotenv` (não um tipo `Env` novo) é quem implementa `Parseable` -- ver `dotenv-loading/context.md`'s decisão de unificar as 2 capacidades numa instância só |
| D3 | Coerção de tipo (string crua da env var → `int`/`bool`/etc do campo) reusa `coerceParamString` (`internal/validate/params.go`) sem NENHUMA mudança nela -- mesmo problema que `param`/`query`/`headers`/`form` já resolvem |
| D4 | `Default(value)` entra no escopo desta feature (decisão via `AskUserQuestion`) -- sem ele todo campo de config sem env setada vira erro `Required`, o que não é o comportamento real desejado pra config (maioria das vars tem default razoável: porta, host) |
| D5 | `env:"NOME"` é só o NOME da variável (análogo a `param:"user_id"`) -- obrigatoriedade continua via `.Required()` no builder, não por atributo extra na tag |

## User Stories

### P1: Bind de struct simples a partir da env ⭐ MVP

**User Story**: Como dev, quero declarar `DatabaseConfig` com tags `env:"..."` e um `Schema`, e obter
uma instância validada/populada com uma chamada.

**Why P1**: É o valor central da feature -- sem isso não há bind algum, só leitura crua de
`os.Getenv`.

**Acceptance Criteria**:

1. WHEN `gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)` roda E toda env var referenciada
   por `env:"..."` está presente E válida pro tipo do campo THEN SHALL retornar uma instância
   `DatabaseConfig` totalmente populada
2. WHEN um campo `Integer()`/`Boolean()`/etc. lê uma env var (sempre string crua) THEN SHALL coercionar
   pro tipo Go do campo (reusando `coerceParamString`), com o MESMO erro de validação que `param`/
   `query` já produzem se a coerção falhar
3. WHEN `gonest.Parse[T]` (não-Must) é usado THEN SHALL retornar `(T, error)` em vez de panicar, mesmo
   contrato de qualquer outro `Parse[T]` já existente

**Independent Test**: `DatabaseConfig` com 4 campos, todas as 4 env vars setadas com valores válidos,
`MustParse` retorna instância idêntica aos valores esperados.

---

### P1: Campo obrigatório ausente falha, com o mesmo formato de erro de REST ⭐ MVP

**User Story**: Como dev, quero que um campo `.Required()` sem env var setada E sem `Default` falhe de
forma clara, coletando TODAS as violações (não só a primeira).

**Why P1**: Consistência com `MustParams`/`MustQuery`/`MustJsonBody`, que já coletam todas as
violações -- config errado merece o mesmo tratamento.

**Acceptance Criteria**:

1. WHEN um campo `.Required()` não tem `Default` E a env var correspondente não está setada THEN
   `MustParse` SHALL panicar com uma lista de `{field, message}` (mesmo formato de `MustParams`/
   `MustQuery`/`MustJsonBody`)
2. WHEN MÚLTIPLOS campos obrigatórios estão faltando THEN a lista de violações SHALL conter TODOS
   eles, não só o primeiro (collect-all, mesma convenção do resto do framework)

**Independent Test**: `DatabaseConfig` com 2 campos `.Required()` sem `Default`, nenhuma env setada,
`MustParse` panica com lista de exatamente 2 violações.

---

### P1: `Default(value)` cobre campo ausente ⭐ MVP

**User Story**: Como dev, quero declarar um valor padrão pra um campo de config, pra não precisar
setar TODA env var em todo ambiente (dev local sem `.env` completo, por exemplo).

**Why P1**: Sem isso a feature é inutilizável na prática -- toda config real tem pelo menos um campo
com default razoável (porta, host).

**Acceptance Criteria**:

1. WHEN um campo tem `.Default(value)` E a env var correspondente NÃO está setada THEN o campo SHALL
   receber `value`, sem disparar `Required` mesmo se `.Required()` também estiver declarado
2. WHEN a env var ESTÁ setada (mesmo que a env var esteja com valor "errado"/inválido pro tipo) THEN
   `Default` NÃO SHALL ser usado -- o valor real da env var (ou o erro de validação dele) prevalece
3. WHEN um campo NÃO tem `.Required()` NEM `.Default()` E a env var está ausente THEN o campo SHALL
   ficar com o zero-value do seu tipo Go (mesmo comportamento hoje de campos opcionais em REST)

**Independent Test**: campo com `.Default("127.0.0.1")`, env var ausente, `MustParse` retorna
`"127.0.0.1"`; mesmo campo com env var `DB_HOST=other-host` setada, `MustParse` retorna `"other-host"`.

---

## Edge Cases

- WHEN uma env var está setada mas VAZIA (`DB_HOST=`, string vazia real) THEN o comportamento (conta
  como "ausente" pra fins de `Default`, ou como "presente com valor vazio" pra fins de validação de
  tipo) fica em aberto pra Design -- reusar a MESMA semântica que `dotenvx`'s `${VAR:-default}` usa
  pra "vazio conta como ausente" é uma opção natural mas não decidida aqui
- WHEN `env:"..."` está ausente da tag de um campo usado num `Schema` consumido via `gonest.Dotenv()`
  THEN o comportamento (erro de build/registro do Schema, ou o campo simplesmente nunca é lido) fica
  em aberto pra Design
- WHEN o MESMO `Schema`/struct é usado TANTO pra `env-schema-binding` QUANTO pra REST (`param`/`query`/
  `json` na mesma struct, tags coexistindo) THEN não deve haver conflito -- cada `Parseable` só lê a
  tag que lhe compete (`envSource` só lê `env:"..."`, ignora `param:"..."`/`query:"..."` no mesmo
  campo), mesmo padrão que já vale entre `param`/`query`/`json` hoje

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --------------- | ---------------------------------------------------------------- | ------- | ------- |
| ENVCFG-01 | P1: Bind de struct simples a partir da env | Execute | Verified |
| ENVCFG-02 | P1: Campo obrigatório ausente falha (collect-all) | Execute | Verified |
| ENVCFG-03 | P1: `Default(value)` cobre campo ausente | Execute | Verified |

**Coverage:** 3 total, 0 mapped to tasks, 3 unmapped ⚠️ (normal em Specify)

## Success Criteria

- [ ] `gonest.MustParse[T](gonest.Dotenv(), schema)`/`Parse[T]` funcionam pra qualquer struct de config
      com tags `env:"..."`, reusando o pipeline de validação REST sem duplicação
- [ ] `PropertyBuilder.Default(value)` funciona e é coberto por teste de regressão
- [ ] `go test ./... -race` passa após a implementação, zero mudança de comportamento em `param`/
      `query`/`headers`/`form`/`json` (reuso comprovado, não reescrita)
