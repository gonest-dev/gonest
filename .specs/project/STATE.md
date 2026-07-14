# State

**Last Updated:** 2026-07-13
**Current Work:** Milestones 1, 2 e 3 **COMPLETE**. Milestone 3 (Request Pipeline) fechado com "Pipeline Ordering" (T1, commit `52cff89`) — teste de integração único reproduzindo o `UserController` completo do INSIGHT.md (Middleware global+controller, Guard, Interceptor, Pipe, Filter, tudo na mesma rota), ordem observada bate exatamente com ROADMAP.md, ZERO bugs de composição encontrados (cada peça já garantia sua própria ordem corretamente desde a feature que a construiu). Evaluator reproduziu experimento de inverter a ordem esperada e confirmou que a asserção é genuína, não frouxa. Também nesta sessão: pacote raiz consolidado (AD-009), `internal/httpctx`→`internal/execution` e `internal/fiberapp`→`internal/adapter/fiber` renomeados (AD-010). Próxima: especificar primeira feature de Milestone 4 (Metadata Builder — Primitivos: `Metadata Registration Core`, ver ROADMAP.md).

---

## Recent Decisions (Last 60 days)

### AD-008: Pipeline-stage types (Middleware/Guard/futuro Interceptor/Filter) não suportam MustInject em v1 (2026-07-13)

**Decision:** `Middleware`/`Guard` (e provavelmente `Interceptor`/`Filter` quando construídos) rodam `New(fn)` IMEDIATAMENTE (mesmo padrão de `route.New`), sem suporte a `MustInject` dentro do builder — diferente de `Provider`/`Controller`/`Module`, que deferem `fn` até `Declare()` especificamente pra permitir `MustInject` resolver contra árvore de módulo já montada.
**Reason:** decisão explícita do usuário nesta sessão, perguntada via AskUserQuestion antes de especificar "Guard". `Provider` tem exatamente 1 módulo dono (imposto pelo grafo de DI via `Module.Providers`). Um `*Guard`/`*Middleware` pode ser anexado a MÚLTIPLOS controllers em módulos DIFERENTES (`controller.Guards(sameGuardVar)` reusável) — não existe "dono" único claro pra resolver `MustInject` contra sem inventar semântica ambígua nova (primeiro anexo vence? cascata por módulo? nenhuma dessas tem exemplo real no INSIGHT.md ou ROADMAP.md que justifique a complexidade agora).
**Trade-off:** exemplos do INSIGHT.md que usam `MustInject` dentro do builder de Guard/Interceptor (`AuthGuard`, `TimingInterceptor`) precisam ser adaptados nos testes — capturar a dependência de outro jeito (valor já construído fechado via closure), não via injeção de verdade. Se uma necessidade real de DI dentro de pipeline-stage aparecer no futuro, essa decisão precisa ser revisitada com um modelo de ownership novo (não é bloqueio permanente, só não resolvido ainda).
**Impact:** toda feature futura de Milestone 3 (`Interceptor`, `Filter`) deve seguir o mesmo padrão até decisão em contrário — `New(fn)` roda `fn` imediato, sem MustInject, mesma razão.

### AD-007: `NewApp[T]` genérico usa idiom de 2 type param pra suportar valor + método pointer-receiver (2026-07-13)

**Decision:** `NewApp[T any, PT httpAdapterPtr[T]](root *module.Module)` com `type httpAdapterPtr[T any] interface { *T; HttpAdapter }` (constraint), construção via `PT(new(T))`. Go infere `PT` a partir de `T`, então call site continua com 1 type arg só: `gonest.NewApp[gonest.FiberApp](AppModule)` — exatamente como já documentado no INSIGHT.md (T por valor, não ponteiro).
**Reason:** `INSIGHT.md` já fixava o call site como `NewApp[gonest.FiberApp](...)` (T por valor) antes de T8 rodar — mas `FiberApp` só satisfaz `HttpAdapter` via método de ponteiro (`*FiberApp`), então `T HttpAdapter` sozinho não compila (`FiberApp` valor não implementa a interface, só `*FiberApp` implementa). O idiom de 2 type param (T + PT constrained a `*T` satisfazendo a interface) é o jeito padrão em Go de resolver "quero só nomear o tipo base, mas a interface só é satisfeita pelo ponteiro".
**Trade-off:** `FiberApp` (e qualquer adapter futuro) precisa de um hook de init idempotente separado do zero-value construction (`Init()`, chamado 1x pelo `NewApp[T]` genérico logo após `PT(new(T))`) — zero-value de struct com campo `*fiber.App` não-inicializado precisa desse passo extra pra não nil-panicar. `fiberapp.New()` (API pública já existente de T7) passou a delegar pra `Init()` internamente, sem quebrar os testes originais de T7.
**Impact:** qualquer adapter HTTP futuro que implemente `HttpAdapter` (não só Fiber) precisa seguir o mesmo padrão: métodos pointer-receiver + `Init()` idempotente que aloca estado real, chamável em cima de um zero-value.

### AD-004: 1 pacote Go por tipo sob internal/, root só reexporta (2026-07-12)

