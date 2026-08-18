# hipótese de logger global estruturado no gonest

O nestjs tem um logger geral que pode ser substituido mediante interface, ex:

```ts
// src/app.logger.ts
import {type LoggerService} from '@nestjs/common';
@Injectable()
export class AppLogger implements LoggerService { /* ... */ }
```

```ts
// src/app.module.ts
import {Module} from '@nestjs/common';
@Module({ provider: [AppLogger] })
export class AppModule {}
```

```ts
// src/main.ts
import {AppLogger} from './app.logger';
import {AppModule} from './app.module';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, {bufferLogs: true, logger: ['error', 'warn']});
  app.useLogger(await app.resolve(AppLogger))
  // ...
}
void bootstrap()
```

Queria que pudéssemos ter o mesmo efeito em gonest: uma interface `gonest.Logger` 
explícita e que é usada internamente pelo framework e e exposta para uso pelo 
ecosistema ex:

```go
package gonest

type Logger interface {
  Error(message string, meta ...map[string]any)
  Warn(message string, meta ...map[string]any)
  Info(message string, meta ...map[string]any)
  Debug(message string, meta ...map[string]any)
  Verbose(message string, meta ...map[string]any)
}
```

```go
package app
import "gonest.dev/gonest"
// app_logger.go
type AppLogger struct{}
var _ gonest.Logger = (AppLogger)(nil)
func (l *AppLogger) Error(message string, meta ...map[string]any)   { /* ... */ }
func (l *AppLogger) Warn(message string, meta ...map[string]any)    { /* ... */ }
func (l *AppLogger) Info(message string, meta ...map[string]any)    { /* ... */ }
func (l *AppLogger) Debug(message string, meta ...map[string]any)   { /* ... */ }
func (l *AppLogger) Verbose(message string, meta ...map[string]any) { /* ... */ }
```

```go
// main.go
func main() {
	app := gonest.MustNewApp[gonest.FiberApp](AppModule_, gonest.AppOptions{
		Logger: &AppLogger{}, // troca a implementação já no factory, sem app.UseLogger() depois
	})
	app.MustListen(":3000")
}
```

Diferença de forma vs Nest: Nest precisa de `bufferLogs: true` + `app.useLogger(await
app.resolve(AppLogger))` em 2 passos porque `AppLogger` pode ser um Provider de verdade (resolvido
via DI, podendo ter suas próprias dependências) e o container só fica pronto DEPOIS de
`NestFactory.create`. Em gonest, `AppOptions.Logger` é só um valor Go passado direto no factory --
sem 2º passo, sem `bufferLogs` (nada fica bufferizado esperando, porque não tem essa janela
assíncrona entre "container criado" e "logger anexado").

A intenção é que eu possa chamar não só o logger diretamente como 

```go
package some
import "gonest.dev/gonest"

type Service struct {}
func( s *Service) Hello() { 
  gonest.GetLogger().Info("world") 
  // output como `[timestamp] INFO {{message}} {{JSON meta}}`
}
```

Quanto criar um logger que carregue uma referência explícita como

```go
package some
import "gonest.dev/gonest"

type Service struct {
  logger gonest.Logger
}
func( s *Service) Hello() { 
  s.logger.Info("world") 
  // output como `[timestamp] INFO [Service] {{message}} {{JSON meta}}`
}

var Service_ = gonest.NewProvider(func(p *gonest.Provider) {
  p.Constructor(func() *Service {
    return &Service{ logger: gonest.GetLogger("Service") }
  })
})
```

Decisão: são 2 funções diferentes, as 2 existem -- `GetLogger(optionalNamedContext ...string)`
(contexto como STRING explícita, escrita por quem chama) e `GetLoggerFor[T any]()` (contexto DERIVADO
do nome do tipo `T` via reflect, zero string manual). Ambas devolvem `Logger`, ambas envolvem `active`
com o mesmo `contextLogger` interno -- só a ORIGEM do nome muda:

```go
// gonest.go / internal/logger

// GetLogger returns the active Logger, optionally wrapped to prefix every
// line with a caller-chosen context name.
func GetLogger(optionalNamedContext ...string) Logger {
	if len(optionalNamedContext) > 1 {
		panic("gonest: GetLogger accepts at most one named context")
	}
	if len(optionalNamedContext) == 0 {
		return active // sem contexto, delega direto
	}
	return &contextLogger{name: optionalNamedContext[0], parent: active}
}

// GetLoggerFor returns the active Logger wrapped to prefix every line with
// T's own type name -- same wrapper as GetLogger(name), context derived via
// reflect instead of a literal string.
func GetLoggerFor[T any]() Logger {
	return &contextLogger{name: reflect.TypeFor[T]().Name(), parent: active}
}
```

`contextLogger` (interno, compartilhado pelas 2) só prefixa `[name]` antes de repassar pra `active`.
Quando usar cada uma: `GetLoggerFor[*Service]()` quando o contexto DEVE bater 1:1 com o tipo Go (caso
comum, feature típica em `.examples/*` -- 1 contexto por struct); `GetLogger("nome-livre")` quando o
contexto precisa divergir do tipo (ex: agrupar por FEATURE em vez de por struct, ou contexto dinâmico
que não é um tipo Go, tipo `"cron:invoice-sync"`).

**Nota**: `active` sempre lido NA HORA da chamada (`GetLogger()`/`.Info()`), nunca capturado antes --
se `Service_`'s `Constructor` guarda `gonest.GetLogger("Service")` numa struct field (exemplo acima),
o VALOR retornado (o `Logger` -- ou o `active` puro, ou o `contextLogger` wrapper) já é fixo a partir
dali; se alguém trocar `active` DEPOIS (não deveria acontecer em produção, só teste trocando
`AppOptions.Logger` entre bootstraps), esse `Service` já resolvido continua com a referência antiga.
Coerente com qualquer outro valor resolvido por DI (nada em gonest hoje reage a troca de
config/instância DEPOIS de resolvido), só vale documentar pra não surpreender.
