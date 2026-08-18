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

### 2.1 Correção: `Module.Lazy`+`MustInject[T](l)` FAZ o caso "decidir Imports com base num Provider
real" -- achado real, dogfooding fora deste repo

O parágrafo acima ("não dá pra escrever `MustInject` dentro do `fn` de `NewModule`") é verdade só pro
`*Module` DIRETO. Existe uma 3ª via, já shipada desde Milestone 24 (Module Lazy Loading,
`internal/module/lazy.go`), que NÃO aparecia aqui até esta sessão: `Module.Lazy(fn func(l
*LazyModule))` roda `fn(l)` IMEDIATAMENTE (ainda dentro do `fn` já-deferido do `NewModule`, Stage 1),
e `gonest.MustInject[T](l)` -- reparar, `l` é `*LazyModule`, não `*Module` -- dispara um 3º branch de
dispatch em `internal/inject/inject.go`'s `Must` (`mustLazy`, linhas 247-335) que invoca o
Constructor do Provider alvo VIA REFLECT, agora, síncrono, sem esperar Stage 3.

Isso resolve exatamente o caso motivador desta seção -- "decidir qual submódulo importar com base
numa config real, JÁ REGISTRADA como Provider, sem duplicar parse" -- contanto que o Provider alvo
cumpra 3 condições (cada uma com panic próprio se violada):

- **T precisa ser pointer** (`inject.go:262-263`, panic imediato se não for -- por isso
  `MustInject[*Config](l)`, nunca `MustInject[Config](l)`).