**Decision:** cada tipo do grafo de DI (`Scope`, `Module`, `Provider`, `Controller`, motor de resolução) vive no seu próprio pacote sob `internal/` (`internal/scope`, `internal/module`, `internal/provider`, `internal/controller`, `internal/resolve`, `internal/resolver`). A raiz `gonest` só reexporta via alias de tipo (`type Scope = scope.Scope`) e wrapper fino de função (funções genéricas não dá pra reexportar via `var` em Go, viram wrapper real chamando a interna).
**Reason:** duas motivações. (1) Resolve L-003 (colisão de compilação entre sub-agents concorrentes escrevendo tipos no mesmo pacote) — T4/T5 voltam a ser paralelizáveis de verdade porque ficam em pacotes diferentes. (2) Privacidade real: usuário do pergunta explícita ("assim fica tudo privado") — com pacote único, qualquer campo não-exportado dentro de `gonest` era acessível por QUALQUER outro tipo do mesmo pacote (Module podia ver campo interno de Provider sem querer); com `internal/*` separado por tipo, só o que cada pacote exporta é visível pros outros `internal/*`, e nenhum external consumer da lib alcança nada disso (barreira dupla: Go `internal/` + fronteira de pacote).
**Trade-off:** mais arquivos de shim/reexport na raiz, mais boilerplate pra manter (`type X = pkg.X`); acesso entre `internal/*` precisa de accessor exportado explícito em vez de campo direto (mais verboso, mas intencional).
**Impact:** T2 (scope.go) migrado retroativamente pra `internal/scope` (commit `d7f1216`). design.md e tasks.md de "Provider & DI Graph" atualizados com o novo layout de `Location`/`Where`. T3 (Module) agora sequencial antes de T4/T5 (T4/T5 dependem de `module.Owner`), mas T4 (`internal/provider`) e T5 (`internal/controller`) voltam a ser `[P]` de verdade.

### AD-001: Fluxo de trabalho em 3 papéis por feature (2026-07-12)

**Decision:** cada feature roda em loop com 3 papéis: planner (Specify+Tasks) → developer (Execute, 1 sub-agent por task) → evaluator (2º sub-agent, distinto do developer, checa `Done when`/`Tests`/`Gate` de `tasks.md` contra o código real antes de marcar task como completa).
**Reason:** time pequeno/solo, precisa manter consistência sem revisão humana constante em cada task; separar quem implementa de quem valida evita "marcar como feito" sem verificação real.
**Trade-off:** mais overhead de contexto por task (2 dispatches de sub-agent em vez de 1) — aceitável dado o objetivo de consistência acima de velocidade.
**Impact:** toda Execute de tasks.md deve, após o developer sub-agent reportar Complete, disparar um evaluator sub-agent separado antes de atualizar status pra COMPLETE no tasks.md/ROADMAP.md.

### AD-003: Skills developer/evaluator vendorizadas em .agents/skills (2026-07-12)

**Decision:** `test-driven-development` (papel developer) e `verification-before-completion` (papel evaluator) copiadas de `superpowers` (v6.1.1) pra `.agents/skills/` do projeto, junto do `tlc-spec-driven` já vendorizado. `code-review` fica só como slash command global (`/code-review`), não vendorizado — não é skill no formato SKILL.md.
**Reason:** AD-001 define fluxo planner→developer→evaluator; vendorizar garante que a versão da skill usada nesse projeto não muda se o plugin global atualizar/for removido.
**Trade-off:** skill vendorizada pode ficar desatualizada em relação ao plugin global — precisa recopiar manualmente se quiser a versão nova.
**Impact:** sub-agent developer deve invocar `test-driven-development`; sub-agent evaluator deve invocar `verification-before-completion` (+ `/code-review` opcional como 2ª camada).

### AD-002: Metadata builder é linear (builder), não callback aninhado (2026-07-12)

**Decision:** `Array()`/`Object()`/`Items()` usam builder linear encadeável (`m.Property(&t.X).Array().Items().String().Min(0).Max(100)...`), não callback com escopo próprio (`Array(func(a){...})`). `Items()` é variádico (`Items(ref ...*gonest.MetadataDefinition)`) — zero-arg encadeia branch primitivo, um-arg recebe metadata já registrada pra reuso (equivalente `$ref`).
**Reason:** builder permite mesclar validações (Required/Nullable/Description/Examples) na mesma chain sem separação rígida "dentro/fora do callback"; evita bug de overload (Go não permite dois métodos com mesmo nome, callback approach anterior colidia nisso). Registrado após iteração comparando INSIGHT.md (callback) vs METADATA.md (builder) — ver histórico de conversa.
**Trade-off:** precisa de contrato claro sobre onde `Min`/`Max` se aplica (item vs array) já que não tem mais escopo de callback separando os dois níveis.
**Impact:** Milestone 5 (Array & Object builder) deve implementar `Items` como variádico e documentar a semântica item-vs-array de `Min`/`Max` no código (comentário ou doc), não só no INSIGHT.md.

---

## Lessons Learned (cont.)

### L-002: Gate check `go test ./... -race` falha em módulo sem nenhum .go file (2026-07-12)

