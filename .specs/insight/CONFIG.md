# INSIGHT-CONFIG — Env loading + Schema binding (rascunho evoluído via brainstorm)

Duas features SEPARADAS (specs/design/tasks distintos quando formalizadas), mas UMA instância só: o
mesmo `*Dotenv` (retornado por `gonest.Dotenv()`, singleton — nunca via DI/`MustInject`, porque precisa
funcionar em `main()` ANTES de qualquer bootstrap/`NewApp` existir) acumula as duas capacidades, uma por
feature:

1. **Dotenv loading** — carregar `.env`/`.env.local` pro processo (`os.Setenv`), inspirado em
   `C:\dev\leandroluk\gox\env`. `Load`/`MustLoad`.
2. **Env → Schema binding** — validar/popular uma struct de config (`DatabaseConfig`, etc.) a partir de
   variáveis de ambiente já presentes no processo, reaproveitando o MESMO `Schema`/`PropertyBuilder`/
   `Parse[T]`/`MustParse[T]` que REST já usa. `*Dotenv` implementa `execution.Parseable` (interface de
   UM método, `ParseInto(dst any, schema any) error`) — não precisa de um tipo `Env` separado, `Dotenv()`
   já é a fonte.

Decisão desta sessão (correção do rascunho anterior, que tinha `gonest.Env()` como um segundo tipo):
usuário notou que ter DUAS "instâncias"/conceitos (`Dotenv` carregando, `Env` lendo) pra descrever o
MESMO espaço de variáveis (`os.Environ()`) é redundante — `Parseable` já é pequeno o bastante
(`ParseInto`) pra `*Dotenv` satisfazer os dois papéis sem custo. `gonest.Env()` sai do design.

Motivado pelo `ConfigModule` do NestJS + um modelo próprio já existente do usuário (`gox/env`:
`env.Load(path)` + `env.Get[T](key, default...)` genérico, sem schema/struct binding).

---

## 1. Dotenv loading

```go
package ex

import "gonest.dev/gonest"

func main() {
  // múltiplos paths -- primeiro que existir "vence" (ou todos se mergeable, decisão de Tasks)
  err := gonest.Dotenv().Load("./.env", "./.env.local")
  if err != nil {
    panic(err) // ou ignorar e seguir com o que já está no os.Environ
  }

  // ou aceitar panicar direto se não achar
  gonest.Dotenv().MustLoad("./.env", "./.env.local")
}
```

`gox/env`'s parser foi construído mirando o comportamento do [`@dotenvx/dotenv`](https://github.com/dotenvx/dotenv)
(interpolação `${VAR}`, comentários `#`, multiline, etc.) — referência real a seguir quando esta
feature for pra Design, não reinventar formato do zero. Escopo em aberto (fica pra Design/Tasks quando
esta feature avançar sozinha): quanto do comportamento do `dotenvx/dotenv` entra na v1 (interpolação e
comentários certamente; expansão de comando/criptografia do `dotenvx` propriamente dito, se existir,
provavelmente fica de fora — v1 é só arquivo texto local) — aqui só precisa popular `os.Environ()`,
tipagem forte fica por conta da feature 2 (Schema binding). Comportamento de múltiplos paths (primeiro
existente vence vs. merge de todos) também não decidido ainda.

---

## 2. Env → Schema binding

### Decisão central (brainstorm desta sessão): reusar `Parseable`/`Parse[T]`, NÃO criar `Schema.Validate(instance)`

O rascunho original desta doc tinha `databaseConfigSchema.Validate(instance)`/`MustValidate(instance)`
— validar uma struct **já construída na mão**. Pesquisa confirmou: isso **não existe** hoje e seria um
caminho de API genuinamente NOVO em `internal/schema` — `*Schema` hoje só é consumido de UM jeito,
`Parse[T](src Parseable, s *Schema) (T, error)`/`MustParse[T]`, que decodifica uma fonte crua
(`map[string]any`-ish) via `validateStruct`+`populate` (`internal/validate/validate.go`). Validar uma
INSTÂNCIA já populada exigiria reescrever a entrada desses dois (aceitar `reflect.Value` de uma
instância em vez de um mapa), ou converter a instância pra mapa primeiro — dois caminhos de validação
diferentes convivendo, quando o framework inteiro (todo `INSIGHT-*.md`) reforça "Uma Única Fonte de
Verdade".

**Decisão:** um `envSource` novo em `internal/validate` (mesmo nível de `paramsSource`/`querySource`/
`jsonBodySource`/`headersSource`/`formBodySource`), implementando `execution.Parseable` — reusa
`validateStruct`/`populate` SEM tocar nenhum dos dois. `Schema` ganha ZERO API nova. Uso fica idêntico
a qualquer outro `Parse[T]`:

```go
package ex

type DatabaseConfig struct {
  Host     string `env:"DB_HOST"`
  Port     int    `env:"DB_PORT"`
  User     string `env:"DB_USER"`
  Password string `env:"DB_PASSWORD"`
}

var databaseConfigSchema = gonest.NewSchema(func(t *DatabaseConfig, s *gonest.Schema) {
  s.Property(&t.Host).String().Required().Default("127.0.0.1")
  s.Property(&t.Port).Integer().Required().Default(5432)
  s.Property(&t.User).String().Required().Default("postgres")
  s.Property(&t.Password).String().Required().Default("postgres")
})

var DatabaseProvider = gonest.NewProvider(func(p *gonest.Provider) {
  p.Constructor(func() DatabaseConfig {
    // MustParse já popular via env source, panica com {field,message} igual REST se algo obrigatório
    // faltar sem default -- Provider.Constructor já tem precedente sólido de panicar (internal/
    // resolver/stage3.go's callConstructor recover()+errgroup cancel, ver seção "Provider" abaixo)
    return gonest.MustParse[DatabaseConfig](gonest.Dotenv(), databaseConfigSchema)
  })
})
```

