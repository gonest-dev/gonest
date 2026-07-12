# Provider & DI Graph Specification

## Problem Statement

Dev NestJS espera `@Injectable()` + constructor injection resolvendo automaticamente, com singletons compartilhados e suporte a escopo por instância/por-request quando precisar. Go não tem container DI nativo nem decorators — `gonest.NewProvider` + `gonest.MustResolve[T]` precisam entregar essa mesma ergonomia, com resolução paralela (equivalente ao `Promise.all` do Nest) e sem reflect mágico onde builder explícito resolver.

## Goals

- [ ] `MustResolve[T]` devolve ponteiro já populado, sem o chamador esperar manualmente (grafo já resolvido por `NewApp` antes de retornar)
- [ ] Providers sem dependência entre si resolvem em paralelo (goroutine + `errgroup`), medido: tempo de bootstrap com N providers independentes ≈ tempo do mais lento, não soma de todos
- [ ] 3 scopes suportados no v1: Singleton, Transient, Request

## Out of Scope

| Feature | Reason |
| --- | --- |
| Module/Controller/Route registration | Feature separada (Milestone 1: "Module Composition", "Controller & Route Registration") |
| Interceptor/Guard/Middleware consumindo provider de Request scope | Depende do Request Pipeline (Milestone 3) — aqui só o mecanismo de scope, não o wiring com HTTP |
| Multi-adapter HTTP | Fora de escopo do v1 inteiro (ver PROJECT.md) |

---

## User Stories

### P1: Singleton Provider Resolution ⭐ MVP

**User Story**: Como dev gonest, quero declarar um `Provider` com `Constructor` e resolvê-lo via `MustResolve[T]` em qualquer outro `Provider`/`Controller`, com uma única instância compartilhada pra todo o app.

**Why P1**: sem isso não existe DI — é o alicerce de todo o resto do framework (Controller, Module, pipeline dependem de resolver dependências).

**Acceptance Criteria**:

1. WHEN `NewProvider` declara `Scope(gonest.ScopeSingleton)` + `Constructor(func() *T {...})` THEN `NewApp` SHALL instanciar exatamente uma vez e reusar a mesma instância em todo `MustResolve[T]` subsequente
2. WHEN dois providers (A, B) não têm dependência entre si THEN `NewApp` SHALL resolver ambos em paralelo (goroutines distintas via `errgroup`)
3. WHEN provider A depende de provider B (`Constructor(func(b *B) *A {...})`) THEN `NewApp` SHALL esperar B terminar antes de rodar o `Constructor` de A
4. WHEN `Constructor` aceita `context.Context` THEN `NewApp` SHALL injetar um `ctx` com timeout de bootstrap configurado, suportando cancelamento
5. WHEN `Constructor` retorna `error` não-nil OU panica THEN `NewApp` SHALL cancelar as demais goroutines (via `context.Context`) e `NewApp`/`MustApp` SHALL retornar/panicar com esse erro, sem subir o servidor
6. WHEN `MustResolve[T]` é chamado em fase de declaração (dentro do builder de outro Provider/Controller) THEN SHALL devolver um ponteiro placeholder que é populado (`*placeholder = *real`) quando a resolução real terminar, e esse mesmo ponteiro SHALL estar totalmente populado no momento em que `NewApp` retorna

**Independent Test**: declarar 3 providers (2 independentes + 1 dependente de um deles), medir que bootstrap não serializa os 2 independentes; `MustResolve` de cada um devolve a mesma instância em chamadas repetidas.

---

### P2: Transient Scope

**User Story**: Como dev gonest, quero declarar `Scope(gonest.ScopeTransient)` pra obter uma instância nova a cada `MustResolve[T]`, equivalente ao `Scope.TRANSIENT` do Nest.

**Why P2**: necessário pra casos onde estado não pode ser compartilhado (ex: builder com estado mutável por uso), mas não bloqueia o MVP do DI básico.

**Acceptance Criteria**:

1. WHEN provider é `ScopeTransient` THEN cada `MustResolve[T]` SHALL rodar `Constructor` novamente e devolver uma instância nova
2. WHEN provider transient depende de provider singleton THEN a dependência singleton SHALL ser reusada (não recriada) a cada resolução do transient