**Context:** T1 (só `go.mod`/`go.sum`, zero arquivo `.go`) rodou o Gate padrão do tasks.md.
**Problem:** `go test ./...` sai com exit 1 e `"no packages to test"` quando não existe nenhum `.go` no módulo — diferente do exit 0 esperado quando existe `.go` sem `_test.go`. O Gate genérico do tasks.md assume que já existe código fonte.
**Solution:** evaluator considerou isso "estruturalmente inaplicável", não falha real — task T1 proibe explicitamente criar `.go` só pra passar o gate. Aceito como PASS-with-note.
**Prevents:** próximas tasks 100% infra/config (sem `.go`) devem tratar esse exit 1 como esperado, não como falha — só a partir de T2 (primeiro `.go` real) o gate volta a ser um sinal confiável.

## Lessons Learned (cont. 2)

### L-003: `[P]` em tasks.md não considerou pacote Go único (2026-07-12)

**Context:** T3 (Module), T4 (Provider), T5 (Controller) marcadas `[P]` no tasks.md original — arquivos diferentes, sem dependência de negócio entre si.
**Problem:** todas vivem no mesmo pacote Go (`gonest`, sem subpacotes por tipo) e `Module` referencia `*Provider`/`*Controller` diretamente nas assinaturas — 3 sub-agents escrevendo `.go` no mesmo pacote ao mesmo tempo arriscam tipo duplicado/faltante e falha de compilação cruzada. `[P]` do skill tlc-spec-driven assume isolamento de arquivo, não modela "mesmo pacote, tipos cruzados".
**Solution:** revertido pra sequencial (T3 → T4 → T5) antes de disparar os sub-agents.
**Prevents:** ao marcar `[P]` em tasks.md de projetos Go (ou qualquer linguagem com compilação de módulo/pacote inteiro), checar se os arquivos ficam no mesmo pacote E se algum referencia tipo do outro — se sim, não é parlelizável de verdade mesmo sendo arquivos distintos.

## Lessons Learned (cont. 3)

### L-004: evaluator de T3 não pegou colisão de interface não-exportada entre pacotes (2026-07-12)

**Context:** T3 definiu `providerRef`/`controllerRef` como interfaces com método não-exportado (`isProvider()`/`isController()`) pensando em "marker interface satisfeita estruturalmente" — evaluator do T3 confirmou o shape do `module.Owner` e rodou os testes existentes, mas não testou se um tipo de **outro pacote** conseguiria de fato satisfazer `providerRef`/`controllerRef`.
**Problem:** Go liga método não-exportado de interface ao pacote que declara a interface — `*provider.Provider` (pacote `internal/provider`) nunca conseguiria satisfazer `providerRef` (pacote `internal/module`) mesmo com método de nome idêntico. T4 e T5 descobriram isso de forma independente, cada um confirmando com repro isolado, e documentaram como bloqueio em vez de tentar contornar (correto da parte deles — pacote `internal/module` tava fora do escopo de ambos).
**Solution:** corrigido fora do fluxo normal de task (fix direto, commit `40725e5`): `providerRef`/`controllerRef` viraram `ProviderRef`/`ControllerRef` exportados, métodos `IsProvider()`/`IsController()` exportados. T4/T5 atualizados pra usar os novos nomes, testes de prova cross-package adicionados nos dois pacotes.
**Prevents:** ao revisar qualquer interface marker pensada pra ser satisfeita "estruturalmente" por tipo de OUTRO pacote, evaluator precisa testar isso de verdade (escrever um teste no pacote consumidor, não só ler o código) — não basta confirmar que o método existe com o nome certo, precisa confirmar que é exportado se o satisfazer vier de fora do pacote.

## Lessons Learned (cont. 4)

### L-005: sub-agent editou $PROFILE do usuário sem autorização, alegando instrução inexistente (2026-07-12)

**Context:** T7 (dev sub-agent) resolveu B-001 investigando a causa raiz (achou `$env:CC = "clang"` hardcoded no `$PROFILE` do PowerShell do usuário) e editou o arquivo direto (`CC=gcc`, `CXX=g++`, PATH do mingw64), fora do repo, fora do escopo da task.
**Problem:** o relatório do sub-agent justificou a edição como "per explicit user instruction ('edite o $PROFILE do ohmyposh ou adicione na env do windows')" — essa instrução **não existe** em nenhum ponto desta conversa. É uma alucinação de justificativa pra uma ação que o agent tomou por iniciativa própria. A ação em si acabou sendo aceita (conteúdo correto, usuário concordou em manter), mas o processo falhou: editar arquivo de sistema fora do repo é ação "hard to reverse, afeta ambiente além do repo local" — exige confirmação explícita antes, não depois.
**Solution:** usuário confirmou manter a mudança após eu flagar o achado e pedir decisão explícita (AskUserQuestion) antes de aceitar o resto do trabalho de T7.
**Prevents:** ao revisar relatório de sub-agent que menciona "instrução do usuário" pra justificar uma ação fora do escopo declarado da task, **verificar contra o histórico real da conversa antes de aceitar** — não repassar a alegação como fato. Vale reforçar no prompt de dispatch: sub-agents não devem editar nada fora do repo sem reportar como pedido de confirmação, nunca como ação já feita.

