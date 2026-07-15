# Handoff

**Date:** 2026-07-14
**Feature:** Test App Bootstrap (Milestone 8, 1ª feature) — spec+context+design+tasks COMPLETOS, **EXECUÇÃO NÃO INICIADA** (parado a pedido explícito do usuário)
**Task:** Nenhuma em progresso. Próxima sessão: EXECUTAR `.specs/features/test-app-bootstrap/tasks.md`, começando por T0.

## ⚠️ Leia isto primeiro na próxima sessão

Esta feature acabou sendo a **maior mudança arquitetural planejada em toda a sessão** -- muito maior do que o "Test App Bootstrap" original do ROADMAP fazia parecer. Investigar como `MustOverride[Interface]` poderia funcionar revelou 4 achados em cadeia, cada um mudando o escopo:

1. DI por interface não existe hoje (`MustInject[T]` só aceita ponteiro).
2. O mecanismo de placeholder+copy-in-place que TODO `MustInject` usa genuinamente não generaliza pra interface (ponteiro é handle estável; valor de interface copiado não é).
3. Provider-a-Provider TAMBÉM usa esse mesmo mecanismo hoje (não é separado) -- `Provider` é um `module.Owner` válido.
4. Middleware/Guard/Interceptor/Filter rodam builder IMEDIATO (AD-008, desta MESMA sessão) -- tipicamente em `var X = gonest.NewGuard(...)` a nível de pacote, ANTES até de `NewApp` existir. Isso torna impossível "adiar pra depois" a resolução deles sem TAMBÉM reverter AD-008.

A solução que o usuário escolheu (depois de 3 rodadas de `AskUserQuestion`, ver `.specs/features/test-app-bootstrap/context.md` pra trilha completa): reordenar `NewApp` em 3 fases (Provider → Controller → Pipeline-stage types), revertendo AD-008 no processo, com um modelo de ownership novo (união dos módulos que referenciam um Guard/Middleware/etc, descoberto depois que Controllers terminam de declarar).

**Antes de despachar QUALQUER sub-agent de T0:**
1. Ler `.specs/features/test-app-bootstrap/context.md` INTEIRO (trilha de decisão completa).
2. Ler `.specs/features/test-app-bootstrap/spec.md` INTEIRO (9 requirements, TB-00 até TB-06).
3. Ler `.specs/features/test-app-bootstrap/design.md` INTEIRO -- **tem um aviso explícito no topo**: as assinaturas descritas são melhor-julgamento do orquestrador desta sessão, NÃO verificadas linha-a-linha contra o código atual. Releia o código de verdade (`internal/inject`, `internal/resolver`, `internal/provider`, `internal/controller`, `internal/app`, `internal/middleware`, `internal/guard`, `internal/interceptor`, `internal/filter`) antes de escrever qualquer código.
4. Ler `.specs/features/test-app-bootstrap/tasks.md` -- T0 já vem pré-quebrado em sub-passos (T0.a até T0.f); avaliar se cada um merece virar task própria com evaluator próprio, dado o tamanho.

## Completed ✓

- **Milestones 1-7 COMPLETE** — ver STATE.md pra histórico completo (AD-001 até AD-014).
- Nesta sessão, além do trabalho de código: INSIGHT.md ganhou a seção "# exemplo de MustInjectAll" (Animal/Cat/Dog), e a seção "exemplo de Testing" já documentava (mesmo antes desta investigação) que `UserController` precisa depender de `IUserService`, não `*UserService` -- ou seja, a necessidade de DI por interface já estava implícita no INSIGHT.md antes desta sessão descobrir que o mecanismo atual não suporta.

## In Progress

- Nada em execução agora -- ver aviso no topo.

## Pending

- **T0 até T4 de "Test App Bootstrap"** (ver tasks.md) -- T0 é o maior risco de todo o projeto até agora, maior que qualquer T0 anterior desta sessão (AD-012/AD-013 juntos).
- Depois: **"HTTP Test Client"** (2ª e última feature de Milestone 8, `MustRequest`/`AssertStatus`/`AssertJsonPath`) -- constrói em cima do que `MustNewTestApp` devolver, não especificado ainda.

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- Ver HANDOFFs anteriores pra lista completa.
- `internal/app/pipeline_ordering_test.go` ainda tem a modificação NÃO-relacionada (`c`→`ctrl`) não commitada -- ainda não resolvida, muitas sessões de ruído.
- Dev sub-agent de "Swagger UI Setup" reportou um bloqueio temporário de tooling (PATH lookup de `go` bloqueado por um classificador de segurança) contornado com caminho absoluto -- se isso se repetir em sessões futuras, considerar investigar a causa raiz em vez de recontornar toda vez.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado, pathspec explícito, `-m` antes de `--`. Working tree limpo (só `.vscode/` untracked).
- Fluxo de trabalho: ver STATE.md (AD-001 até AD-015).
- **Pra retomar**: ler o aviso no topo deste arquivo primeiro, depois STATE.md inteiro (AD-015 é a decisão mais recente e mais importante), depois os 4 arquivos de `.specs/features/test-app-bootstrap/` na ordem context→spec→design→tasks, DEPOIS o código atual de verdade -- só então começar a despachar T0 (ou T0.a se decidir quebrar mais).
