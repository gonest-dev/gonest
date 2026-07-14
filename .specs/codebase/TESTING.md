# Testing

**Status:** greenfield — definido antes da 1ª feature (Provider & DI Graph), sem código Go ainda.

## Tooling

- Runner: `go test` (stdlib)
- Assertions: `github.com/stretchr/testify/assert` (sem mock framework por enquanto — feature de DI não precisa)
- Race detector: sempre ligado (`-race`) — relevante desde a 1ª feature por causa do `errgroup`/paralelismo no bootstrap

## Gate Check Commands

**Ambiente Windows local:** o processo que spawna cada shell injeta `CC=clang` (target MSVC), que quebra o build cgo do `-race` (`clang: error: unsupported option '-mthreads'`) — não vem de variável de ambiente User/Machine (essas ficaram vazias antes do B-001), então `go env -w` e `setx` não resolvem, só reiniciar a sessão do harness pegaria a variável persistida. MinGW-w64 já instalado (WinLibs, `winget install BrechtSanders.WinLibs.POSIX.UCRT`) — enquanto a sessão não reinicia, todo Gate command precisa do override inline abaixo.

| Gate | Command | Quando roda |
| --- | --- | --- |
| quick | `CC=gcc CXX=g++ PATH="/c/Users/Leandro/AppData/Local/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe/mingw64/bin:$PATH" go test ./... -race` | A cada task com só `unit` |
| full | mesmo comando acima (sem flag extra — `app.Test(req)` do Fiber não sobe porta real, roda dentro do `go test` normal, sem precisar de gate separado) | Task com `integration` + antes de fechar a feature inteira |

**Nota:** se uma sessão futura do harness já nascer sem `CC=clang` embutido (ex: depois de reiniciar o Claude Code), o prefixo `CC=gcc CXX=g++ PATH=...` vira redundante mas inofensivo — pode simplificar de volta pra `go test ./... -race` puro quando confirmar isso.

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
| Metadata/reflection puro (`internal/metadata` — identificação de campo via offset de ponteiro, builders `Metadata`/`PropertyBuilder`) | unit (sem dependência de DI graph/HTTP — domínio isolado, cada campo confirmado individualmente por nome+tipo, não só "compilou") |

**Atualizado na feature "Controller & Route Registration":** primeira camada e2e/integration do projeto — dispatch HTTP real via Fiber precisa provar que rota registrada responde corretamente, não só que a struct `Route` foi montada certo (isso é unit).

**Atualizado na feature "App Bootstrap & Listen":** segunda camada integration — agora cobrindo bind real de porta (`Listen`/`MustListen`/`OnListen`) e, no T6 final, um dial de verdade via `net/http.Client` (não `app.Test`) contra o `UserController`/`UserService` de exemplo, provando a cadeia inteira ponta a ponta.

## Parallelism Assessment

| Package/Área | Parallel-Safe | Motivo |
| --- | --- | --- |
| Testes de builders isolados (Module/Provider/Controller sem bootstrap completo) | Sim | Sem estado global compartilhado entre casos |
| Testes do motor de resolução (Stage 1-3 rodando `NewApp` completo) | Não | Cada teste monta seu próprio `AppModule`/grafo — mas builders usam `var X = gonest.NewXxx(fn)` em nível de pacote nos exemplos reais; testes que reaproveitam os mesmos exemplos globais (não recriam struct a cada `t.Run`) correm risco de estado cruzado. Marcar como não-paralelo até o design confirmar que cada `NewApp` cria estado isolado por instância (sem registry global compartilhado entre bootstraps) |

**Nota:** revisar esse assessment quando "Module Composition" (feature seguinte) definir se builders (`NewModule` etc) retornam handles reutilizáveis entre múltiplos `NewApp` no mesmo processo (relevante pra rodar testes em paralelo sem vazar estado).