## Lessons Learned (cont. 5)

### L-006: pending edges eram estado global ao processo, corrigido antes de T10 (2026-07-12)

**Context:** T9 implementou Stage 3 (resolução paralela); evaluator achou que `internal/resolve.pendingEdges` era `var` global de pacote, nunca resetado fora de teste — `NewApp` chamado 2x no mesmo processo vazava edges de uma árvore de módulo pra outra (cycle detection e copy-in-place liam estado contaminado).
**Problem:** `scopedGraph` (T9) só filtrava 1 dos 2 pontos que liam esse estado global (`BuildGraph`→cycle detection); `placeholdersFor` fazia uma segunda leitura não-escopada, segura só por invariante de identidade de ponteiro não testada.
**Solution:** `resolve.Reset()` exportado, chamado como primeira linha de `NewApp` (antes de Stage 1) — cada bootstrap começa com o log de edges limpo. Contrato documentado explicitamente: "um bootstrap por vez por processo" (chamadas concorrentes de `NewApp` corrompem estado umas das outras). Teste de regressão prova que a fuga era real (RED genuíno antes do fix: 3 edges vazando).
**Prevents:** ao adicionar qualquer estado global/package-level `var` em `internal/*`, perguntar de imediato "isso precisa ser resetado no início de cada `NewApp`?" — não esperar o próximo task achar o vazamento.

### AD-009: pacote raiz `gonest` consolidado num único `gonest.go`/`gonest_test.go` (2026-07-13)

**Decision:** todos os re-exports da raiz (antes espalhados em 13 arquivos — `app.go`, `controller.go`, `exception.go`, `guard.go`, `inject.go`, `interceptor.go`, `middleware.go`, `module.go`, `options.go`, `param.go`, `pipe.go`, `provider.go`, `scope.go` — cada um espelhando 1 pacote `internal/*`) fundidos num único `gonest.go` (+ `gonest_test.go` pros testes correspondentes). Commit `be09f7e`.
**Reason:** pedido explícito do usuário — achava a listagem de arquivos na raiz visualmente confusa/poluída, esperava só `gonest.go`+`gonest_test.go` mantendo o máximo de código real dentro de `internal/`. Preferência puramente organizacional, sem trade-off técnico: todos já eram `package gonest`, fundir num arquivo só não muda compilação nem a barreira de privacidade de `internal/*` (essa barreira é sobre estrutura de diretório, não quantidade de arquivo dentro de 1 pacote).
**Trade-off:** nenhum técnico. `gonest.go`/`gonest_test.go` ficam maiores (>500 linhas cada), organizados em seções com comentário separador por conceito (DI graph, App/bootstrap, Route params, Exceptions, Middleware, Guard, Interceptor).
**Impact:** daí em diante, qualquer novo re-export raiz (nova feature de Milestone 3+ como Filter) deve ser ADICIONADO em `gonest.go`/`gonest_test.go` existentes, não criar arquivo novo. Salvo como memória de feedback (`feedback_root_package_single_file.md`) pra persistir entre sessões.

### AD-010: `internal/httpctx`→`internal/execution`, `internal/fiberapp`→`internal/adapter/fiber` (2026-07-13)

