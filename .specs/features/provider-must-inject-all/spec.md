# Spec: Provider-side MustInjectAll

## Summary

Hoje `gonest.MustInjectAll[T](owner)` (`internal/inject.MustAll`) só funciona quando `owner`
satisfaz `directResolver` (`*controller.Controller`, `*middleware.Middleware`, `*guard.Guard`,
`*interceptor.Interceptor`, `*filter.Filter`) -- resolução acontece DIRETO porque, nesse ponto do
bootstrap (phase 2/3), os Providers que essas peças dependem já foram construídos por Stage 3.
`*provider.Provider` é explicitamente rejeitado (`panic("... is not supported from this owner")`)
porque um Provider é declarado durante Stage 2 (fase de `Declare`), ANTES de Stage 3 resolver
qualquer valor real -- não existem instâncias pra devolver ainda, só a árvore de módulos
finalizada (Stage 1 `Assemble` já rodou nesse ponto).

Esta feature reformula `MustInjectAll[T]` pra também aceitar `owner *provider.Provider`,
usando o MESMO mecanismo de placeholder+edge diferido que `MustInject[T]` (ponteiro único) já
usa pra Provider-a-Provider -- generalizado pra multi-valor via um slice de comprimento fixo,
alocado no momento da chamada (já é possível contar QUANTOS Providers no escopo implementam `T`,
porque a árvore de módulos já está montada), com cada elemento escrito in-place por Stage 3
quando o Provider correspondente termina de resolver. Motivado pelo exemplo real documentado em
`INSIGHT-MUST-INJECT-ALL.md` (raiz do repo): um `HealthUsecase` que quer injetar TODOS os
`port.Pingable` do grafo (vários adapters de infra, cada um em seu próprio módulo/Provider) sem
precisar de um `Module.Providers([]port.Pingable{...})` manual construído fora do DI.

## Requirements

- REQ-001: `gonest.MustInjectAll[T](p gonest.Provider)` (T = tipo interface) retorna `[]T`
  DENTRO do builder fn de `gonest.NewProvider`, análogo ao uso já suportado a partir de
  Controller/Middleware/Guard/Interceptor/Filter.
- REQ-002: O slice retornado é alocado com o comprimento final JÁ CORRETO no momento da chamada
  (structural match contra a árvore de módulos assemblada, mesma lógica de precedência de
  `findDirectMatches`/`Find` -- próprio módulo antes de imports), populado ELEMENTO A ELEMENTO
  por Stage 3 conforme cada Provider casado termina de resolver. A variável Go que recebe o
  retorno (capturada por um `p.Constructor(func() T {...})` fechado sobre ela) enxerga os valores
  reais assim que Stage 3 os grava -- sem exigir um passo extra de "commit" do chamador.
- REQ-003: Zero Providers no escopo implementando `T` retorna slice vazio (comprimento 0), nunca
  panic -- mesmo contrato que `MustInjectAll` via `directResolver` já tem hoje.
- REQ-004: A ordem dos elementos no slice NÃO é garantida (documentar explicitamente, sem
  compromisso de estabilidade).
- REQ-005: Só Provider `scope.Singleton` pode casar como membro do slice. Um Provider
  `scope.Transient` que implementaria `T` e estaria no escopo faz `NewApp`/`MustNewApp` panicar
  alto (fail-fast na construção do grafo, não silenciosamente ignorado) -- mesma postura de
  `mustLazy`'s LAZY-07 (`ScopeSingleton` obrigatório), mesmo motivo: Stage 3 só resolve um
  Provider Singleton exatamente 1 vez (`invokeAndCopy`), Transient tem fan-out por edge
  (`invokeAndCopyEdge`) incompatível com "1 slot fixo no slice compartilhado" -- fora de escopo
  nesta versão (não há caso de uso real hoje: `INSIGHT-MUST-INJECT-ALL.md` só cobre health-check
  agregando adapters Singleton).
- REQ-006: `T` deve ser `Kind() == reflect.Interface` (mesma exigência de `MustAll` hoje) --
  panic caso contrário, mesma mensagem/formato de erro já usado.
- REQ-007: Ciclo/ordenação: o grafo de dependência de Stage 3 (`BuildGraph`/`scopedGraph`) passa
  a incluir uma aresta owner→matched PARA CADA Provider casado (não uma aresta única pro tipo
  interface) -- `waitDeps` do owner (o Provider que chamou `MustInjectAll`) só libera depois que
  TODOS os Providers casados tiverem terminado. Detecção de ciclo (`DetectCycle`) continua
  funcionando sem mudança de algoritmo, já que essas arestas são arestas normais no mesmo grafo
  node-a-node.
