# Scheduler Specification

**Status: COMPLETE (2026-07-15, commit `34cb536`).** Cron expression parsing uses a new external dependency, `github.com/robfig/cron/v3` (`cronlib.ParseStandard`), rather than a hand-written parser -- correct 5-field cron parsing (ranges, steps, lists) is a well-solved problem not worth reinventing. Each of Cron/Interval/Timeout spawns its own background goroutine when called (during the Scheduler's own Declare, phase 2) -- no explicit Stop/cancellation mechanism exists (Out of Scope, unchanged from the original spec).

## Problem Statement

Milestone 10 (equivalente `@nestjs/schedule`, ver ROADMAP.md): jobs agendados (`Cron`/`Interval`/`Timeout`), cada execução isolada (recover próprio, não derruba o processo). INSIGHT.md's "# exemplo de Schedule" já especifica o call shape completo.

**Dependência bloqueante**: `NewScheduler(fn)` é mais um tipo `New*`-builder que chama `MustInject` dentro do próprio builder (`userService := gonest.MustInject[*UserService](scheduler)`) -- precisa do bootstrap de 3 fases especificado (não implementado ainda) em "Test App Bootstrap" (`.specs/features/test-app-bootstrap/`, ver AD-015 em STATE.md). Esta feature NÃO deve ser executada antes daquela.

## Goals

- [ ] `NewScheduler(fn func(scheduler *Scheduler)) *Scheduler` -- mesmo padrão `New*`-deferred de Provider/Controller (AD-015)
- [ ] `Module.Schedulers(...*Scheduler)` -- registro no bootstrap
- [ ] `scheduler.Cron(name string, expr string, fn func(ctx context.Context)) *Scheduler` -- agenda via expressão cron padrão (5 campos, formato `"0 0 * * *"`)
- [ ] `scheduler.Interval(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler` -- roda a cada `d`
- [ ] `scheduler.Timeout(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler` -- roda UMA vez, após `d`
- [ ] Cada execução (de QUALQUER dos 3 tipos) roda isolada: panic ou erro dentro do handler NUNCA derruba o processo, cai só no logger interno

## Out of Scope

| Feature | Reason |
| --- | --- |
| Cancelamento/pause de job já agendado em runtime | Nenhum exemplo pede isso |
| Cron com timezone customizado | INSIGHT.md's exemplo não especifica timezone -- assumir UTC/local do processo, mesma convenção de qualquer lib de cron padrão Go, sem API extra |
| Overlap policy (o que acontece se uma execução de Cron ainda está rodando quando a próxima dispara) | Não especificado em nenhum exemplo -- decisão de design pra quando especificar de verdade (design.md), não travada aqui |

---

## User Stories

### P1: Cron/Interval/Timeout, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: Como usuário do gonest, quero declarar um `Scheduler` com os 3 tipos de job (`Cron`/`Interval`/`Timeout`), cada um resolvendo sua própria dependência via `MustInject`, reproduzindo INSIGHT.md's `CleanupScheduler` verbatim.

**Acceptance Criteria**:

1. WHEN `scheduler.Cron(name, expr, fn)` é registrado THEN sistema SHALL disparar `fn` seguindo a expressão cron, indefinidamente, até o processo encerrar
2. WHEN `scheduler.Interval(name, d, fn)` é registrado THEN sistema SHALL disparar `fn` a cada `d`, indefinidamente
3. WHEN `scheduler.Timeout(name, d, fn)` é registrado THEN sistema SHALL disparar `fn` EXATAMENTE uma vez, após `d`
4. WHEN `fn` (de qualquer um dos 3) panica ou devolve erro THEN sistema SHALL capturar via recover próprio DAQUELA execução específica, logar internamente, NUNCA derrubar o processo nem impedir a PRÓXIMA execução agendada
5. WHEN `Module.Schedulers(s)` é chamado THEN `s`'s builder SHALL seguir o MESMO bootstrap de 3 fases que Guard/Middleware/Interceptor/Filter/Listener (AD-015)

**Independent Test**: reproduzir `CleanupScheduler` do INSIGHT.md verbatim (os 3 tipos), usando durações curtas o suficiente pra teste (ex: `Interval` de milissegundos em vez de `time.Minute`); confirmar cada tipo dispara conforme esperado, e que um handler panicando não impede a execução SEGUINTE do mesmo job.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| SC-01 | P1: Cron dispara seguindo expressão | Tasks | Pending |
| SC-02 | P1: Interval dispara repetidamente | Tasks | Pending |
| SC-03 | P1: Timeout dispara uma vez | Tasks | Pending |
| SC-04 | P1: isolamento de panic/erro por execução, não derruba processo nem futuras execuções | Tasks | Pending |

**ID format:** `SC-[NUMBER]`

**Coverage:** 4 total, 0 mapped yet.

---

## Success Criteria

- [ ] Exemplo do INSIGHT.md reproduzido end-to-end
- [ ] Zero regressões na suite existente

---

## Blocking Dependency

**Não executar antes de "Test App Bootstrap" (Milestone 8) estar implementado.** Mesma razão de Emitter -- `Scheduler` precisa do bootstrap de 3 fases (AD-015) pra `MustInject` dentro do seu builder funcionar.
