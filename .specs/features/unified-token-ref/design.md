# Unified Token (TokenRef) — Design

## Approach

Novo marker de base, mesmo padrão dos outros (`internal/module/module.go`):

```go
type TokenRef interface {
    IsToken()
}
```

Todo marker existente passa a embutir `TokenRef` em vez de declarar seus próprios métodos soltos:

```go
type ProviderRef interface {
    TokenRef
    IsProvider()
    ResolvedType() reflect.Type
    SetOwnerModule(m *Module)
}

type ControllerRef interface {
    TokenRef
    IsController()
    SetOwnerModule(m *Module)
}

type ResolverRef interface {
    TokenRef
    IsResolver()
    SetOwnerModule(m *Module)
}

type MiddlewareRef interface {
    TokenRef
    IsMiddleware()
}

type FilterRef interface {
    TokenRef
    IsFilter()
}

type ListenerRef interface {
    TokenRef
    IsListener()
    SetOwnerModule(m *Module)
}

type SchedulerRef interface {
    TokenRef
    IsScheduler()
    SetOwnerModule(m *Module)
}
```

`ExportableRef` é deletado inteiro -- não vira alias de nada, simplesmente para de existir.

`*Module` implementa `TokenRef` diretamente (troca o antigo `IsExportable()`):

```go
func (m *Module) IsToken() {}
```

## Builder methods — todos com o mesmo esqueleto (type-switch + panic no default)

```go
func (m *Module) Providers(refs ...TokenRef) {
    for _, ref := range refs {
        p, ok := ref.(ProviderRef)
        if !ok {
            panic(fmt.Sprintf("gonest: Module.Providers received a TokenRef that is not a ProviderRef (%T)", ref))
        }
        m.providers = append(m.providers, p)
    }
}
```

Mesmo esqueleto pra `Controllers`/`Resolvers`/`Use`/`Filters`/`Listeners`/`Schedulers` (1 case
esperado cada, panic em qualquer outra coisa) e `Imports`:

```go
func (m *Module) Imports(refs ...TokenRef) {
    for _, ref := range refs {
        mod, ok := ref.(*Module)
        if !ok {
            panic(fmt.Sprintf("gonest: Module.Imports received a TokenRef that is not a *Module (%T)", ref))
        }
        m.imports = append(m.imports, mod)
    }
}
```

`Exports` continua com 2 cases válidos (era o único método já assim antes desta feature), mas
ganha o `default: panic` que faltava:

```go
func (m *Module) Exports(refs ...TokenRef) {
    for _, ref := range refs {
        switch v := ref.(type) {
        case *Module:
            m.exportedModules = append(m.exportedModules, v)
        case ProviderRef:
            m.exports = append(m.exports, v)
        default:
            panic(fmt.Sprintf("gonest: Module.Exports received a TokenRef that is neither a ProviderRef nor a *Module (%T)", ref))
        }
    }
}
```

Mensagem de panic sempre nomeia o método (`Module.Xxx`) e usa `%T` pro tipo concreto recebido --
mesmo padrão de clareza que `MustInject` já usa pra erro de resolução.

## Campos internos — inalterados

`m.providers []ProviderRef`, `m.controllers []ControllerRef`, `m.resolvers []ResolverRef`,
`m.exports []ProviderRef`, `m.exportedModules []*Module`, `m.middleware []MiddlewareRef`,
`m.filters []FilterRef`, `m.listeners []ListenerRef`, `m.schedulers []SchedulerRef`,
`m.imports []*Module` continuam com os MESMOS tipos de sempre -- só o parâmetro de ENTRADA dos
builders virou `TokenRef`, a assertion dentro do type-switch devolve o tipo concreto certo antes
de dar append. Todo getter (`OwnProviders`, `OwnControllers`, `ImportedModules`,
`ExportedProviders`, `EffectiveExports`, etc) e toda lógica de `assemble.go`/`resolver`
permanecem 100% inalterados -- leem os mesmos campos tipados de sempre.

## Tipos concretos — IsToken()

| Tipo | Arquivo | Ação |
| --- | --- | --- |
| `*provider.Provider` | `internal/provider/provider.go` | Remove `IsExportable()`, adiciona `IsToken()` |
| `*module.Module` | `internal/module/module.go` | Remove `IsExportable()`, adiciona `IsToken()` |
| `*module.providerAsRef` | `internal/module/provider_as.go` | Remove `IsExportable()`, adiciona `IsToken()` |
| `*controller.Controller` | `internal/controller/controller.go` | Adiciona `IsToken()` |
| `*graphql.Resolver` | `internal/graphql/resolver.go` | Adiciona `IsToken()` |
| `*middleware.Middleware` | `internal/middleware/middleware.go` | Adiciona `IsToken()` |
| `*filter.Filter` | `internal/filter/filter.go` | Adiciona `IsToken()` |
| `*emitter.Listener[T]` | `internal/emitter/listener.go` | Adiciona `IsToken()` |
| `*scheduler.Scheduler` | `internal/scheduler/scheduler.go` | Adiciona `IsToken()` |

