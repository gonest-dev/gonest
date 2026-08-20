# Design: Provider-side MustInjectAll

## Architecture Overview

Reusa o EXATO mecanismo de indireção que `MustInject[T]` (ponteiro, Provider-a-Provider) já usa
há muito tempo -- generalizado de "1 placeholder" pra "N slots de um slice pré-alocado":

- `MustInject[T]` ponteiro hoje: aloca `placeholder := reflect.New(t.Elem())` (memória vazia),
  devolve o PONTEIRO já (endereço estável), registra `PendingEdge{Owner, TargetType, Placeholder}`.
  Stage 3, quando o Provider alvo termina, faz `placeholder.Elem().Set(real.Elem())` -- muta o
  QUE o ponteiro aponta, não o ponteiro em si. Quem já recebeu o ponteiro (fechado numa closure de
  `Constructor`) enxerga o valor real na hora em que a closure roda de verdade (Stage 3, depois que
  as dependências terminam).

- `MustInjectAll[T]` (Provider owner, feature nova): T é interface, não existe "endereço de 1
  valor" pra devolver -- mas o número de Providers candidatos (que implementam T e estão visíveis
  no escopo de `owner`) já é conhecido AGORA, porque a árvore de módulos (Stage 1 `Assemble`) já
  terminou quando o builder fn de um Provider roda (Stage 2 `Declare`). Isso permite pré-alocar um
  slice de comprimento FIXO (`reflect.MakeSlice(reflect.SliceOf(t), N, N)`) e devolvê-lo JÁ AGORA
  -- elementos de um `reflect.Value` de slice são sempre endereçáveis/settable via `Index(i)`,
  mesmo que o slice em si não tenha sido obtido através de um ponteiro (slice já carrega um
  ponteiro pro array subjacente). Cópias do slice header (inclusive a capturada por uma closure de
  `Constructor`) compartilham o MESMO array -- escrever em `slice.Index(i)` depois é visível em
  QUALQUER cópia do header já distribuída. Zero passo de "commit" exigido do chamador.

```
MustInjectAll[port.Pingable](p)          -- Stage 2 (Declare), owner = *provider.Provider
        │
        ├─ 1. acha candidatos: owner.OwnerModule() próprios + imports.EffectiveExports(),
        │     ResolvedType() == port.Pingable (match EXATO, mesma convenção de findDirectMatches/
        │     Find -- interface só resolve via gonest.ProviderAs[T] explícito, sem fallback
        │     estrutural Implements(), ver AD-053)
        │
        ├─ 2. desembrulha qualquer providerAsRef casado pro ref CONCRETO que ele embrulha
        │     (InnerRef()) -- é o ref concreto que Stage 3 de fato constrói; o view em si é
        │     excluído do passo de construção (allProviders já filtra isProviderAsView)
        │
        ├─ 3. valida REQ-005: todo candidato deve ser scope.Singleton -- panic imediato
        │     (ainda em Stage 2, fail-fast) se algum for Transient
        │
        ├─ 4. aloca slice := reflect.MakeSlice(SliceOf(T), N, N), devolve JÁ AGORA
        │     (slice.Interface().([]T)) pro chamador -- N elementos zero-value (nil interface)
        │
        └─ 5. registra PendingAllEdge{Owner, InterfaceType, Matches (refs concretos), Slice}

                                    ▼ (Stage 3, resolveGraph)

BuildGraph() expande cada PendingAllEdge em N arestas owner→match[i] (além das arestas de
PendingEdge já existentes) -- waitDeps(owner) só libera depois que TODOS os N candidatos tiverem
terminado.

invokeAndCopy(node)  -- quando node É um dos Matches de alguma PendingAllEdge:
  depois de calcular `real` (valor resolvido de node) e do loop existente de placeholdersFor,
  roda um loop novo: pra cada PendingAllEdge cujo Matches contém node no índice i,
  `edge.Slice.Index(i).Set(real)` -- escreve o valor no SLOT certo do slice compartilhado.
```

## Dependency Paths (sem grafo indexado -- leitura direta de código)

- REQ-001/REQ-002 → `internal/inject.MustAll[T]` (dispatch) → `mustAllProvider[T]` (novo) →
  `findAllRefs` (novo, mesmo pacote) → `internal/module.Module.OwnProviders`/`ImportedModules`/
  `EffectiveExports` (já existentes, reusados sem mudança de assinatura).
