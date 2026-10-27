# Design: Modernização Go 1.27 — Generic Methods & Convenção Parse/Safe

## Architecture Overview

A introdução de métodos genéricos em tipos concretos no Go 1.27 elimina o intermediário global de funções soltas e a necessidade de `owner any`. A nova arquitetura transfere a responsabilidade de injeção, parsing, mocking e binding diretamente para as structs donas do contexto.

```
┌─────────────────────────────────────────────────────────────┐
│                       gonest (root)                         │
│  - NewApp(root, opts...) *App                               │
│  - SafeNewApp(root, opts...) (*App, error)                  │
│  - NewController / NewProvider / NewModule / etc.           │
└──────────────────────────────┬──────────────────────────────┘
                               │
       ┌───────────────────────┼────────────────────────┐
       ▼                       ▼                        ▼
┌──────────────┐       ┌──────────────┐         ┌──────────────┐
│  Controller  │       │   Provider   │         │  Parseable   │
│  - Inject[T] │       │ - Inject[T]  │         │ - Parse[T]   │
│- InjectAll[T]│       │- InjectAll[T]│         │- SafeParse[T]│
│              │       │ - As[T]()    │         │              │
└──────────────┘       └──────────────┘         └──────────────┘
       │                       │                        │
       ▼                       ▼                        ▼
internal/inject         internal/module         internal/validate
```

## Dependency Paths (from nexspec)

- REQ-GEN-001 → `go.mod` + `PROJECT.md`
- REQ-GEN-002, REQ-GEN-003 → `internal/inject` → `Controller`/`Provider`/`Guard`/`Middleware`/`Interceptor`/`Filter`/`Listener`/`Resolver`
- REQ-GEN-004 → `internal/module` → `internal/provider.Provider.As[T]()`
- REQ-GEN-005 → `internal/execution.Parseable` → `internal/validate`
- REQ-GEN-006, REQ-GEN-007, REQ-GEN-008 → `internal/app` (`App`, `TestBuilder`, `Test`)
- REQ-GEN-009 → `internal/emitter.Emitter.Subscribe[T]()`
- REQ-GEN-010 → `internal/dotenv.Dotenv.Parse[T]()`
- REQ-GEN-011 → `../site/content/docs/**/*.mdx`

## New & Modified Components

| Component | Responsibility / Change | Location |
|---|---|---|
| `internal/inject` | Métodos `Inject[T]`, `InjectAll[T]`, `SafeInject[T]` desacoplados de `any` | `internal/inject/inject.go` |
| `internal/provider.Provider` | Adição de `.As[T]() ProviderRef` | `internal/provider/provider.go` |
| `internal/execution.Parseable` | De interface para struct concreta portando `Parse[T]` e `SafeParse[T]` | `internal/execution/request.go` |
| `internal/app.App` | `New` retorna `*App`, `SafeNewApp` retorna `(*App, error)`, `Listen`/`SafeListen` | `internal/app/app.go` |
| `internal/app.TestBuilder` | `Override[T](mock T) *TestBuilder` com encadeamento | `internal/app/test_app.go` |
| `internal/app.Test` | `Request(req)` direto e `SafeRequest(req)` | `internal/app/test_app.go` |
| `internal/emitter.Emitter` | `(e *Emitter) Subscribe[T](done)` método genérico | `internal/emitter/subscribe.go` |
| `gonest.go` | Limpeza de 8+ funções globais, unificação com novo padrão | `gonest.go` |

## Risks & Mitigation

- **Breaking Changes Massivos**: Afeta todos os 25 pacotes de testes e os `.examples/`.
  - *Mitigação*: Implementar pacote por pacote, garantindo compilação incremental e suite verde a cada estágio.
- **Parseable como struct**: As fontes em `internal/validate` implementavam a interface `Parseable`.
  - *Mitigação*: `execution.Parseable` agora envelopa a interface interna `intoSource` (`ParseInto(dst any, schema any) error`), preservando o desacoplamento entre `execution` e `validate`.

## Decision Log

- AD-068: Decisão confirmada de adotar a convenção `Parse` / `SafeParse` e descontinuar o prefixo `Must*` na API pública.