Fakes de teste que implementam qualquer um desses markers ganham `IsToken()` do mesmo jeito
(`internal/module/module_test.go`: `fakeProvider`, `fakeController`, `fakeResolver`,
`fakeMiddleware`, `fakeFilter`, `fakeListener` se existir, `fakeScheduler` se existir;
`internal/resolver/{resolver_test.go,direct_test.go}`: `fakeProvider`).

## gonest.go

```go
// remove:
// type ExportableRef = module.ExportableRef

// adiciona (perto dos outros 7 alias, mesmo bloco):
type TokenRef = module.TokenRef
```

Doc comment do bloco de alias (`ProviderRef`/`ControllerRef`/etc, já existente) ganha 1 frase
citando que todos agora embutem `TokenRef` e por quê (permite reusar a mesma slice tipada em mais
de um builder sem conversão).

## Consumer erc — migração

3 arquivos com o padrão idêntico (`app/auth/module.go`, `app/system/module.go`,
`infra/database/module.go`):

```go
// antes:
var providers = []gonest.ProviderRef{...}
m.Providers(providers...)
m.Exports(any(providers).([]gonest.ExportableRef)...)

// depois:
var providers = []gonest.TokenRef{...}
m.Providers(providers...)
m.Exports(providers...)
```

Nenhum outro callsite em `erc` usa slice tipada de `ControllerRef`/`ResolverRef`/etc (confirmado
via grep -- só esses 3 arquivos referenciam qualquer `gonest.XxxRef`), então nenhum outro arquivo
muda.

## Tech Decisions

| Decision | Rationale |
| --- | --- |
| `TokenRef` vira base de TODOS os 7 markers + `Imports`, não só `Providers`/`Exports` | Usuário pediu escopo máximo explicitamente ("unifica tudo") -- paridade total com o "token" do Nest, que é intercambiável entre `providers`/`imports`/`exports`/`controllers` sem tipo dedicado por conceito. |
| Panic fail-fast em vez de ignorar silenciosamente (inclusive fix retroativo no `Exports`) | Perder o erro de compilação (trade-off aceito abaixo) só é seguro se o erro em runtime for ALTO-SINAL -- ignorar silenciosamente deixaria um provider sumir do grafo sem pista nenhuma, pior que hoje. Mesma postura fail-loud de `MustInject`. |
| `ExportableRef` deletado sem alias/deprecated | Mesma filosofia "uma forma de fazer" já usada em toda decisão anterior (AD-052 bullet 4, este projeto nunca manteve 2 nomes pro mesmo conceito). |
| Getters/campos internos/assembly/resolver inalterados | Escopo é estritamente na ENTRADA dos builders -- a leitura já era por tipo concreto (`[]ProviderRef` etc) e continua sendo; reescrever leitura seria mudança não pedida e sem motivo. |

## Trade-off (novo, não existia antes desta feature)

Perda de checagem em tempo de compilação nos builders: hoje `m.Controllers(someProvider)` não
compila (tipo errado); depois desta feature, compila (qualquer `TokenRef` é aceito) e só panica em
runtime na primeira execução daquele código. Aceito porque (a) o panic é imediato e nomeia
método+tipo, não é um bug silencioso, e (b) é o preço estrutural de Go não ter union types --
mesmo preço que `Exports` já pagava desde AD-052, agora generalizado pros outros 8 métodos.

## Testing Strategy

`go test ./... -race -count=1` (gonest repo) -- gate padrão de toda feature anterior. Testes novos
cobrindo o panic path de cada builder (`Providers`/`Controllers`/`Resolvers`/`Use`/`Filters`/
`Listeners`/`Schedulers`/`Imports`/`Exports` recebendo um `TokenRef` do tipo errado) usando
`recover()` + checagem de mensagem, mesmo padrão que testes de panic existentes no repo já usam
(`MustInject`, etc -- confirmar padrão exato lendo um teste de panic existente antes de escrever
os novos). Testes existentes que só passavam 1 valor por vez continuam passando sem mudança
(assinatura mais ampla aceita o mesmo valor). Testes que faziam spread de slice tipada
(`fakeProviders...` etc, se existirem) precisam trocar a declaração da slice pra `[]TokenRef`.
Consumer `erc`: `go build ./...` (ou `go vet ./...`) depois da migração, confirmando compilação
limpa -- não é parte da suite de testes do gonest, mas é o critério real de "funcionou" pro
achado que motivou a feature.
