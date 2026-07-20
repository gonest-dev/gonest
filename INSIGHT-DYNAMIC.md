# Dynamic Modules — Insight

Rascunho de reflexão (não spec formal) sobre se/como gonest precisa de um equivalente aos Dynamic
Modules do NestJS. Motivado por `TODO.md`'s item pendente "Dynamic Modules: Permite que módulos sejam
customizados em tempo de inicialização usando `Register` ou `forRoot` recebendo configurações
dinâmicas (ex: `ConfigModule.forRoot()`)".

## Como funciona no NestJS (confirmado via Context7, `nestjs/docs.nestjs.com`)

- `@Module()` é um **decorator** — sua metadata (`providers`/`exports`/`imports`) é **estática**,
  decidida em tempo de DEFINIÇÃO da classe, não de uso.
- Um Dynamic Module é uma classe com um método `static` (convencionalmente `forRoot()`/`register()`)
  que recebe `options` em runtime e retorna um objeto `DynamicModule` (`{ module, providers, exports,
  imports? }`) — essas propriedades **estendem** (não substituem) a metadata estática do
  `@Module()` original.
- `forRootAsync()` existe pro caso em que `options` precisa vir de algo assíncrono/injetado (ex: outro
  Provider como `ConfigService`) — resolvido via `ConfigurableModuleClass`/factory, mecanismo mais
  pesado especificamente porque a metadata do decorator não pode "esperar" nada.
- Motivação central do padrão inteiro: decorators TS são estáticos, `forRoot()` é o **escape hatch**
  pra injetar config de runtime numa estrutura que, por padrão, não aceita.

## E no gonest?

gonest não tem decorator nenhum — `Module` já é um **valor Go de primeira classe**, construído por uma
função builder (`gonest.NewModule(func(m *gonest.Module) {...})`) que roda em Stage 1 (`assemble`), não
em tempo de "definição de classe". Isso significa que a limitação que motiva Dynamic Modules no Nest
**não existe aqui** — qualquer função Go comum que recebe `options` e retorna `*gonest.Module` já é,
por construção, um "Dynamic Module":

```go
type DatabaseOptions struct {
  DSN string
}

func DatabaseModuleForRoot(opts DatabaseOptions) *gonest.Module {
  connProvider := gonest.NewProvider(func(p *gonest.Provider) {
    p.Constructor(func() *Connection { return connect(opts.DSN) })
  })
  return gonest.NewModule(func(m *gonest.Module) {
    m.Providers(connProvider)
    m.Exports(connProvider)
  })
}

var AppModule = gonest.NewModule(func(m *gonest.Module) {
  m.Imports(DatabaseModuleForRoot(DatabaseOptions{DSN: "postgres://..."}))
})
```

**Confirmado rodando de verdade** (não assumido) — um scratch build real com esse padrão exato
(`Module.Imports`, `MustInject` de um Provider de OUTRO módulo importado dinamicamente) bootstrapou sem
erro nesta sessão. `gonest.NewModule`/`Module.Imports(mods ...*Module)` já aceitam isso sem NENHUMA
mudança de framework.

## O que precisa de atenção (não é grátis, mas não é código novo)

### 1. Deduplicação em diamond-imports

`internal/module/assemble.go`'s BFS (`assemble`) deduplica por **ponteiro** (`visited
map[*Module]bool`). Se dois módulos diferentes chamarem `DatabaseModuleForRoot(mesmasOpts)`
**separadamente**, cada chamada retorna um `*Module` NOVO — viram DOIS módulos distintos (dois
`Connection` diferentes, duas conexões reais), não um só compartilhado.

**Solução, sem mudar nada no framework**: mesmo padrão que TODO exemplo do repo já usa pra módulo
estático (`var UserModule = gonest.NewModule(...)`, uma `var` de pacote) — construir o dynamic module
UMA VEZ, guardar num `var` de pacote, e todo consumidor importa essa MESMA variável:

```go
var DatabaseModule = DatabaseModuleForRoot(DatabaseOptions{DSN: os.Getenv("DATABASE_URL")})

// em qualquer outro módulo que precise:
m.Imports(DatabaseModule) // MESMO ponteiro, assemble() dedupa igual a qualquer módulo estático
```

Isso é o análogo direto do padrão Nest "chame `forRoot()` uma vez no `AppModule`, os outros só
importam a classe sem chamar `forRoot()` de novo" — só que em Go a "instância única" É a própria
variável, não precisa de registry de classe nenhum.

### 2. Sem equivalente a `forRootAsync` (config vinda de outro Provider via DI)

`Module`'s builder fn (`func(*Module)`) **não** tem capacidade de `MustInject` — só `Provider`,
`Controller`, `Resolver` e `Listener` implementam `internal/inject`'s `directResolver` (via
`ResolveDirect`/`ResolveDirectAll`), `Module` não. Isso é estrutural: Stage 1 (assembly de módulos,
onde o builder de `Module` roda) acontece ANTES de qualquer Provider ser resolvido (Stage 3) — não tem
nenhum valor injetável ainda disponível nesse ponto.

Concretamente: **não dá** pra escrever algo como

```go
// NÃO FUNCIONA -- Module não tem MustInject
gonest.NewModule(func(m *gonest.Module) {
  config := gonest.MustInject[*ConfigService](m) // sem método ResolveDirect em Module
  ...
})
```

Isso cobre o caso onde `options` é um valor Go passado diretamente (o caso comum, `forRoot()`
síncrono do Nest) — mas NÃO cobre o caso `forRootAsync` onde a config precisa vir de outro Provider já
resolvido pelo DI. Pra esse segundo caso, o caminho natural em gonest já existe e é mais simples que o
do Nest: o PROVIDER que precisa da config resolve ela via `MustInject` dentro do próprio `Constructor`
(dependência Provider-a-Provider comum, já suportada desde sempre) -- não precisa de nenhum mecanismo
de módulo especial, só de estruturar a config como ela mesma sendo um Provider (ex: o já existente
`env-schema-binding`/`gonest.MustParse[Config](gonest.Dotenv(), schema)` rodando dentro do
`Constructor` de outro Provider).

## Conclusão preliminar (pré-Discuss)

**Hipótese central**: "Dynamic Modules" pra gonest não é uma feature nova de framework — é um PADRÃO de
uso que já funciona hoje, sem nenhuma linha de código nova. O trabalho real (se o usuário confirmar
que vale a pena) seria:

1. Documentar o padrão (README.md + site `/docs`) — uma função `XxxForRoot(opts) *Module` que fecha
   sobre `opts` e retorna um `*Module`, guardada numa `var` de pacote pra evitar diamond-import
   duplicado.
2. Talvez um exemplo novo em `.examples/` demonstrando isso de ponta a ponta com um caso real (ex: um
   módulo de "cache" ou "database" configurável).
3. **Nenhuma mudança em `internal/module`, `internal/app`, ou `gonest.go`** — `NewModule`/`Imports` já
   suportam o padrão inteiro tal como estão.

Ponto em aberto pra Discuss, se o usuário quiser seguir adiante: vale a pena formalizar isso com um
HELPER (ex: `gonest.NewDynamicModule[Options](func(opts Options) func(*Module) {...})`, um wrapper fino
só pra dar um nome/convenção oficial ao padrão) ou é melhor deixar 100% como "isso já é só Go, use
função normal" sem nenhuma API nova, só documentação? A segunda opção é mais alinhada com o resto do
projeto (evitar abstração que não compra nada -- ver coding-principles.md).