- REQ-002 (escrita in-place) → `internal/resolver/stage3.go`'s `invokeAndCopy` (Singleton path) →
  loop novo sobre `inject.PendingAllEdges()`.
- REQ-007 (ordenação/ciclo) → `internal/resolver/graph.go`'s `BuildGraph` → loop novo sobre
  `inject.PendingAllEdges()`, mesma forma do loop já existente sobre `inject.PendingEdges()`.
  `DetectCycle`/`scopedGraph`/`resolveGraph`/`waitDeps` NÃO mudam -- arestas owner→matched entram
  no MESMO `map[module.ProviderRef][]module.ProviderRef` que `BuildGraph` já produz, tratadas de
  forma idêntica a qualquer outra aresta.
- REQ-005 (Transient rejeitado) → checado dentro de `mustAllProvider`, reusando o padrão duck-typed
  `ResolvedScope() scope.Scope` que `mustLazy`'s `lazyScoped` já declara neste mesmo arquivo
  (`internal/inject/inject.go`) -- reaproveitado, não duplicado de novo.

## New Components

| Component | Responsibility | Location |
|---|---|---|
| `PendingAllEdge` (struct) | Registra `{Owner module.Owner, InterfaceType reflect.Type, Matches []module.ProviderRef, Slice reflect.Value}` -- contraparte multi-valor de `PendingEdge` | `internal/inject/inject.go` |
| `pendingAllEdges` + mutex | Bookkeeping process-global (mesmo padrão de `pendingEdges`), limpo em `Reset()` | `internal/inject/inject.go` |
| `PendingAllEdges()` | Getter exportado (cópia defensiva), consumido por `BuildGraph`/`invokeAndCopy` | `internal/inject/inject.go` |
| `findAllRefs(ownerModule *module.Module, t reflect.Type) []module.ProviderRef` | Busca TODOS os candidatos (próprio módulo + imports `EffectiveExports()`) cujo `ResolvedType() == t`, desembrulhando `providerAsRef` via `InnerRef()` -- duplicada aqui (não em `internal/resolver`) pelo MESMO motivo de ciclo de import que já força `mustLazy` a duplicar `constructable`/`scoped`/etc: `internal/resolver` já importa `internal/inject` (`graph.go`, `stage3.go`), então o inverso criaria ciclo | `internal/inject/inject.go` |
| `mustAllProvider[T any](owner module.Owner, t reflect.Type) []T` | Implementa o algoritmo dos 5 passos acima -- chamado pelo novo branch de `MustAll[T]` | `internal/inject/inject.go` |
| `allAsView` (interface local, duck-typed) | `interface{ InnerRef() module.ProviderRef }` -- mesmo formato exportado que `module.providerAsRef.InnerRef()` já expõe (exportado justamente pra permitir esse duck-typing cross-package, mesmo padrão de `IsProviderAsView`) | `internal/inject/inject.go` |

## Modified Components