**Independent Test**: resolver o mesmo provider transient duas vezes, comparar ponteiros (devem ser diferentes); resolver a dependência singleton dele duas vezes, comparar ponteiros (devem ser iguais).

---

### P3: Request Scope

**User Story**: Como dev gonest, quero declarar `Scope(gonest.ScopeRequest)` pra obter uma instância nova por request HTTP, compartilhada entre tudo que resolve esse provider dentro da mesma request.

**Why P3**: paridade com `Scope.REQUEST` do Nest, mas depende do ciclo de vida de request que só existe de fato quando o Request Pipeline (Milestone 3) estiver implementado — aqui entra só o mecanismo de registro/scope, o wiring completo com `Context` de request fica marcado como dependência.

**Acceptance Criteria**:

1. WHEN provider é `ScopeRequest` THEN o mecanismo de scope SHALL existir e ser testável isoladamente (ex: via um "request context" mockado), mesmo sem o Pipeline completo ainda existir
2. WHEN duas resoluções acontecem dentro do mesmo "request context" THEN SHALL devolver a mesma instância
3. WHEN duas resoluções acontecem em "request contexts" diferentes THEN SHALL devolver instâncias diferentes

**Independent Test**: simular dois "request contexts" distintos resolvendo o mesmo provider request-scoped, confirmar instâncias diferentes entre contexts e iguais dentro do mesmo context.

---

## Edge Cases

- WHEN existe dependência circular entre providers (A→B→A) THEN `NewApp` SHALL detectar o ciclo antes de tentar resolver e retornar erro claro (ex: `"circular dependency: A -> B -> A"`), nunca deadlock silencioso
- WHEN `MustResolve[T]` é chamado pra um tipo `T` que nenhum `Provider` registrou THEN SHALL panicar com mensagem clara identificando o tipo não encontrado
- WHEN múltiplos providers em módulos diferentes registram o mesmo tipo `T` THEN SHALL coexistir sem conflito — cada `Module` tem container de DI isolado (ver context.md)
- WHEN `MustResolve[T](builder)` é chamado dentro de um Controller/Provider de um módulo M THEN SHALL procurar T primeiro no escopo de M, depois nos módulos importados por M que exportam T (`module.Exports(...)`) — nunca a partir da raiz do app
- WHEN T existe em outro módulo mas esse módulo NÃO o exporta THEN `MustResolve[T]` SHALL panicar com mensagem que distingue "não exportado" de "não registrado em lugar nenhum"
- WHEN `Constructor` de um provider independente falha enquanto outros ainda resolvem em paralelo THEN os demais SHALL ser cancelados via `context.Context`, não continuar rodando em background após `NewApp` retornar erro

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| DI-01 | P1: Singleton Provider Resolution | Design | Pending |
| DI-02 | P1: Resolução paralela via errgroup | Design | Pending |
| DI-03 | P1: Constructor com context.Context (timeout bootstrap) | Design | Pending |
| DI-04 | P1: Falha em Constructor cancela demais goroutines | Design | Pending |
| DI-05 | P1: Placeholder + copy-in-place no MustResolve | Design | Pending |
| DI-06 | P2: Transient scope | Design | Pending |
| DI-07 | P3: Request scope (mecanismo isolado, sem pipeline) | Design | Pending |
| DI-08 | Edge: detecção de ciclo | Design | Pending |
| DI-09 | Edge: MustResolve de tipo não registrado → panic | Design | Pending |
| DI-10 | Edge: resolução escopada por módulo + Export | Design | Pending |

**Coverage:** 10 total, 0 mapped to tasks, 10 unmapped ⚠️ (aguardando fase Design/Tasks)

---

## Success Criteria

- [ ] Exemplo `UserProvider`/`UserService` do INSIGHT.md compila e resolve via `MustResolve[*UserService]`
- [ ] Teste de bootstrap com providers independentes demonstra paralelismo real (não serializado)
- [ ] Ciclo de dependência produz erro determinístico, não deadlock/timeout
