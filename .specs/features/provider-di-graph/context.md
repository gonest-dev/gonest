# Provider & DI Graph Context

**Gathered:** 2026-07-12
**Spec:** `.specs/features/provider-di-graph/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Mecanismo de resolução de `Provider` via `MustResolve[T]`, com paralelismo (errgroup), 3 scopes (Singleton/Transient/Request) e semântica de visibilidade por módulo (não é container global único).

---

## Implementation Decisions

### Conflito de tipo entre módulos

- Dois `Provider`s podem registrar o mesmo tipo `T` em módulos diferentes sem conflito — cada `Module` tem seu próprio container de DI, isolado.
- Não é "última declaração vence" nem erro de bootstrap — é escopo legítimo por módulo, igual Nest.

### Visibilidade entre módulos (Export)

- Provider só é visível fora do módulo que o declara se o módulo fizer `module.Exports(XProvider)` explicitamente — igual `exports: []` do Nest.
- `module.Imports(OtherModule)` sozinho NÃO torna todo provider de `OtherModule` visível — só os exportados.
- API `Exports()` em si (sintaxe do builder de `Module`) é implementada na feature "Module Composition" (Milestone 1) — aqui na DI Graph entra a semântica de resolução que essa API precisa suportar desde o design inicial (não dá pra adicionar depois sem redesenhar o resolver).

### Escopo do MustResolve dentro de um módulo

- `MustResolve[T](builder)` recebe o builder (Controller/Provider) como argumento — framework usa isso pra saber a qual módulo esse builder pertence.
- Busca T primeiro no escopo do próprio módulo, depois nos módulos importados que exportam T. Não busca a partir da raiz (AppModule) pra baixo.

### Agent's Discretion

- Estrutura de dados interna do container por módulo (map, árvore, etc) — decisão de Design, sem preferência expressa do usuário.
- Mensagem exata de erro quando T não é exportado por nenhum módulo importado (distinta de "T não existe em lugar nenhum") — Design decide o texto, só precisa ser claro sobre a causa (não exportado vs não registrado).

---

## Specific References

Modelo mental é explicitamente "igual Nest": `exports: []` do `@Module()` decorator, resolução por escopo de módulo (não container global), constructor injection resolvendo peer/import chain.

---

## Deferred Ideas

- Sintaxe exata de `module.Exports(...)` (builder de Module) — pertence à feature "Module Composition" (Milestone 1), não a esta.
- Wiring de Request scope com o `Context` HTTP real — depende do Request Pipeline (Milestone 3), já registrado como P3 em spec.md com essa ressalva.
