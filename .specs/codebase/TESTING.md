# Testing

**Status:** maduro — v1 completo (Milestones 1-11) + Milestone 12 (Multipart Form Streaming) pós-v1, suite inteira verde ao longo de toda a sessão.

## Tooling

- Runner: `go test` (stdlib)
- Assertions: stdlib puro (`t.Fatalf`/`t.Errorf`/`t.Fatal`) -- `testify/assert` foi cogitado no planejamento inicial (greenfield) mas NUNCA usado na prática; não está em `go.mod`. Atualizado aqui pra refletir a convenção real, não a intenção original.
- Race detector: sempre ligado (`-race`) — relevante desde a 1ª feature por causa do `errgroup`/paralelismo no bootstrap

## Gate Check Commands

**Ambiente Windows local:** o workaround antigo (`CC=clang` injetado quebrando o build cgo do `-race`, exigindo override inline `CC=gcc CXX=g++ PATH=...`) não se aplica mais nesta sessão do harness -- `CC`/`CXX` já vêm resolvidos (`gcc`/`g++`) sem override manual, confirmado empiricamente rodando `go test ./... -race` puro repetidamente. Simplificado de volta, exatamente como a nota anterior já previa.

| Gate | Command | Quando roda |
| --- | --- | --- |
| quick | `go test ./... -race` | A cada task com só `unit` |
| full | mesmo comando acima (sem flag extra — `app.Test(req)` do Fiber não sobe porta real, roda dentro do `go test` normal, sem precisar de gate separado) | Task com `integration` + antes de fechar a feature inteira |

**Se o workaround `CC=clang` voltar a aparecer** (harness reiniciado com config antiga), o prefixo é `CC=gcc CXX=g++ PATH="<pasta do MinGW-w64 instalado via winget>:$PATH"` na frente de qualquer comando `go test`/`go build` que use `-race`.

## Test Coverage Matrix

| Code Layer | Test Type Required |
| --- | --- |
| Builders públicos (`NewModule`, `NewProvider`, `NewController`, `MustInject`) | unit |
| Motor interno de resolução (`internal/resolver` — 3 estágios) | unit |
| Detecção de ciclo | unit |
| Erros de escopo/export (módulo não exporta, tipo não registrado) | unit |
| `Route`/`Pipe`/`Context` isolados (builder, coerção de param, validação de assinatura) | unit |
| Dispatch de rota via Fiber real (`internal/adapter/fiber`, `app.go`'s Stage 2.5) — request HTTP de verdade batendo numa rota registrada | integration (usa `app.Test(req)` do próprio Fiber, sem subir porta real) |
| Bind/Listen real (`internal/adapter/fiber`, `HttpAdapter.Listen`) — porta TCP de verdade aberta, `OnListen` disparando, `App.MustListen` bloqueando até shutdown | integration (sobe porta real em `127.0.0.1:<porta fixa>`, sincronizado via channel/waitgroup — nunca `time.Sleep` — e derrubada em `t.Cleanup` via `Shutdown()`) |
| Schema/reflection puro (`internal/schema` — identificação de campo via offset de ponteiro, builders `Schema`/`PropertyBuilder`) | unit (sem dependência de DI graph/HTTP — domínio isolado, cada campo confirmado individualmente por nome+tipo, não só "compilou") |

**Atualizado na feature "Controller & Route Registration":** primeira camada e2e/integration do projeto — dispatch HTTP real via Fiber precisa provar que rota registrada responde corretamente, não só que a struct `Route` foi montada certo (isso é unit).

**Atualizado na feature "App Bootstrap & Listen":** segunda camada integration — agora cobrindo bind real de porta (`Listen`/`MustListen`/`OnListen`) e, no T6 final, um dial de verdade via `net/http.Client` (não `app.Test`) contra o `UserController`/`UserService` de exemplo, provando a cadeia inteira ponta a ponta.

**Atualizado na feature "Multipart Form Streaming" (Milestone 12):** achado importante -- `app.Test(req)` do Fiber (linha da matriz acima, "Dispatch de rota via Fiber real") NÃO serve pra provar comportamento de STREAMING: `Test` usa `httputil.DumpRequest(req, true)` internamente, que lê `req.Body` inteiro pra memória ANTES do `ServeConn` sequer rodar -- confirmado via leitura direta do source do `gofiber/fiber/v3`, não assumido. Qualquer teste que precise provar "bytes chegaram progressivamente, sem buffer completo antes" precisa do dial real (`Bind/Listen real`, linha abaixo), não do `app.Test` mais leve. A matriz de cobertura em si não ganhou linha nova (a rota multipart continua "Dispatch via Fiber real" pra correção funcional) -- só a prova ESPECÍFICA de streaming exige a camada mais pesada.

## Parallelism Assessment

| Package/Área | Parallel-Safe | Motivo |
| --- | --- | --- |
| Testes de builders isolados (Module/Provider/Controller sem bootstrap completo) | Sim | Sem estado global compartilhado entre casos |
| Testes do motor de resolução (Stage 1-3 rodando `NewApp` completo) | Não | Cada teste monta seu próprio `AppModule`/grafo — mas builders usam `var X = gonest.NewXxx(fn)` em nível de pacote nos exemplos reais; testes que reaproveitam os mesmos exemplos globais (não recriam struct a cada `t.Run`) correm risco de estado cruzado. Marcar como não-paralelo até o design confirmar que cada `NewApp` cria estado isolado por instância (sem registry global compartilhado entre bootstraps) |

**Nota:** revisar esse assessment quando "Module Composition" (feature seguinte) definir se builders (`NewModule` etc) retornam handles reutilizáveis entre múltiplos `NewApp` no mesmo processo (relevante pra rodar testes em paralelo sem vazar estado).