- REQ-008: Nenhuma mudança de comportamento pro caminho `directResolver` já existente
  (`MustInjectAll` via Controller/Middleware/Guard/Interceptor/Filter) -- dispatch continua
  idêntico, só ganha um branch novo pra `owner` ser `*provider.Provider`.

## Affected Components

(Sem grafo indexado ainda neste projeto -- `.specs/graph/graph.json` ausente, análise via leitura
direta de código, modo degradado do skill.)

- `internal/inject/inject.go` -- `Must[T]`/`MustAll[T]` (dispatch), `PendingEdge` (hoje só
  suporta 1 target por edge, pointer-only) precisa de uma contraparte multi-valor: novo tipo
  registrando `{Owner, InterfaceType, Slice reflect.Value, Matches []module.ProviderRef}`
  (nome sugerido: `PendingAllEdge`), + `pendingAllEdges`/`PendingAllEdges()`/limpeza em `Reset()`.
- `internal/resolver/stage3.go` -- ponto central do reformulação:
  - `scopedGraph`/`BuildGraph` (provavelmente em `resolver.go` ou `graph.go`, não lido ainda)
    precisam expandir `PendingAllEdge` em N arestas owner→matched no grafo de dependência.
  - `invokeAndCopy` (caminho Singleton) precisa, além do loop atual sobre
    `placeholdersFor(node)` (edges de `MustInject[T]` ponteiro), escrever `real` no slot certo
    de qualquer `PendingAllEdge` slice em que `node` seja um dos `Matches` (`slice.Index(i).Set(real)`).
  - Fail-fast de REQ-005 (Transient matched como membro) precisa rodar cedo -- candidato:
    validação na hora de montar o grafo (`scopedGraph`) ou logo após `MustAll` coletar os matches,
    ainda em Stage 2 (falha mais cedo = melhor diagnóstico).
- `internal/resolver/direct.go` -- `findDirectMatches`/`FindDirectAll` já implementam a lógica de
  precedência estrutural (próprio módulo > imports) que o novo matcher precisa espelhar, mas
  operando sobre `module.ProviderRef` (refs, não valores resolvidos) já que Stage 3 não rodou
  ainda -- provavelmente precisa de uma variante nova (`FindAllRefs`? nome a definir em Design)
  que reusa a MESMA regra de precedência sem exigir `ResolvedValue()`.
- `internal/module` -- `module.Owner`/`Module.OwnProviders()`/estrutura de imports, pra andar a
  árvore já assemblada e achar Providers cujo `ResolvedType()` implementa `T`.
- `gonest.go` -- `MustInjectAll[T]` (re-export público) já existe, só precisa continuar
  funcionando (nenhuma mudança de assinatura pública esperada).
- `INSIGHT-MUST-INJECT-ALL.md` (raiz) -- rascunho motivador, referência pro exemplo canônico
  (`Pingable`/`HealthUsecase`).

## Out of Scope

- Provider `scope.Transient` como membro do slice (REQ-005) -- usuário confirmou que não há caso
  de uso real hoje (uso previsto é health-check agregando adapters Singleton).
- Garantir ordem estável no slice retornado (REQ-004).
- `MustInjectAll[T]` a partir de `*module.LazyModule` (Lazy dispatch, `mustLazy`) -- não mencionado
  no insight nem pedido pelo usuário; Lazy já é escopo restrito (só própria módulo, sem imports,
  só Singleton) e não foi solicitado suportar slice ali nesta rodada.
- Reexpor/alterar a assinatura pública de `gonest.MustInjectAll[T]` -- já aceita `owner any` hoje
  (`gonest.Provider`/`gonest.Controller`/etc compartilham o mesmo formato de owner opaco), nenhuma
  mudança de tipo esperada, só o dispatch interno ganha um branch.

## Open Questions

(Resolvidas via discuss nesta sessão -- registradas aqui por rastreabilidade)

- ~~Transient como membro do slice?~~ → Não suportado (REQ-005).
- ~~Ordem garantida?~~ → Não garantida (REQ-004).
- ~~Zero matches panica?~~ → Não, slice vazio (REQ-003).

Nenhuma pergunta bloqueante restante. Pronto pra Design.