| Component | Change | Risk |
|---|---|---|
| `internal/inject.MustAll[T]` | Novo branch: se `owner` não é `directResolver` mas satisfaz `module.Owner` (e não é `*module.LazyModule`, explicitamente fora de escopo -- ver Out of Scope do spec.md), despacha pra `mustAllProvider`. Branch existente (directResolver) e o panic de "T deve ser interface" continuam idênticos. | Baixo -- aditivo, branch novo isolado, nenhum branch existente muda de comportamento |
| `internal/inject.Reset()` | Limpa `pendingAllEdges` também | Baixo -- mesmo padrão de `pendingEdges`/`lazyResolved`/`globalSingletons` |
| `internal/resolver/graph.go` (`BuildGraph`) | Loop novo sobre `inject.PendingAllEdges()`, expandindo cada edge em N arestas `owner→match[i]` no mesmo `map[module.ProviderRef][]module.ProviderRef` já retornado | Baixo-médio -- `BuildGraph` é usado por `scopedGraph`→`resolveGraph` (todo bootstrap passa por aqui); erro de unwrap (providerAsRef entrando como target em vez do ref concreto) quebraria ordenação SILENCIOSAMENTE (`waitDeps`'s `done[dep]` lookup falha e faz `continue`, "should not happen" -- ver comentário existente) -- por isso `findAllRefs` já desembrulha ANTES de gravar em `Matches`, nunca deixando esse caso acontecer |
| `internal/resolver/stage3.go` (`invokeAndCopy`) | Loop novo (Singleton path only) escrevendo em `edge.Slice.Index(i)` pra cada `PendingAllEdge` cujo `Matches` contém `node` | Médio -- `invokeAndCopy` é o núcleo do Stage 3 Singleton, qualquer regressão aqui afeta TODO bootstrap, não só esta feature; mitigado por reusar o mesmo guard de tipo (`AssignableTo`) que o loop de `placeholdersFor` já usa (`real.Type() != placeholder.Type() { continue }`), adaptado pra `AssignableTo(edge.InterfaceType)` (interface, não igualdade exata) |
| `internal/resolver/stage3.go` (`invokeAndCopyEdge`, Transient path) | NENHUMA mudança -- REQ-005 garante que nenhum `Matches` member é Transient, então este caminho nunca precisa saber de `PendingAllEdge` | Nenhum (por construção) |

## Risks

- `invokeAndCopy` é chamado por TODA resolução Singleton do processo (não só providers alvo de
  `MustInjectAll`) -- o loop novo roda pra TODO node, mesmo quando `PendingAllEdges()` está vazio
  (custo O(edges × matches) por node resolvido). Em bootstraps sem uso desta feature,
  `PendingAllEdges()` retorna slice vazio/nil -- custo real é zero (loop externo não itera nada).
  Em uso normal (poucos MustInjectAll por app), overhead é desprezível. Não visto como risco real,
  registrado por transparência.
- `findAllRefs` duplica (não reusa) a lógica de precedência de busca que `Find`/`candidateProviders`
  já têm em `internal/resolver` -- mesmo trade-off que `mustLazy` já aceita hoje (comentado em
  `inject.go`: "ciclo de import impede reuso"). Risco de DRIFT entre as duas implementações se uma
  mudar sem a outra -- mitigado tendo os dois pontos de duplicação (Find/candidateProviders E
  findAllRefs) cross-referenciados via comentário `// mirrors internal/resolver's Find/
  candidateProviders -- see their doc comment for why this is duplicated, not imported`.
- Nenhum "God Node" identificado via grafo (`.specs/graph/graph.json` ausente, modo degradado) --
  por inspeção manual, `internal/resolver/stage3.go`'s `invokeAndCopy`/`BuildGraph` são os pontos
  de maior centralidade estrutural do bootstrap inteiro (todo provider passa por eles), mas a
  mudança proposta é estritamente aditiva (loop novo condicional a `PendingAllEdges()` não-vazio) --
  risco de regressão em bootstraps EXISTENTES (sem uso da feature) é próximo de zero por construção.

## Decision Log

- Reusar a MESMA classe de indireção reflect (endereço estável / elemento de slice sempre
  endereçável) que `MustInject[T]` ponteiro já usa, em vez de inventar um mecanismo de callback/
  future -- menor superfície nova, mesmo modelo mental que o resto do DI já ensina.
- `findAllRefs` fica em `internal/inject` (duplicado), não em `internal/resolver` (reusado) --
  `internal/resolver` já importa `internal/inject` (BuildGraph/stage3.go), então o inverso cria
  ciclo. Mesmo padrão já estabelecido por `mustLazy`.
- Unwrap de `providerAsRef` acontece em `findAllRefs`, ANTES de gravar `PendingAllEdge.Matches` --
  não em `BuildGraph` nem em `invokeAndCopy` -- centraliza o único ponto onde esse desembrulho
  precisa acontecer, evitando esquecê-lo em um dos dois consumidores (grafo e cópia de slot).
- Checagem de REQ-005 (Transient rejeitado) acontece em `mustAllProvider`, durante Stage 2 (na
  hora da chamada de `MustInjectAll`), não adiada pra Stage 3/`scopedGraph` -- falha mais cedo,
  mesma decisão já tomada por `mustLazy`'s LAZY-07.
- `gonest.MustInjectAll[T]`/`gonest.go` NÃO mudam -- já são passthrough genérico pra
  `inject.MustAll[T]`, confirmado lendo o wrapper existente.
