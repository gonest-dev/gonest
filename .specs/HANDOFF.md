# Handoff

**Date:** 2026-07-14
**Feature:** Roadmap v1 (Milestones 1-8) TOTALMENTE ESPECIFICADO + Milestones 9-11 (Emitter/Scheduler/Terminus) promovidos e especificados. **NENHUMA execução de código pendente foi iniciada** -- todo trabalho desta última parte da sessão foi documentação/planejamento, a pedido explícito do usuário.
**Task:** Nenhuma em progresso. Próxima sessão: EXECUTAR, começando por "Test App Bootstrap" T0.

## ⚠️ Leia isto primeiro na próxima sessão

Toda a última parte desta sessão foi ESPECIFICAÇÃO PURA, sem tocar em código. Ordem de dependência das 5 features especificadas-mas-não-executadas:

```
Test App Bootstrap (Milestone 8, 1ª feature)  ← EXECUTAR PRIMEIRO, bloqueia tudo abaixo
  ├── HTTP Test Client (Milestone 8, 2ª feature)
  ├── Emitter (Milestone 9)
  ├── Scheduler (Milestone 10)
  └── Terminus/health (Milestone 11)
```

Todas as 4 dependentes usam o MESMO motivo de bloqueio: são tipos `New*`-builder (`Listener`/`Scheduler`/`HealthCheck`, mesma família de `Guard`/`Middleware`/`Interceptor`/`Filter`) que chamam `MustInject` dentro do próprio builder -- só funcionam corretamente com o bootstrap de 3 fases especificado em "Test App Bootstrap" (AD-015, STATE.md).

**Comece por**: `.specs/features/test-app-bootstrap/context.md` → `spec.md` → `design.md` (tem aviso explícito de que as assinaturas são melhor-julgamento, releia o código de verdade antes de codificar) → `tasks.md` (T0 já vem pré-quebrado em T0.a-T0.f).

## Completed ✓ (documentação/spec, não código)

- **Milestones 1-7 COMPLETE** (código real, ver AD-001 até AD-014 em STATE.md).
- **"Test App Bootstrap"** (Milestone 8, 1ª feature) -- spec+context+design+tasks completos. Maior mudança arquitetural planejada na sessão: bootstrap de 3 fases + reversão de AD-008 + DI por interface. Ver AD-015 em STATE.md.
- **"HTTP Test Client"** (Milestone 8, 2ª feature) -- spec.md escrito, deliberadamente SEM design.md detalhado (depende de "Test App Bootstrap" estar implementado de verdade primeiro -- flagado um gap de design real: `HttpAdapter` hoje não tem método de dispatch in-memory, só `Init`/`RegisterRoute`/`Listen`).
- **"Emitter"** (Milestone 9, NOVA) -- spec.md escrito. Corrigi um bug real no exemplo do INSIGHT.md (`provider.Constructor(func(emitter *gonest.Emitter) *UserService{...})` -- parâmetro de dependência direto no Constructor, incompatível com o mecanismo real de Provider confirmado durante a pesquisa de "Test App Bootstrap"). Exemplo corrigido pra usar `MustInject` dentro do builder do Provider.
- **"Scheduler"** (Milestone 10, NOVA) -- spec.md escrito, baseado no exemplo já existente do INSIGHT.md.
- **"Terminus/health"** (Milestone 11, NOVA) -- spec.md escrito, baseado no exemplo já existente do INSIGHT.md.
- ROADMAP.md atualizado: Milestones 9-11 promovidos de "Future Considerations" pra Milestones de verdade (com Status PLANNED explícito). Multi-adapter/CLI/microservices continuam em Future Considerations (sem exemplo concreto, não especificados).

## In Progress

- Nada em execução agora.

## Pending (ordem de execução recomendada)

1. **"Test App Bootstrap" T0-T4** (`.specs/features/test-app-bootstrap/tasks.md`) -- maior risco do projeto até agora.
2. **"HTTP Test Client"** -- especificar `design.md` PRÓPRIO assim que "Test App Bootstrap" fechar (spec.md já existe, mas foi deliberadamente deixado sem design.md detalhado, aguardando implementação real de `Tester`/`HttpAdapter`).
3. **"Emitter"**, **"Scheduler"**, **"Terminus/health"** -- podem ser especificadas com `design.md`/`tasks.md` próprios e executadas em qualquer ordem entre si, uma vez "Test App Bootstrap" estiver pronto (todas dependem só dele, não umas das outras).

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- Ver HANDOFFs anteriores pra lista completa.
- `internal/app/pipeline_ordering_test.go` ainda tem a modificação NÃO-relacionada (`c`→`ctrl`) não commitada -- ainda não resolvida, muitas sessões de ruído.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado, pathspec explícito, `-m` antes de `--`. Working tree limpo (só `.vscode/` untracked).
- Fluxo de trabalho: ver STATE.md (AD-001 até AD-015).
- **Pra retomar**: ler o aviso no topo deste arquivo, depois STATE.md inteiro, depois `.specs/features/test-app-bootstrap/` na ordem context→spec→design→tasks, DEPOIS o código atual de verdade -- só então começar T0.
