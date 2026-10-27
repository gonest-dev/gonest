# Spec: Modernização Go 1.27 — Generic Methods & Convenção Parse/Safe

## Summary

Com o lançamento do Go 1.27, o Go passa a suportar nativamente métodos genéricos em tipos/receivers concretos (`func (r *Receiver) Method[T any](...)`). A restrição arquitetural histórica formalizada em `PROJECT.md` ("Go não tem parâmetro de tipo em método") deixa de existir. Esta feature moderniza a API pública do GoNest de ponta a ponta sem retrocompatibilidade:
1. Elimina as funções genéricas globais de `gonest.go` (`MustInject`, `MustInjectAll`, `ProviderAs`, `MustOverride`, `Subscribe`, `Parse`, `MustParse`), substituindo-as por métodos diretos e fluentes nos respectivos tipos (`c.Inject[T]()`, `p.As[T]()`, `b.Override[T]()`, `e.Subscribe[T]()`, etc.).
2. Adota o padrão moderno **Parse / SafeParse** (inspirado no ecossistema Zod/NestJS), eliminando o ruído ubíquo do prefixo `Must*`: o método sem prefixo (`Parse`, `NewApp`, `Listen`, `SetupSwagger`, `Request`) executa o fluxo principal e propaga exceções/panics estruturados, enquanto o prefixo `Safe*` (`SafeParse`, `SafeNewApp`, `SafeListen`, `SafeRequest`) devolve a tupla com erro explícito `(T, error)` para quem deseja controle manual.
3. Sincroniza obrigatoriamente o repositório irmão de documentação `../site` com as novas assinaturas e convenções nos 3 idiomas (en, pt, es), seguido de commit e push em ambos os repositórios.

## Requirements

- REQ-GEN-001: **Go 1.27 Runtime Baseline** — A diretiva `go` em `go.mod` é atualizada para `go 1.27.0`. A restrição técnica em `.specs/project/PROJECT.md` referente à ausência de parâmetros de tipo em métodos é revogada.
- REQ-GEN-002: **DI Builder Generic Methods** — Os tipos de ciclo de vida (`*Controller`, `*Provider`, `*Middleware`, `*Guard`, `*Interceptor`, `*Filter`, `*Listener`, `*GraphqlResolver`, `*LazyModule`, `*TestApp`) recebem métodos genéricos diretos:
  - `Inject[T any]() T` (resolução direta por tipo ponteiro ou interface; falha alto em erro de injeção).
  - `InjectAll[T any]() []T` (resolução multi-binding para slices de interfaces).
  - `SafeInject[T any]() (T, error)` (resolução não-panificante retornando erro descritivo).
- REQ-GEN-003: **Morte de Funções Globais de DI** — As funções globais `gonest.MustInject[T any](owner any) T` e `gonest.MustInjectAll[T any](owner any) []T` são removidas do pacote raiz, eliminando o boxing `any` e validações em tempo de execução de owner.
- REQ-GEN-004: **Provider Interface Binding** — O tipo `*Provider` recebe o método fluente `(p *Provider) As[T any]() ProviderRef`. A função global `gonest.ProviderAs[T any](ref ProviderRef)` é removida.
- REQ-GEN-005: **Padrão Parse / SafeParse em Request Sources** — O tipo `Parseable` passa a ser uma struct concreta (ou expor métodos genéricos concretos) com:
  - `(p Parseable) Parse[T any](s *Schema) T` (valida e desserializa; lança `*BadRequestException` em caso de erro, formatado pelo pipeline de filters).
  - `(p Parseable) SafeParse[T any](s *Schema) (T, error)` (retorna tupla `(T, error)` sem lançar panic/exception).
  - As funções globais `gonest.Parse` e `gonest.MustParse` são removidas.
- REQ-GEN-006: **Inversão de Bootstrap (NewApp vs SafeNewApp)**:
  - `gonest.NewApp(root *Module, opts ...AppOptions) *App` passa a ser a função padrão direta (panica se houver erro de ciclo, validação ou configuração).
  - `gonest.SafeNewApp(root *Module, opts ...AppOptions) (*App, error)` é introduzido para fluxos que requerem inspeção explícita de erro.
  - `gonest.MustNewApp` é removido.
