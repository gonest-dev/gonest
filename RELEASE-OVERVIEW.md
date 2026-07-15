1# Sem tag de versão/semver ainda — é tudo master, sem release formal

- ok, definido nas `.specs`

  **Evolução:** convenção `v0.{major}-{minor}.{release}` registrada em `.specs/project/STATE.md` (2026-07-15). `v0` fixo pra sempre. Nenhuma tag cortada ainda -- número inicial fica pra decidir depois de mais uso real.

2# Mudança MAIOR e recente no núcleo (bootstrap 3 fases, T0 desta sessão) — arquitetura de DI inteira foi reordenada há poucas horas, ainda não "assentou" com uso real

- identifiquei que o fiber está mostrando seu feedback por default, ex:

    ```bash
         _______ __
       / ____(_) /_  ___  _____
      / /_  / / __ \/ _ \/ ___/
     / __/ / / /_/ /  __/ /
    /_/   /_/_.___/\___/_/          v3.4.0
    --------------------------------------------------
    INFO Server started on:         http://127.0.0.1:3000 (bound on host 0.0.0.0 and port 3000)       
    INFO Total handlers:            7
    INFO Prefork:                   Disabled
    INFO PID:                       65672
    INFO Total process count:       1
    ```

- queria que tenhamos uma apresentação nossa e não do fiber como o nestjs faz mesmo que no log diga que está usando o fiber,
  afinal vamos ter outros adapters no futuro

  **Evolução:** feito (commit `97740cb`). Banner do Fiber suprimido via `fiber.ListenConfig{DisableStartupMessage: true}`. `App.MustListen` agora imprime o próprio banner do gonest (formato item #4 abaixo), igual em qualquer adapter presente ou futuro. Rodando `blog-api` de verdade:

    ```
    2026-07-15T15:29:44.112Z [INFO] Gonest started on: http://:3001
    2026-07-15T15:29:44.112Z [INFO] Loaded:            Modules(5), Controllers(3), Routes(8)
    2026-07-15T15:29:44.112Z [INFO] PID:               69568
    ```

3# Só 1 adapter HTTP (Fiber) — nunca testado contra alternativa, "abstração" é teórica

- vamos pensar em outros adapters somente quando tivermos uma versão 1.0 e com uso real

  **Evolução:** sem ação -- decisão mantida, nenhum trabalho feito aqui de propósito.

4# Sem Logger real — panics em Emitter/Scheduler são engolidos silenciosamente (recover() sem log nenhum), aceitável em dev mas não em produção

- precisamos realmente de um logger, mesmo que seja simples, para acompanhar o que está acontecendo como o nestjs faz printando
  fazer algo como:

    TIMESTAMP (format: ISO 8601: YYYY-MM-DD'T'HH:mm:ss.sss'Z') | LOG LEVEL | MESSAGE | CONTEXT
    
    2026-07-15T10:18:07.123Z [INFO] Gonest started on: http://127.0.0.1:3000
    2026-07-15T10:18:07.123Z [INFO] Loaded:            Modules(1), Controllers(2), Routes(7)
    2026-07-15T10:18:07.123Z [INFO] PID:               70652

  **Evolução:** feito (commit `97740cb`), formato EXATO do exemplo acima. `internal/logger` novo -- `Configure([]LogLevel)` (liga em `AppOptions.LogLevels`, antes inerte), `Error`/`Warn`/`Info`/`Debug`/`Verbose`. Default: Error+Warn+Info habilitados, Debug/Verbose exigem opt-in (igual NestJS). `Emitter.Emit`/Scheduler's `runIsolated` agora chamam `logger.Error(...)` no lugar do `recover()` mudo. Ainda NÃO exposto na raiz pro usuário chamar diretamente (`gonest.Logger`/customização de output) -- só uso interno do framework por enquanto; avisar se precisar de logger customizável publicamente.

5# Scheduler sem Stop/cancelamento — goroutine roda até o processo morrer, sem jeito de desligar um job

- precisa funcionar o mais próximo possível da api do https://docs.nestjs.com/techniques/task-scheduling que inclui
  intervals, timeouts, permitindo a declaração dinamica e podendo ser parado quando quisermos.

  **Evolução:** feito (commit `97740cb`). `Scheduler.Stop(name string)` -- Cron/Interval param de disparar (parada afeta só disparos FUTUROS, execução já em andamento não é interrompida), Timeout ainda não disparado nunca dispara. Implementado via `chan struct{}` + `sync.Once` por job nomeado, sem lib nova além do `robfig/cron/v3` já usado pro parsing de expressão. NÃO replica a API completa do NestJS (`SchedulerRegistry.getCronJobs()`, listagem de jobs registrados, etc) -- só `Stop(name)`. Avisar se precisar de mais paridade (listar jobs ativos, por exemplo).

6# Zero uso em projeto real fora dos próprios testes — nunca rodou contra tráfego de verdade

- em teoria com os .examples podemos considerar testes reais não? além disso acredito que podemos criar novos exemplos 
  cada vez mais complexos ali pra simular o uso real. 

  **Evolução:** feito (commits `a7d0018` e seguintes). `.examples/simple-todo` (MVC mínimo, in-memory, sem deps externas) e `.examples/blog-api` (denso: SQLite via `modernc.org/sqlite`, Guard/Interceptor/Middleware/Filter, User→Posts→Comments simulando M:N via 2 FKs, OpenAPI/Swagger). Ambos rodados de verdade via `curl`, não só compilados. Dogfooding JÁ achou e corrigiu bugs reais:
  - `gonest.Context`/`gonest.FiberApp`/`gonest.InterceptorNext` nunca tinham sido promovidos pra raiz (INSIGHT.md já assumia que existiam)
  - 4 comentários de doc no `gonest.go` (`NewMiddleware`/`NewGuard`/`NewInterceptor`/`NewFilter`) desatualizados desde a reversão do AD-008 nesta mesma sessão
  - deadlock real de `db.SetMaxOpenConns(1)` + SQLite `:memory:` sob fasthttp -- corrigido usando `file::memory:?cache=shared`
  - `.examples/*` viraram módulos Go PRÓPRIOS (go.mod + replace local) depois que `go mod tidy` na raiz removeu `modernc.org/sqlite` por engano (dot-dir é invisível pro `go build/test/mod tidy ./...` da raiz)

7# modificação o gonest.NewHttpException

- acho que poderia ser uma fluent api do tipo builder, ex:

  ```go
  package ex

  import "github.com/gonest-dev/gonest"

  type DuplicateEmailException struct {	gonest.HttpException }
  func NewDuplicateEmailException(email string) *DuplicateEmailException {
    return &DuplicateEmailException{ HttpException: gonest.NewHttpException().
      SetStatus(http.StatusConflict). // se não definido é 500
      SetName("DuplicateEmailException"). // se não definido pega o nome da struct
      SetMessage("email already in use"). // se não definido fica em branco
      SetDetails(map[string]string{"email": email}) // se não definido fica nil
    }
  }
  ```

- também acho que no arquivo `C:\dev\gonest-dev\gonest\.examples\blog-api\shared\exception.go` 
  poderia existir uma forma mais fácil de fazer a conversão pra json, talvez se o gonest.HttpException
  implementar o MarshalJSON e UnmarshalJSON de forma a evitar a necessidade de definição explícita
  dos campos quando extender do gonest.HttpException

  **Evolução:** feito com 1 ressalva importante (commit `6933320`). Builder fluente implementado exatamente como pedido: `NewHttpException()` (zero-arg, status default 500) + `SetStatus`/`SetName`/`SetMessage`/`SetDetails` encadeáveis, cada um devolvendo uma cópia nova (imutável). `MarshalJSON` implementado -- `ctx.Json(exc)` agora funciona sozinho, sem mapa manual (já simplificado em `.examples/blog-api/shared/exception.go`).

  **Ressalva no "SetName se não definido pega o nome da struct":** GENUINAMENTE não dá pra fazer isso dentro do `Name()`/`MarshalJSON()` do próprio `HttpException` -- método promovido por embedding nunca enxerga o tipo de FORA que o está embedando (Go não tem essa introspecção). Resolvido com `EffectiveName(exc)` (`gonest.ExceptionName` na raiz): função separada que recebe o valor completo (com tipo concreto de verdade) e faz fallback via reflect pro nome do tipo SE `Name()` vier vazio. O formatador padrão de exceção não tratada (`internal/adapter/fiber`) já usa isso automaticamente; um Filter próprio (como `DomainFilter`) precisa chamar `gonest.ExceptionName(exc)` explicitamente se quiser o mesmo fallback -- `ctx.Json(exc)` sozinho usa o nome cru (vazio se `SetName` nunca foi chamado).

  **`UnmarshalJSON` NÃO implementado** -- não achei um caso de uso claro (uma exceção normalmente é CONSTRUÍDA via builder no código, não DECODIFICADA de JSON recebido). Avisar se há um cenário real precisando disso (ex: cliente HTTP do gonest reconstruindo a exceção a partir da resposta de outro serviço?).