- **Provider alvo já registrado no MESMO módulo, ANTES do `Lazy`** (`m.Providers(Config_)` precisa
  vir antes de `m.Lazy(...)` no código -- `mustLazy` procura só em `l.OwnProviders()`, panic "no
  provider... registered via this module's own Providers" se não achar).
- **Provider alvo sem NENHUMA dependência própria** (zero `MustInject` dentro do SEU PRÓPRIO builder
  fn -- LAZY-06, panic "recorded a new dependency during eager construction" se violar) **e** Scope
  Singleton (default -- LAZY-07, panic se `Transient`/`Request`).

Achado confirmado rodando de verdade (scratch build + suite real, `-race`, trocando a env var e
provando que o submódulo importado muda de fato via HTTP round-trip) E aplicado a um caso real fora
deste repo (`erc/ctrl/api`'s `internal/infra/broker/module.go` -- broker com 2 implementações,
`fake`/`nats`, selecionadas por `BROKER_PROVIDER`):

```go
var BrokerModule_ = gonest.NewModule(func(m *gonest.Module) {
	m.Providers(config.BrokerConfig_) // registrado ANTES do Lazy -- fica em l.OwnProviders()

	m.Lazy(func(l *gonest.LazyModule) {
		cfg := gonest.MustInject[*config.BrokerConfig](l) // eager-resolve REAL, mesmo Provider usado por DI normal em qualquer outro lugar
		var selected *gonest.Module
		switch cfg.Provider {
		case "fake":
			selected = fake.BrokerFakeModule_
		case "nats":
			selected = nats.BrokerNatsModule_
		default:
			panic(fmt.Sprintf("broker: invalid BROKER_PROVIDER %q", cfg.Provider))
		}
		l.Imports(selected)
		l.Exports(selected) // sem isso, quem importar BrokerModule_ não injeta nada do submódulo escolhido
	})
})
```

Zero parse duplicado (diferente da alternativa "chama a função de parse 2x, uma síncrona outra dentro
do Constructor" cogitada antes nesta sessão e rejeitada -- essa versão via `Lazy` usa o Provider
REGISTRADO uma vez só, a mesma instância que qualquer outro consumidor via DI normal também recebe).

**Escopo real da correção**: isso cobre só "escolher entre N submódulos baseado num valor de UM
Provider zero-dep Singleton registrado no PRÓPRIO módulo". NÃO cobre o `forRootAsync` genérico onde a
config viria de uma CADEIA de Providers com dependências entre si (LAZY-06 bloqueia exatamente esse
caso de propósito -- eager-resolve com dependência própria quebraria a ordenação topológica normal de
Stage 3). Pra esse caso mais geral, o caminho ainda é o de sempre: resolver dentro do `Constructor` do
Provider que realmente precisa, não do `Module`.

## De/Para: ConfigurableModule e SelectableModule (mixins reais, TS)

Achado dogfooding gonest fora deste repo: 2 mixins TS reais (`configurableModuleMixin`/
`selectableModuleMixin`, `@packages/nest-core/src/mixin`) resolvem 2 problemas distintos que juntos
formam o caso concreto motivador desta seção -- um módulo (`WhatsappModule`) com 2 implementações
(`fake`/`fetch`), cada implementação com sua PRÓPRIA config validada e injetada via DI
(`nest-whatsapp/src/module.ts` + `implementation/{fake,fetch}/module.ts`).

### 1. ConfigurableModule -- "factory provider via dynamic module"

**Nest** (`configurableModuleMixin`): a classe do módulo estende um mixin que ganha `forRootAsync({useFactory,
inject})` -- registra `{provide: configClass, useFactory, inject}` como provider. `configClass` é só o TOKEN
DI (a classe usada tanto de tipo quanto de identidade em runtime); `useFactory`/`inject` permitem a config
depender de outro provider já resolvido pelo container.

```ts
@Module({})
export class WhatsappFakeModule extends configurableModuleMixin({
  configClass: WhatsappFakeConfig,
  providers: [WhatsappFakeAdapter, WhatsappFakeConfig, WhatsappFakeLifecycle],
  exports: [WhatsappFakeAdapter, WhatsappFakeConfig, WhatsappFakeLifecycle],
}) {}

// uso: WhatsappFakeModule.forRootAsync({ useFactory: () => WhatsappFakeConfig.new({...}) })
```

**gonest**: `Provider.Constructor` NÃO aceita parâmetro de dependência nenhum -- só 4 assinaturas
fixas (`func() T`/`func() (T, error)`/`func(context.Context) T`/`func(context.Context) (T, error)`,
validadas via reflect em `internal/provider/provider.go`'s `isValidConstructorSignature`, panic pra
qualquer outra forma). Diferente de `useFactory: (...args) => T, inject: [...]` (PUSH -- o framework
chama sua função passando os deps por posição), gonest é PULL: a dependência é resolvida via
`gonest.MustInject[T](p)` chamado DENTRO do `fn` externo de `NewProvider` (Stage 2 -- devolve um
placeholder por enquanto, preenchido antes do Constructor rodar pela ordenação topológica de Stage 3),
capturada por closure, e só então `p.Constructor(func() T {...})` (Stage 3, zero-arg de verdade) usa o
valor já resolvido. Resultado funcional idêntico a `useFactory`+`inject`, mecanismo diferente:

```go
var Config_ = gonest.NewProvider(func(p *gonest.Provider) {
	p.Constructor(func() *Config {
		return gonest.MustParse[*Config](gonest.Dotenv(), configSchema) // equivalente a useFactory sem inject
	})
})

var Adapter_ = gonest.NewProvider(func(p *gonest.Provider) {
	cfg := gonest.MustInject[*Config](p) // equivalente a inject: [Config] -- PULL, não parâmetro de Constructor
	p.Constructor(func() Port { // Constructor continua zero-arg -- cfg já resolvido, só capturado por closure
		return NewFakeAdapter(cfg)
	})
})

var WhatsappFakeModule_ = gonest.NewModule(func(m *gonest.Module) {
	m.Providers(Config_, Adapter_)
	m.Exports(Adapter_)
})
```

Zero API nova -- `NewProvider`+`Constructor`+`MustInject` (chamado no ponto certo, Stage 2, nunca
dentro do próprio `Constructor`) cobrem `forRootAsync`'s caso geral desde sempre (Milestone 1, DI
core). O que muda é a FORMA: Nest declara deps como array (`inject: [...]`) e recebe por posição no
`useFactory`; gonest declara puxando (`MustInject` explícito por linha) e captura por closure -- sem
decorator/classe pra "declarar o token", o próprio tipo do generic (`*Config`) já É o token.

### 2. SelectableModule -- "construir módulos dinamicamente" (escolha entre N ConfigurableModules)

**Nest** (`selectableModuleMixin`): módulo com `selectionMap: Record<string, ModuleClass>`:
`forRootAsync({provider, useFactory, inject})` escolhe o módulo pelo `provider` (string SEMPRE
síncrona -- nunca vem de DI, é passada como literal), e se o módulo escolhido também expõe
`forRootAsync`, delega `useFactory`/`inject` pra ele:

