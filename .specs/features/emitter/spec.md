# Emitter Specification

**Status: COMPLETE (2026-07-15, commit `1e08298`).** `Emitter`'s "resolve from any module, no registration" requirement (spec's own Goal, not resolved by design.md since none was written) solved via a new generic mechanism in `internal/inject`: `RegisterGlobalSingleton`/`GlobalSingletonFor`, checked by `MustInject[T]` BEFORE the existing `directResolver`/placeholder dispatch. `internal/app`'s `NewApp`/`MustNewTestApp` register a fresh `*emitter.Emitter` this way at the very start of every bootstrap (alongside `inject.Reset()`). `Listener` ended up following `Controller`'s single-module-ownership pattern (registered via `Module.Listeners`, declared in phase 2), not the union-of-referencing-modules pattern Middleware/Guard/Interceptor/Filter use -- `Module.Listeners(l)` is a direct per-module registration, same level as `Module.Providers`/`Controllers`, so there's no multi-controller-reference ambiguity to resolve.

## Problem Statement

Milestone 9 (equivalente `@nestjs/event-emitter`, ver ROADMAP.md): evento tipado por struct (não string solta), listener registrado via `Module.Listeners()`, emissão assíncrona fire-and-forget via um `Emitter` global (singleton do framework, sempre disponível). INSIGHT.md's "# exemplo de Emitter" já especifica o call shape completo (corrigido nesta sessão -- ver nota abaixo).

**Dependência bloqueante**: `NewListener(fn)` é mais um tipo `New*`-builder que chama `MustInject`/`MustOn` dentro do próprio builder -- precisa do bootstrap de 3 fases especificado (não implementado ainda) em "Test App Bootstrap" (`.specs/features/test-app-bootstrap/`, ver AD-015 em STATE.md) pra funcionar corretamente. Esta feature NÃO deve ser executada antes daquela.

## Correção feita nesta sessão

INSIGHT.md's exemplo original tinha `provider.Constructor(func(emitter *gonest.Emitter) *UserService {...})` -- um parâmetro de dependência DIRETO no `Constructor`. Isso é INCOMPATÍVEL com o mecanismo real de `Provider` (confirmado durante a investigação de "Test App Bootstrap", context.md's Discovery 3): `Constructor` só aceita `func()`/`func()(T,error)`/`func(ctx)T`/`func(ctx)(T,error)` -- NUNCA parâmetro de dependência. A dependência é capturada via `MustInject` chamado DENTRO do builder do Provider, ANTES de `provider.Constructor(...)` ser chamado (mesmo padrão de Guard/Scheduler/HealthCheck). Corrigido no INSIGHT.md nesta sessão -- ver o exemplo atual.

## Goals

- [ ] `gonest.Emitter` -- tipo singleton do FRAMEWORK (não um Provider que o usuário registra) -- `MustInject[*gonest.Emitter](owner)` deve funcionar em QUALQUER módulo, sem `module.Providers(EmitterProvider)` explícito em lugar nenhum
- [ ] `NewListener(fn func(listener *Listener)) *Listener` -- mesmo padrão `New*`-deferred de Provider/Controller (per AD-015, execução adiada até a fase certa do bootstrap)
- [ ] `MustOn[EventType any](listener *Listener, handler func(ctx context.Context, event EventType))` -- função LIVRE (não método -- Go não permite parâmetro de tipo em método, L-001 em STATE.md), registra `handler` pro tipo `EventType`
- [ ] `Module.Listeners(...*Listener)` -- registro no bootstrap, mesmo nível de `Module.Providers()`
- [ ] `Emitter.Emit(event any)` -- assíncrono: dispara 1 goroutine POR listener registrado pro tipo EXATO de `event`, retorna imediatamente (fire-and-forget), NUNCA bloqueia o chamador
- [ ] Panic ou erro dentro de um listener NUNCA propaga pro chamador de `Emit` -- cai só no logger interno do framework (mesmo comportamento isolado que Scheduler terá, ver Milestone 10)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Emit síncrono (esperar todos os listeners terminarem) | INSIGHT.md's comentário é explícito: "não bloqueia quem chamou Emit" -- fire-and-forget é o único modo especificado |
| Listener com prioridade/ordem garantida entre múltiplos listeners do MESMO evento | Nenhum exemplo pede isso -- goroutines concorrentes, ordem de execução não é garantida nem precisa ser |
| Wildcard/pattern matching de eventos (ex: `"user.*"`) | Evento é tipado por struct Go, não string -- não existe conceito de wildcard nesse modelo |

---

## User Stories

### P1: Emitter + Listener, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: Como usuário do gonest, quero declarar um evento tipado (`UserCreatedEvent`), um listener via `MustOn`, e emitir de dentro de um Provider via `gonest.MustInject[*gonest.Emitter]`, reproduzindo INSIGHT.md's exemplo completo.

**Acceptance Criteria**:

1. WHEN `Emitter.Emit(event)` é chamado THEN sistema SHALL disparar 1 goroutine por listener registrado (via `MustOn[T]`) cujo `T` bate EXATAMENTE com o tipo de `event`, e retornar imediatamente
2. WHEN um listener panica ou seu handler devolve erro (se essa forma existir) THEN sistema SHALL capturar via recover próprio por goroutine, logar internamente, NUNCA propagar pro chamador de `Emit`
3. WHEN `MustInject[*gonest.Emitter](owner)` é chamado em QUALQUER módulo, sem registro explícito de Emitter em lugar nenhum THEN sistema SHALL resolver com sucesso (singleton do framework, sempre presente)
4. WHEN `Module.Listeners(l)` é chamado THEN `l`'s builder (via `NewListener`) SHALL rodar seguindo o MESMO bootstrap de 3 fases que Guard/Middleware/Interceptor/Filter (AD-015) -- `MustOn`/`MustInject` dentro dele resolvem direto, sem placeholder

**Independent Test**: reproduzir o exemplo `UserCreatedEvent`/`UserCreatedListener`/`UserProvider` do INSIGHT.md verbatim; emitir o evento, confirmar o listener roda (via canal/sync de teste, já que é assíncrono) com o payload correto; confirmar listener panicando não derruba o teste/processo.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| EM-01 | P1: Emitter é singleton do framework, sempre injetável | Tasks | Pending |
| EM-02 | P1: NewListener + MustOn seguem bootstrap de 3 fases | Tasks | Pending |
| EM-03 | P1: Emit assíncrono, fire-and-forget, isolamento de panic por listener | Tasks | Pending |

**ID format:** `EM-[NUMBER]`

**Coverage:** 3 total, 0 mapped yet.

---

## Success Criteria

- [ ] Exemplo do INSIGHT.md reproduzido end-to-end
- [ ] Zero regressões na suite existente

---

## Blocking Dependency

**Não executar antes de "Test App Bootstrap" (Milestone 8, `.specs/features/test-app-bootstrap/`) estar implementado.** `Listener` precisa do mecanismo de bootstrap de 3 fases + ownership por união de módulos (AD-015) pra `MustInject`/`MustOn` dentro do seu builder funcionarem corretamente.