- REQ-GEN-007: **App Lifecycle (Listen & SetupSwagger)**:
  - `(*App).Listen(addr string, onListen ...OnListen)` panica em falha de bind/listen.
  - `(*App).SafeListen(addr string, onListen ...OnListen) error` retorna o erro diretamente.
  - `SetupSwagger(app *App, ...)` / `(*App).SetupSwagger(...)` torna-se direto (panica se a rota não puder ser registrada).
  - `SafeSetupSwagger(...) error` retorna o erro diretamente.
  - Métodos `MustListen` e `MustSetupSwagger` são removidos.
- REQ-GEN-008: **Test Harness (NewTestApp, Override, Request)**:
  - `gonest.NewTestApp(root *Module, configure func(*TestBuilder)) *TestApp` retorna `*TestApp` direto (substituindo `MustNewTestApp`).
  - `(*TestBuilder).Override[T any](mockValue T) *TestBuilder` torna-se método fluente de `*TestBuilder` (substituindo `MustOverride`).
  - `(*TestApp).Request(req *http.Request) *TestResponse` panica em falha interna de dispatch (substituindo `MustRequest`).
  - `(*TestApp).SafeRequest(req *http.Request) (*TestResponse, error)` introduzido para testes de falha de transporte.
- REQ-GEN-009: **Emitter Subscriptions** — O tipo `*Emitter` recebe o método genérico direto `(e *Emitter) Subscribe[EventType any](done <-chan struct{}) <-chan EventType`. A função global `gonest.Subscribe` é removida.
- REQ-GEN-010: **Dotenv Parsing** — O tipo `*Dotenv` recebe os métodos `(d *Dotenv) Parse[T any](s *Schema) T` e `(d *Dotenv) SafeParse[T any](s *Schema) (T, error)`.
- REQ-GEN-011: **Sincronização do Site (`../site`) & Multi-repo Push** — Todos os arquivos em `../site/content/docs/` que referenciam APIs alteradas (`MustInject`, `MustParse`, `MustNewApp`, `MustListen`, etc.) são atualizados nos 3 idiomas (`.mdx`, `.pt.mdx`, `.es.mdx`). Ambas as árvores de trabalho (`gonest` e `../site`) passam pelos gates de testes e recebem commit e push limpos.

## Affected Components (from graph)

- `gonest.go` ⚠️ high fan-in — Remoção de 8+ funções globais `Must*` e exportação dos novos tipos/métodos.
- `internal/inject/inject.go` — Adição dos métodos `Inject[T]`, `InjectAll[T]`, `SafeInject[T]` nas structs proprietárias de DI (`Controller`, `Provider`, `Middleware`, etc.).
- `internal/execution/request.go` & `internal/validate/` — Transformação de `Parseable` em struct concreta que encapsula `ParseInto` e expõe `Parse[T]` / `SafeParse[T]`.
- `internal/provider/provider.go` — Adição de `(p *Provider) As[T any]() ProviderRef`.
- `internal/app/app.go` & `internal/app/test_app.go` — Refatoração de `NewApp`/`SafeNewApp`, `Listen`/`SafeListen`, `NewTestApp`, `TestBuilder.Override`.
- `internal/emitter/subscribe.go` & `internal/emitter/emitter.go` — Adição de `Subscribe[T]` como método em `*Emitter`.
- `internal/dotenv/dotenv.go` — Adição de `Parse[T]` / `SafeParse[T]`.
- Todo o conjunto de testes (`*_test.go`) em 25 pacotes e os exemplos em `.examples/*`.
- Repositório de documentação `../site` (`content/docs/**/*.{mdx,pt.mdx,es.mdx}`).

## Out of Scope

- Introdução de métodos genéricos em interfaces Go (o compilador Go 1.27 não permite type parameters em assinaturas de interfaces).
- Criação de novos adapters HTTP além do Fiber.
- Alteração da lógica interna de resolução topológica de Stage 1-3 (o grafo de resolução continua o mesmo, mudando apenas a superfície de chamada).

## Open Questions

Nenhuma pergunta aberta. A decisão de não manter retrocompatibilidade e adotar a nova ergonomia de `Parse` / `SafeParse` foi confirmada.