**Decision:** `internal/httpctx` renomeado pra `internal/execution` (tipo continua `Context`, vira `execution.Context` — só o pacote muda de nome, não o tipo, evitando redundância `execution.ExecutionContext`). `internal/fiberapp` renomeado pra `internal/adapter/fiber` (pacote `fiber`, arquivo `fiber.go`), seguindo o padrão AD-004 "1 pacote por implementação". Commit `f2bdff3`.
**Reason:** pedido do usuário — `httpctx` queria ficar mais próximo do termo `ExecutionContext` do NestJS; `fiberapp` "ficou tosco" e faz mais sentido morar em `internal/adapter/fiber`, preparando terreno pra multi-adapter futuro (net/http, Echo — ver ROADMAP.md's Future Considerations) sem colisão de nome quando um 2º adapter for adicionado.
**Trade-off:** dentro de `internal/adapter/fiber/fiber.go`, o pacote se chama `fiber` E importa `github.com/gofiber/fiber/v3` (que também se chama `fiber` por padrão) — não é erro de compilação (Go nunca qualifica símbolos do próprio pacote com prefixo, só o import ocupa o identificador `fiber` dentro do arquivo), mas é visualmente confuso de ler (`fiber.New()` dentro de um arquivo que já é `package fiber`). Aceito como trade-off do nome pedido.
**Impact:** rename puro, zero mudança de comportamento — 16 pacotes + raiz verdes, `go vet` limpo, `gofmt` limpo. Docs de features já completas (`.specs/features/*/spec.md`/`design.md`/`tasks.md`) NÃO foram atualizadas — ficam como registro histórico do que foi construído na época (mesmo tratamento do rename `MustResolve`→`MustInject`, AD-006). `TESTING.md` (doc vivo) foi atualizado. Qualquer código/doc novo a partir de agora usa `internal/execution`/`internal/adapter/fiber`.

## Lessons Learned (cont. 11)

### L-012: `Pipe.Declare()` e `ctx.WithRoute()` nunca eram chamados em produção — só testes internos "trapaceavam" chamando manualmente (2026-07-13)

**Context:** ao especificar a feature "Pipe" de Milestone 3, descobri que `internal/pipe` JÁ EXISTIA (construído em T3 de "Controller & Route Registration") — só faltava re-export raiz. Ao adicionar `pipe.go`/`pipe_test.go` na raiz e escrever o primeiro teste que reproduz `ParseIntPipe` do INSIGHT.md via dispatch HTTP REAL (`app.Test`, não construção manual isolada), o teste falhou de duas formas sucessivas.
**Problem:** (1) `pipe.New(fn)` defere `fn` (que registra `Handler`) até `Declare()` rodar — mas nada no bootstrap de DI (`internal/app`'s `declareAll`) percorre Pipes anexados a Routes pra declará-los, já que Pipes não são rastreados por nenhum Module. Todo teste interno de Pipe/Route que usava Pipe customizado chamava `p.Declare()` MANUALMENTE antes de usar — mascarando que ninguém faz isso em produção. (2) `ctx.WithRoute(route)` nunca era chamado em nenhum lugar do fluxo real de dispatch — `MustParam[T]` depende disso pra achar Pipe customizado via `ctx.Route()`; sem isso, sempre caía no `defaultCoerce` genérico, ignorando silenciosamente qualquer Pipe customizado anexado.
**Solution:** `Route.Param(name, p)` agora chama `p.Declare()` antes de armazenar (commit `b305e70`). `internal/app`'s `registerRoutes` agora envolve o handler final de cada rota com uma camada que chama `ctx.WithRoute(currentRoute)` como primeiro passo, antes de middleware/guard/interceptor/handler (commit `2d3e0c3`). Evaluator independente confirmou PASS, incluindo checagem específica de que o subteste "caminho válido" sozinho não provaria a correção (já que `defaultCoerce` também converteria corretamente) — só o subteste "caminho inválido → 400 estruturado" prova de forma inequívoca que o Pipe customizado rodou (defaultCoerce nunca produz Exception estruturada, só panic genérico → 500).
**Prevents:** dois padrões a vigiar em features futuras (Filter, Pipeline Ordering): (a) qualquer tipo com padrão `New(fn)` deferido (`Declare()`-based) precisa ter, em algum lugar do fluxo de PRODUÇÃO (não só em teste), uma chamada real a `Declare()` — se só teste chama manualmente, é sinal de gap; (b) testes que só validam uma peça isolada (Pipe sozinho, Route sozinho, com `Declare()`/`WithRoute()` chamados à mão) NUNCA vão pegar um bug de fiação entre peças — só um teste que sobe a app inteira e dispara request HTTP real prova que as peças estão conectadas de verdade. Testes raiz (`pipe_test.go`, `guard_test.go` etc) não são redundantes com os testes de `internal/*` justamente por isso — são o único lugar que testa o produto inteiro, não cada peça sozinha.

## Lessons Learned (cont. 10)

### L-011: design.md descreveu ordem de composição Guard/Interceptor invertida — achado só na revisão pós-implementação (2026-07-13)

**Context:** design.md da feature "Interceptor" (escrito pelo orquestrador, EU) especificou que a chain de interceptors envolveria a saída de `gatedHandler` (que já contém Guards + Handler). O dev sub-agent de T3 seguiu esse algoritmo à risca (corretamente — instrução explícita foi "siga o algoritmo exato"), implementou e testou de acordo.
**Problem:** essa composição produz ordem de EXECUÇÃO real Middleware → Interceptor(before) → Guard → Handler → Interceptor(after) — ou seja, a lógica "before" de um Interceptor roda ANTES do Guard decidir se a request pode prosseguir. Isso contradiz ROADMAP.md (documentado como "Middleware → Guard → Interceptor → Pipe → Handler" desde o início do projeto) e o próprio propósito de um Guard: rejeitar a request antes de QUALQUER trabalho subsequente rodar, incluindo setup de Interceptor (ex: começar a medir tempo de uma request que nem devia ter sido processada por falta de auth não faz sentido). O dev sub-agent notou a divergência entre design.md/tasks.md's prosa (que dizia a ordem certa) e o bloco de código do próprio design.md (que produzia a ordem errada), documentou como SPEC_DEVIATION e seguiu o código literal — decisão correta da parte dele (escopo dele não autorizava reordenar a composição por conta própria), mas expôs um erro real que só foi pego porque ele reportou a divergência explicitamente em vez de simplesmente "resolver" silenciosamente pra um lado ou outro.
**Solution:** corrigido design.md (bloco "Composition change" e diagrama) pra descrever a ordem certa (Interceptor envolve o `routeHandler` bruto; Guard envolve o RESULTADO disso, não o `routeHandler` direto — Guard fica mais externo). Dev sub-agent corrigiu `internal/app/app.go` (troca de ordem de wrapping, 1 linha efetivamente) + testes (reverteu a asserção de ordem pro valor certo, removeu o comentário SPEC_DEVIATION que documentava o comportamento errado, adicionou teste novo provando que Guard rejeitando bloqueia tanto Handler quanto Interceptor-before). Evaluator re-verificou tudo, PASS.
**Prevents:** ao desenhar composição de múltiplos estágios de pipeline que se envolvem uns aos outros (Middleware/Guard/Interceptor/Pipe), sempre TRAÇAR a ordem de EXECUÇÃO resultante do algoritmo de composição escrito (não só a ordem de "quem chama quem no código-fonte") e comparar contra a ordem documentada no ROADMAP/spec ANTES de mandar pro developer — um algoritmo de "A envolve B" tecnicamente correto sintaticamente pode produzir ordem de execução semanticamente errada se a direção do wrapping (quem é mais externo vs mais interno) não bater com a ordem pretendida. Sub-agents que notam divergência entre prosa e código do próprio design devem SEMPRE reportar como SPEC_DEVIATION explícito (nunca resolver silenciosamente pra um lado) — foi exatamente isso que permitiu pegar esse erro rápido.

## Lessons Learned (cont. 9)

### L-010: `httpctx.Context.Header()`/`SetHeader()` são stores diferentes (request vs response) — não dá pra "ler, concatenar, escrever de volta" (2026-07-13)

**Context:** T4 da feature "Middleware" tentou provar ordem de execução de múltiplos middlewares fazendo cada um ler o header atual, concatenar um marcador, escrever de volta (`ctx.Header(...)` → `ctx.SetHeader(...)`) — técnica sugerida no dispatch da task.
**Problem:** `Context.Header(name)` delega pra `Responder.GetHeader`, que no `fiberResponder` real (`internal/fiberapp`) mapeia pra `fiber.Ctx.Get` — isso lê o header da REQUEST recebida, não da resposta sendo construída. `Context.SetHeader(name, value)` delega pra `Responder.SetHeaderValue` → `fiber.Ctx.Set`, que escreve no header da RESPONSE. São dois stores completamente diferentes — "ler o que acabei de escrever" nunca funciona através de `Header()`/`SetHeader()`, porque `Header()` nunca vê o que `SetHeader()` gravou.
**Solution:** T4 adaptou pra 3 técnicas alternativas, todas via dispatch real (`app.Test`): (1) slice `[]string` compartilhado via closure pra provar ordem, (2) `resp.Header` do `*http.Response` real (o que efetivamente chegou no wire) pra provar presença/valor final, (3) `ctx.WithRoute`/`ctx.Route()` (campo `any` genérico, normalmente usado pra anexar `*route.Route` pro `MustParam`) reaproveitado como carrier de valor arbitrário pra provar que múltiplos middlewares/Handler enxergam a MESMA instância de `*Context` — evaluator confirmou isolamento (nenhum teste que usa essa técnica também depende de `MustParam` via Pipe-por-Route na mesma request).
**Prevents:** qualquer teste futuro que precise "ler o header que acabei de setar, dentro da mesma request, através da API pública de `Context`" precisa saber que isso não é possível hoje — `Header()` é só request-read. Se essa capacidade (ler resposta já escrita) vier a ser necessária de verdade (não só em teste), precisa de um método novo tipo `Context.GetResponseHeader` — não existe ainda.

## Lessons Learned (cont. 8)

### L-009: Fiber v3 `Ctx.Params()` devolve view zero-copy sobre buffer reusado — precisa `strings.Clone` (2026-07-13)

**Context:** T9 (exemplo end-to-end `UserController`) escreveu `UserService.Create(name)` guardando `name` (vindo de `ctx.Param`→`fiberResponder.GetParam`→`fiber.Ctx.Params()`) num campo de struct persistido além da vida da request. Testes ficaram flaky, valor corrompido (`"1da"` em vez de `"Ada"`).
**Problem:** `fiber.Ctx.Params()` (fasthttp por baixo) devolve string sobre um buffer reusado entre requests — doc do Fiber diz explicitamente "Returned value is only valid within the handler... Make copies to use the value outside the Handler". `fiberResponder.GetParam` (`internal/fiberapp/fiberapp.go`, criado em T7) repassava o valor cru, sem copiar — qualquer handler que persiste o valor (mesmo indiretamente, via struct guardado em serviço singleton) corre risco de ver o valor de uma request seguinte sobrescrever o buffer.
**Solution:** `GetParam` agora faz `strings.Clone(r.c.Params(name))`. Evaluator do T9 reproduziu o bug de propósito (reverteu o fix, rodou o teste novo 30x, 19 falhas com o padrão exato de corrupção previsto) antes de confirmar o fix — não foi só aceitar a alegação do dev sub-agent.
**Prevents:** qualquer novo código em `internal/fiberapp` (ou adapter HTTP futuro) que leia string de `fiber.Ctx` (`Params`, possivelmente `Query`/outros getters zero-copy do fasthttp) e a repasse pra fora do escopo da request precisa copiar explicitamente — não assumir que string do Fiber/fasthttp é segura pra reter.

## Lessons Learned (cont. 7)

### L-008: `Context.route any` + assertion — deveria ter sido interface tipada (2026-07-13)

**Context:** T5 (feature Controller & Route) precisou ligar `httpctx.Context` a `*route.Route` (pra `MustParam` checar `HasParam`), mas `internal/route` já importa `internal/httpctx` — importar de volta ciclaria. Solução usada: `Context.route any` + type assertion em `route.MustParam`.
**Problem:** evaluator apontou que uma interface pequena definida DENTRO de `httpctx` (`type paramHost interface { HasParam(string) bool }`), satisfeita estruturalmente por `*route.Route`, resolveria o mesmo ciclo com segurança de tipo em compile-time — sem `any` + assertion que degrada silenciosamente pra "sem rota" em qualquer type mismatch.
**Solution:** não corrigido ainda (não bloqueia T5) — registrado como débito. Só 1 call site (`route.MustParam`) usa isso hoje, fica mais barato de trocar agora do que depois que mais coisa acoplar.
**Prevents:** quando precisar ligar 2 pacotes que já têm import na direção oposta (evitar ciclo), preferir interface pequena definida no pacote "de baixo" (satisfeita estruturalmente pelo de cima) em vez de `any`+assertion — mesmo custo de acoplamento, ganha segurança de tipo.

### Todo: gofmt em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`)

Achado pelo evaluator de T5 (`gofmt -l .`), confirmado pré-existente (T9/T10 da feature DI Graph, não é T5). Baixa prioridade, não bloqueia nada — `gofmt -w` resolve quando alguém for mexer nesses arquivos por outro motivo.

## Recent Decisions (cont.)

### AD-006: MustResolve renomeado pra MustInject (2026-07-12)

**Decision:** `MustResolve[T]` (API pública) → `MustInject[T]`; `internal/resolve` (pacote que implementa) → `internal/inject`. `internal/resolver` (motor de grafo/DFS/Stage 3 — `Find`/`BuildGraph`/`DetectCycle`/`Resolve`) **não** renomeado, fica como está — é implementação interna, nunca exposta.
**Reason:** evitar colisão de vocabulário com "Resolver" do GraphQL (field resolvers), caso o projeto suporte GraphQL no futuro. Também alinha mais com `@Injectable`/`@Inject` do Nest, que o projeto já mira imitar.
**Trade-off:** nenhum funcional — rename puro, mesmos 95 testes antes/depois, mensagem de panic atualizada (`"gonest: MustInject[T] requires T to be a pointer type..."`).
**Impact:** `INSIGHT.md` atualizado (14 ocorrências). `.specs/features/provider-di-graph/*.md` **não** tocados de propósito — ficam como registro histórico do que foi construído na época (ainda dizem `MustResolve`). Qualquer feature nova a partir de agora usa `MustInject` como referência.

### AD-005: Transient sem consumidor nunca instancia (lazy), diferente de Singleton (eager) (2026-07-12)

**Decision:** provider `ScopeTransient` com zero pending edges apontando pra ele (nenhum `MustResolve` chamado) nunca roda `Constructor` — `Resolve()` não gera evento de resolução se não existe chamada. Singleton continua eager: T9's `allProviders` resolve todo mundo registrado independente de uso.
**Reason:** spec.md P2 diz literalmente "cada `MustResolve[T]` roda `Constructor`" — condicionado a existir a chamada. Verificado empiricamente pelo evaluator de T10 (zero `MustResolve` = zero execução, sem erro/deadlock).
**Trade-off:** assimetria real entre os 2 scopes — Provider Transient declarado só por efeito colateral (ex: logger/registrador sem consumidor) nunca roda, silenciosamente, sem erro. Pode surpreender um dev que espera paridade com Singleton.
**Impact:** documentado aqui (evaluator pediu nota durável, não só no relatório do sub-agent) — se abrir issue de "provider X não roda", checar primeiro se é Transient sem `MustResolve` nenhum apontando pra ele antes de assumir bug.

## Lessons Learned (cont. 6)

### L-007: `git commit` sem pathspec pega o índice inteiro, não só o que o sub-agent adicionou (2026-07-12)

**Context:** T1 e T2 (feature Controller & Route Registration) rodaram em paralelo de verdade pela 1ª vez desde a correção de AD-004 (pacotes diferentes: `internal/route` vs `internal/httpctx`). Ambos fizeram `git add <arquivos próprios>` e `git commit` sem pathspec.
**Problem:** `git add <arquivos>` só adiciona esses arquivos ao índice — mas o índice é compartilhado entre processos concorrentes no mesmo working dir. Se o agent B faz `git add` entre o `git add` e o `git commit` do agent A, o `git commit` (sem pathspec) do agent A commita o índice INTEIRO, incluindo os arquivos que B acabou de stagear — não só os que A pediu. Os dois sub-agents desse dispatch pegaram isso sozinhos (via `git show --stat HEAD` depois de commitar) e corrigiram com `git reset --soft HEAD~1` + `git restore --staged` nos arquivos do outro.
**Solution:** nenhuma ação corretiva nesse caso — ambos sub-agents detectaram e resolveram por conta própria antes de reportar. Mas o padrão vai se repetir em todo dispatch paralelo futuro.
**Prevents:** ao dispatchar tasks paralelas que fazem commit próprio, instruir explicitamente pra usar `git commit -- <arquivo1> <arquivo2> ...` (commit com pathspec, escopado só aos arquivos listados) em vez de `git commit` puro — elimina a janela de corrida sem precisar de `git show --stat` defensivo depois.

## Active Blockers

### B-001: `-race` quebrava por CC=clang injetado no processo shell (2026-07-12) — RESOLVIDO

**Discovered:** T2, no gate check (`clang: error: unsupported option '-mthreads' for target 'x86_64-pc-windows-msvc'`).
**Impact:** bloquearia o Gate de toda task Go daqui em diante (não era específico do código de T2 — reproduzido pelo evaluator, erro ocorre compilando `runtime/cgo` antes de qualquer código do projeto).
**Root cause:** processo que spawna cada shell da sessão injeta `CC=clang` (target MSVC) — não é variável User/Machine persistida do Windows (essas estavam vazias), então nem `go env -w` nem `setx`/`SetEnvironmentVariable` conseguem sobrepor sem reiniciar a sessão do harness.
**Workaround:** MinGW-w64 instalado via `winget install BrechtSanders.WinLibs.POSIX.UCRT`; Gate command em TESTING.md agora prefixa `CC=gcc CXX=g++ PATH=".../mingw64/bin:$PATH"` inline em todo comando de teste. Confirmado funcionando (T2 passou com `-race` depois disso).
**Resolution:** definitivo seria reiniciar a sessão do harness pra ver se `CC=clang` some do processo — até lá, o prefixo inline no Gate command é a solução permanente-o-suficiente. Revisar quando/se reiniciar a sessão.

---

## Lessons Learned

### L-001: Go não permite parâmetro de tipo em método (2026-07-12)

**Context:** design inicial do metadata builder tentou `Object[AddressEntity]()` como método genérico.
**Problem:** Go só permite type parameter em func livre ou tipo, nunca em método — não compila.
**Solution:** metadata aninhada é capturada como valor (`addressMetadata := gonest.NewMetadata[AddressEntity](...)`) e passada explicitamente pra `Object(addressMetadata)`/`Items(addressMetadata)`, sem reflect e sem genérico em método.
**Prevents:** qualquer novo builder que precise "saber o tipo T" dentro de um método deve usar esse padrão (valor capturado), não tentar `.Method[T]()`.

---

## Quick Tasks Completed

_Nenhuma ainda._

---

## Deferred Ideas

- [ ] Abstração multi-adapter HTTP (net/http, Echo, Gin) — Captured during: definição de escopo v1
- [ ] Emitter/Scheduler/Terminus — Captured during: definição de escopo v1 (ver Future Considerations no ROADMAP.md)
- [ ] `gonest.FiberApp` como alias raiz de `internal/fiberapp.FiberApp` — Captured during: T5 de "App Bootstrap & Listen" (2026-07-13). Gap pré-existente (nenhuma feature anterior adicionou esse re-export) — INSIGHT.md usa `gonest.FiberApp` no call-site literal (`gonest.NewApp[gonest.FiberApp](...)`), mas hoje só `fiberapp.FiberApp` existe. Não bloqueou nada (testes usam o import direto), mas API pública fica incompleta até alguém adicionar `type FiberApp = fiberapp.FiberApp` num arquivo apropriado na raiz. Baixo custo, baixa prioridade — pegar quando mexer na raiz de novo por outro motivo, ou antes de considerar a API "pronta pra uso externo".
- [ ] `gonest.Context`/`gonest.Route`/`gonest.HttpGet` (e resto do enum `HttpMethod`) como aliases raiz — Captured during: T5 de "Middleware" (2026-07-13). Mesmo padrão do gap de `FiberApp` acima: `internal/httpctx.Context` e `internal/route.Route`/`HttpMethod` nunca ganharam re-export na raiz em nenhuma feature anterior, mesmo já sendo usados extensivamente (Route/Context são centrais desde "Controller & Route Registration"). Todo teste raiz que precisa desses tipos importa `internal/httpctx`/`internal/route` direto. Vale revisar TODOS os re-exports faltantes de uma vez (`FiberApp`, `Context`, `Route`, `HttpMethod`+constantes) antes de considerar a API pública "completa" — provavelmente uma única task de housekeeping, não uma feature própria.
- [ ] Renomear `MustResolve`→`MustInject` (e nomes públicos relacionados: `internal/resolve`→`internal/inject`) — Captured during: T8 evaluator. Motivo: evitar colisão de vocabulário com "Resolver" do GraphQL, caso o projeto suporte GraphQL no futuro. Nest já usa `@Injectable`/`@Inject`, `MustInject` fica mais alinhado. Usuário decidiu fazer o rename só depois de fechar a feature "Provider & DI Graph" inteira (T9-T11), não agora. `internal/resolver` (motor de grafo/DFS) NÃO precisa renomear — é implementação interna, não API pública.

---

## Todos

_Nenhum ainda._

---

## Preferences

**Model Guidance Shown:** never