```ts
@Module({})
export class WhatsappModule extends selectableModuleMixin({
  selectionMap: { fake: WhatsappFakeModule, fetch: WhatsappFetchModule },
}) {}

// uso: WhatsappModule.forRootAsync({ provider: 'fake', useFactory: () => WhatsappFakeConfig.new({}) })
```

**gonest**: `map[string]*gonest.Module` + `Module.Lazy`+`MustInject[T](l)` (seção 2.1 acima) pra ler
"qual provider" a partir do Provider de config REAL, já registrado -- não repete parse. Cada entrada
do map já é um `WhatsappFakeModule_`/`WhatsappFetchModule_` completo (seção 1):

```go
var WhatsappModule_ = gonest.NewModule(func(m *gonest.Module) {
	m.Providers(WhatsappConfig_) // registrado ANTES do Lazy -- fica em l.OwnProviders()

	m.Lazy(func(l *gonest.LazyModule) {
		cfg := gonest.MustInject[*WhatsappConfig](l) // eager-resolve REAL, mesmo Provider usado por DI normal
		choices := map[string]*gonest.Module{
			"fake":  fake.WhatsappFakeModule_,
			"fetch": fetch.WhatsappFetchModule_,
		}
		choice, ok := choices[cfg.Provider]
		if !ok {
			panic(fmt.Sprintf("whatsapp: unknown provider %q (valid: fake, fetch)", cfg.Provider))
		}
		l.Imports(choice)
		l.Exports(choice)
	})
})
```

Diferença estrutural real (não é limitação, é consequência do modelo): no Nest, `selectionMap`
repassa `useFactory`/`inject` PRA FORA (quem monta `AppModule` decide a config do submódulo
escolhido, no ponto de uso). Em gonest, cada submódulo (`fake`/`fetch`) já se autoconfigura (seção 1)
-- não tem "ponto de uso" separado repassando factory, porque `Provider.Constructor` já roda com DI
completo onde quer que o módulo seja importado. Resultado prático idêntico (a implementação
selecionada acaba com sua config validada e injetada), só o LOCAL onde a config é declarada muda: no
Nest fica no call site de `forRootAsync`; em gonest fica dentro do próprio módulo escolhido (mais
perto do código que a usa, sem precisar propagar `useFactory`/`inject` por 2 camadas).

**Conclusão desta seção**: os 2 mixins não exigem NENHUMA mudança em `internal/module`/`internal/app`/
`gonest.go` -- `NewModule`+`NewProvider`+`Constructor`+`Module.Lazy`+`MustInject` já cobrem os 2
casos, TODOS já existentes antes desta sessão. Um helper genérico tipo `module.Selectable[T](m, key,
choices)` recebendo `key`/`choices` de FORA foi tentado de verdade num projeto consumidor
(`erc/ctrl/api`) e NÃO compôs -- não tem como alcançar `Lazy`/`MustInject` de fora do `fn` de
`NewModule`. A forma que funciona é sempre inline, dentro do próprio `NewModule`+`Lazy`, como os 2
exemplos acima.

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

# Hipótese estrutural refletindo exatamente o que o nestjs faz

```go
package ex

import (
  "reflect"
  "gonest.dev/gonest"
)

type Service struct {}
func (s *Service) Hello() string { return "world" }

var ValueProvider_ = gonest.NewProvider[*Service](func (p *gonest.Provider) {
  p.Global() // marca (que o provider é global)
  p.Provide("ValueProvider[Service]") // o name seria opcional, se não passado faz via reflect.TypeFor do NewProvider[T any]
  p.Value(&Service{})
})

var ClassProvider_ = gonest.NewProvider[*OtherService]() // a função passa a ser opcional 

var FactoryProvider_ = gonest.NewProvider(func (p *gonest.Provider) {
  p.Inject(
    gonest.Token[*OtherService](), 
    gonest.Token[any]("someOtherService"))
  p.Factory(func(f *gonest.Factory) *OtherService {
    otherService := gonest.MustInject(f)
    depValue := otherService.DoStuff()
    return &Service{depValue: depValue}
  })
})

var Module_ = gonest.NewModule(func (m *gonest.Module) {
  m.Providers(
    ValueProvider_,
    ClassProvider_,
    FactoryProvider_
  )
})
```