`gonest.Dotenv()`'s `ParseInto` lê de `os.Getenv` (chave = tag `env:"..."`, exatamente como
`param:"..."`/`query:"..."` já mapeiam nome de campo → fonte). NÃO é `execution.Request`-scoped (não
existe request numa config de boot) — é uma fonte standalone, no mesmo espírito de `gonest.NewValue`/
`NewSchema` já serem chamáveis fora de qualquer request.

### `PropertyBuilder.Default(value)` — novo, decisão desta sessão: ENTRA no escopo

`Default(value any) *PropertyBuilder` também não existe hoje (grep confirmado, zero ocorrência fora
deste rascunho). Decisão: entra no escopo desta feature — sem ele, TODO campo de config sem env setada
vira erro `Required`, o que não é o comportamento real desejado (a maioria das vars de config tem
default razoável: porta, host). Semântica: campo ausente da fonte (env var não setada) usa `Default`
em vez de disparar `Required`; campo PRESENTE mas com valor inválido continua validando normalmente
(Default não é fallback pra erro de tipo, só pra ausência).

Nota: `Default` como feature genérica do `PropertyBuilder` beneficiaria TODO consumidor de `Schema`
(REST também), não só env — decisão de Tasks é se `Default` nasce escopado só pra `envSource`
(mínimo necessário) ou já sai geral (mais reuso, mas mais superfície de teste de uma vez).

### `required:"true"` como tag de struct — REJEITADO, usa `.Required()` do builder

O rascunho original tinha `required:"true"` como tag. Decisão desta sessão: **não** — toda outra branch
do framework (`String`/`Integer`/`Boolean`/etc.) já marca obrigatoriedade via `.Required()` no
`PropertyBuilder`, nunca por tag de struct. Uma tag `required:"true"` seria a ÚNICA exceção e uma
segunda forma redundante de expressar a mesma coisa. `env:"DB_HOST"` continua necessário (é o NOME da
env var, análogo a `param:"user_id"`/`query:"page"`) — só o `required` sai da tag.

### Coerção de tipo: reusa `coerceParamString` sem mudança

Variável de ambiente é sempre string crua — mesmo problema que `param`/`query`/`headers`/`form` já
resolvem hoje via `coerceParamString(raw string, kind string) (any, error)`
(`internal/validate/params.go:132`, já compartilhada por `params.go`/`query.go`/`headers.go`/
`form.go`). `envSource` reusa a MESMA função, zero mudança nela — `DatabaseConfig.Port int` lido de
`DB_PORT=5432` (string) já converte certo pra `int` do mesmo jeito que um path param `:port` já
converte hoje.

### Provider: precedente já suporta Constructor que falha

`internal/provider.Provider.Constructor` já aceita `func() T`/`func() (T, error)`/
`func(context.Context) T`/`func(context.Context) (T, error)` — um `Constructor` que chama
`gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)` e panica se a env estiver mal configurada
segue o MESMO caminho de erro que qualquer outro Constructor que já falha hoje:
`internal/resolver/stage3.go`'s `callConstructor` recupera o panic, converte em `error`
(`"gonest: provider for type %s panicked during resolution: %v"`), cancela o `context.Context` do
`errgroup`, e o erro sobe até `NewApp`/`MustNewApp` — nunca derruba o processo direto, mesmo tratamento
de qualquer outra falha de bootstrap. Zero mudança necessária em `internal/provider`/`internal/resolver`
pra isso funcionar.

### `NewValue` vs `NewSchema` — `NewSchema` é o certo aqui

`DatabaseConfig` é uma struct com múltiplos campos nomeados — usa `NewSchema[T]` (como o rascunho já
fazia, confirmado certo). `NewValue[T]`/`Accessor[T]` (Milestone 15) são pra um valor PRIMITIVO único
(ex.: um CPF string solto), não pra descrever campos nomeados de uma struct — não se aplica aqui.

---

## O que fica em aberto (fora do escopo desta reflexão, decisões de Design/Tasks quando formalizar)

- Formato exato de `gonest.Dotenv()` (parser própio vs. adaptar `gox/env`'s parser, expansão
  `${VAR}`, comentários) — feature 1, separada.
- `Default` nasce só pra `envSource` ou geral pra todo `PropertyBuilder` (REST incluso)?
- Múltiplos `Provider`s de config (um por domínio — `DatabaseConfig`, `RedisConfig`, etc.) vs. um
  `AppConfig` guarda-chuva único — nenhuma decisão tomada, ambos os padrões já cabem na API acima sem
  mudança (é só quantos `Provider`+`Schema` o dev declara).
- `Dotenv.ParseInto` reconstrói o snapshot de `os.Environ()` a cada chamada, ou cacheia? Relevante se
  `Dotenv().Load()` roda DEPOIS de algum `Parse[T](gonest.Dotenv(), ...)` já ter lido -- cache
  precisaria invalidar em `Load`, senão ordem de bootstrap importa de um jeito sutil.
