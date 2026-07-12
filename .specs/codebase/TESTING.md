# Testing

**Status:** greenfield — definido antes da 1ª feature (Provider & DI Graph), sem código Go ainda.

## Tooling

- Runner: `go test` (stdlib)
- Assertions: `github.com/stretchr/testify/assert` (sem mock framework por enquanto — feature de DI não precisa)
- Race detector: sempre ligado (`-race`) — relevante desde a 1ª feature por causa do `errgroup`/paralelismo no bootstrap

## Gate Check Commands

| Gate | Command | Quando roda |
| --- | --- | --- |
| quick | `go test ./... -race` | A cada task (não existe gate "leve" separado neste projeto — mesmo comando pra quick e full) |
| full | `go test ./... -race` | Antes de fechar a feature inteira |

## Test Coverage Matrix

| Code Layer | Test Type Required |
| --- | --- |
| Builders públicos (`NewModule`, `NewProvider`, `NewController`, `MustResolve`) | unit |
| Motor interno de resolução (`internal/resolver.go` — 3 estágios) | unit |
| Detecção de ciclo | unit |
| Erros de escopo/export (módulo não exporta, tipo não registrado) | unit |
| Nenhuma camada e2e/integration nesta feature | none |

## Parallelism Assessment

| Package/Área | Parallel-Safe | Motivo |
| --- | --- | --- |
| Testes de builders isolados (Module/Provider/Controller sem bootstrap completo) | Sim | Sem estado global compartilhado entre casos |
| Testes do motor de resolução (Stage 1-3 rodando `NewApp` completo) | Não | Cada teste monta seu próprio `AppModule`/grafo — mas builders usam `var X = gonest.NewXxx(fn)` em nível de pacote nos exemplos reais; testes que reaproveitam os mesmos exemplos globais (não recriam struct a cada `t.Run`) correm risco de estado cruzado. Marcar como não-paralelo até o design confirmar que cada `NewApp` cria estado isolado por instância (sem registry global compartilhado entre bootstraps) |

**Nota:** revisar esse assessment quando "Module Composition" (feature seguinte) definir se builders (`NewModule` etc) retornam handles reutilizáveis entre múltiplos `NewApp` no mesmo processo (relevante pra rodar testes em paralelo sem vazar estado).
