# State

Last synced commit: c8e05a8
**Last Updated:** 2026-08-20

## Current Work

**Current Work:** guard novo contra `MustInject`/`MustInjectAll` chamado de DENTRO de um
`Constructor` (em vez do builder fn que o antecede) -- achado real do usuário ("eu mesmo já fiz
isso"). Antes desta mudança, esse engano nunca dava erro: o `PendingEdge`/`PendingAllEdge`
registrado durante a execução do `Constructor` (Stage 3, quando o grafo de dependência já foi
montado e todo goroutine já foi despachado) nunca era consultado por nada -- o placeholder/slice
retornado ficava permanentemente zerado/vazio, sem panic, sem erro, silêncio total. Fix:
`internal/inject.resolving` (atomic.Bool) marcado `true` por `internal/resolver/stage3.go`'s
`resolveGraph` durante toda a janela de Stage 3 (`defer` desmarca ao sair), checado no topo de
`Must[T]`/`MustAll[T]` (depois do short-circuit de `GlobalSingletonFor`, que é imediato e sempre
seguro) -- panic imediato e explícito nomeando o tipo e explicando a causa/fix, em vez de falha
silenciosa. Testado: unit (`internal/inject`, seta a flag manualmente, prova panic com a flag
ligada e comportamento normal desligada) + integração real (`internal/app`, `Constructor` chamando
`inject.Must` de dentro de si mesmo, prova que `New()` retorna `error` contendo a mensagem --
`callConstructor` já converte panic em erro via `recover()`, então não é um panic cru saindo de
`New()` -- e que `MustNewApp()` panica). `go test ./... -race -count=1` verde, 25 pacotes. Site
(`site` repo) ainda precisa de nota explícita sobre esse erro na doc de dependency-injection.
Ver AD-064.

**Current Work (histórico, superado acima):** Provider-side `MustInjectAll` feature (`.specs/features/provider-must-inject-all/`)
**COMPLETE** (Milestone 27, T1-T10 executados via pipeline PO→DEV→QA em subagentes, T1-T4
sequenciais no mesmo arquivo `internal/inject/inject.go`, T5/T6 em paralelo genuíno (arquivos
diferentes, `graph.go`/`stage3.go`), T7-T10 sequenciais no mesmo arquivo novo de teste de
integração. PO revisou tasks.md antes do DEV começar, achou 1 gap real (REQ-004 sem doc comment
explícito cobrindo "ordem não garantida") -- corrigido em tasks.md antes de qualquer código.
QA rodou suite inteira (`go test ./... -race -count=1`, 24 pacotes, 932 subtests verdes) +
repetição `-race -count=10` nos 3 pacotes tocados (zero flake) + auditoria de 3 cenários de risco
que não estavam em nenhum "Done when" explícito (mesmo Provider casado em 2 `PendingAllEdge`
diferentes; `OwnerModule()` nil antes de Stage 1 -- confirmado inalcançável via API pública;
`AssignableTo` mascarando bug -- confirmado morto-mas-defensivo, `validateProviderAsRefs` já
garante assignability antes de Stage 3 chegar lá) -- zero bug real achado, zero fix necessário.
Ver AD-063.
**Current Work (histórico, superado acima):** Provider-side `MustInjectAll` feature
**DESIGNED** (spec.md + design.md prontos, Tasks pendente). Design fechado: reusa o MESMO
mecanismo de indireção reflect que `MustInject[T]` ponteiro já usa (endereço estável) -- só que
generalizado pra N slots: `reflect.MakeSlice(SliceOf(T), N, N)` alocado JÁ na chamada (N =
candidatos achados agora, árvore de módulos já montada em Stage 2), devolvido imediatamente, e
`Stage 3` escreve em `slice.Index(i)` (sempre endereçável mesmo sem o slice ter vindo de um
ponteiro) quando cada candidato termina -- cópias do slice header compartilham array, então quem
já recebeu o retorno enxerga os valores reais depois. Novo: `PendingAllEdge`/`findAllRefs`/
`mustAllProvider` em `internal/inject/inject.go` (duplicando busca own+imports por CICLO de
import, mesmo padrão que `mustLazy` já usa -- `internal/resolver` importa `internal/inject`, não
dá pra inverter); `findAllRefs` desembrulha `providerAsRef` (`InnerRef()`) ANTES de gravar em
`Matches` -- se não desembrulhar, `BuildGraph`/`invokeAndCopy` (que operam sobre o ref CONCRETO,
nunca o view) quebrariam ordenação SILENCIOSAMENTE. `internal/resolver/graph.go`'s `BuildGraph` e
`stage3.go`'s `invokeAndCopy` ganham loops novos (aditivos, custo zero quando a feature não é
usada). Transient rejeitado com panic fail-fast ainda em Stage 2 (dentro de `mustAllProvider`),
mesma postura de `mustLazy`'s LAZY-07. `gonest.go`'s `MustInjectAll[T]` público confirmado sem
mudança (já passthrough genérico). Próxima sessão: Tasks phase. Motivada por `INSIGHT-MUST-INJECT-ALL.md` (raiz) --
usuário quer `gonest.MustInjectAll[port.Pingable](p)` funcionando DENTRO do builder fn de um
`gonest.NewProvider` (hoje só funciona a partir de Controller/Middleware/Guard/Interceptor/Filter,
via `directResolver`; `*provider.Provider` explicitamente rejeitado em `internal/inject.MustAll`).
Design da solução (discuss via `AskUserQuestion`, decisões já fechadas no spec.md): generalizar o
mecanismo de placeholder+edge diferido que `MustInject[T]` ponteiro já usa Provider-a-Provider --
slice de comprimento FIXO alocado no momento da chamada (árvore de módulos já assemblada nesse
ponto de Stage 2, dá pra contar quantos Providers no escopo implementam `T` antes de Stage 3
resolver qualquer valor), cada elemento escrito in-place por Stage 3 via `slice.Index(i).Set(real)`
quando o Provider casado correspondente termina. Decisões do usuário: (1) só `scope.Singleton` pode
ser membro do slice -- Transient fica fora de escopo, panic fail-fast se casar (sem caso de uso real
hoje, mesma postura de `mustLazy`'s LAZY-07); (2) ordem do slice NÃO garantida; (3) zero matches =
slice vazio, sem panic (mesmo contrato do `MustInjectAll` via `directResolver` já existente).
Componentes afetados identificados (leitura direta de código, `.specs/graph/graph.json` ainda
ausente neste projeto -- modo degradado do skill graph-spec-design, sem `graphify query` disponível):
`internal/inject/inject.go` (`Must`/`MustAll` dispatch, `PendingEdge` novo tipo multi-valor),
`internal/resolver/stage3.go` (`invokeAndCopy`/grafo de dependência precisam expandir a aresta
owner→interface em N arestas owner→matched), `internal/resolver/direct.go` (`findDirectMatches`
tem a lógica de precedência estrutural a reusar, mas operando sobre `module.ProviderRef` não-
resolvido). Próxima sessão: Design phase.

STATE.md compactado nesta sessão (237KB→19.7KB, protocolo `state_compaction.md`) -- histórico
completo preservado verbatim em `STATE_ARCHIVE.md` (era caso de "Legacy Migration", seções sem
windowing "(Last N)"). `.env` do projeto tem `GEMINI_API_KEY` -- exportar antes de rodar
`graph-spec-design .`/`--update` quando o grafo for inicializado (ainda não foi nesta sessão).

**Current Work (histórico, superado acima):** Logger feature (`.specs/features/logger/`) **COMPLETE**. Motivada por dogfooding
fora deste repo (`erc/ctrl/api`) -- consumidor tentou `gonest.MustInject[port.Logger](p)` dentro do
`Constructor` de outro Provider e panicou (`MustInject[T]` Provider-a-Provider só aceita `T` pointer,
nunca interface -- regra pré-existente, não bug novo). `internal/logger` deixou de ser só funções de
pacote com formato fixo: `Logger` (interface pública, 5 severidades, `meta ...map[string]any`
opcional) + `active Logger` trocável (`consoleLogger` default = formato de sempre) +
`contextLogger` (prefixa `[name]`). `AppOptions.Logger` novo -- troca no FACTORY (`NewApp(root,
AppOptions{Logger: instance})`, sem `app.UseLogger()` separado -- decisão do usuário, gonest não tem
a janela assíncrona que motiva o 2-passos do Nest), `nil` mantém o console default; `MustNewTestApp`
(sem `Options`) sempre reseta pro default, nunca vaza logger customizado entre bootstraps do mesmo
processo. `gonest.GetLogger(optionalNamedContext ...string)`/`gonest.GetLoggerFor[T any]()` novos --
acesso DIRETO ao `active` (função de pacote, não `MustInject`), funciona de QUALQUER lugar incluindo
dentro de `Constructor` de Provider, onde `MustInject[interface]` sempre foi (e continua sendo)
inválido -- resolve o caso motivador sem tocar a regra pointer-only do DI. Ecosystem trace completo
(`spec.md`): rastreados TODOS os 10 `recover()` do codebase; 8 estavam 100% silenciosos
server-side (T1 `fiber.go` RegisterRoute -- todo panic de request HTTP; T2 `graphql/generate.go`
Field.Resolve -- todo panic de Query/Mutation; T3, 4 sites de streaming SSE/WS que faziam `_ =
recover()` puro; T4 `resolver/stage3.go`+`provider/lifecycle.go`, já propagavam erro mas sem log
estruturado), agora todos chamam `logger.Error`/`logger.GetLogger(ctx)` antes de converter em
resposta/erro -- só o branch de `exception.Exception` (erro de negócio esperado) fica de fora de
propósito, pra não virar ruído. T5: `emitter`/`scheduler` (já logavam) upgradados pra
`logger.GetLogger(nomeRuntime)` -- usa string, não `GetLoggerFor[T]` genérico, porque o contexto ali
é um `reflect.Type`/`name string` só conhecido em runtime, não um type param compile-time (achado
durante a execução, corrigido no spec.md). `.specs/insight/LOGGER.md` tem o rascunho completo
(de/para Nest, decisões de design, exemplos). `go build`/`go vet`/`go test ./... -race -count=1`
verdes, 25 pacotes, zero assertion pré-existente mudada. Ver AD-062.

## Todos

- [ ] Nenhum pendente no momento (ver Deferred Ideas para trabalho futuro não priorizado).

## Active Blockers

### B-001: `-race` quebrava por CC=clang injetado no processo shell (2026-07-12) — RESOLVIDO

**Discovered:** T2, no gate check (`clang: error: unsupported option '-mthreads' for target 'x86_64-pc-windows-msvc'`).
**Impact:** bloquearia o Gate de toda task Go daqui em diante (não era específico do código de T2 — reproduzido pelo evaluator, erro ocorre compilando `runtime/cgo` antes de qualquer código do projeto).
**Root cause:** processo que spawna cada shell da sessão injeta `CC=clang` (target MSVC) — não é variável User/Machine persistida do Windows (essas estavam vazias), então nem `go env -w` nem `setx`/`SetEnvironmentVariable` conseguem sobrepor sem reiniciar a sessão do harness.
**Workaround:** MinGW-w64 instalado via `winget install BrechtSanders.WinLibs.POSIX.UCRT`; Gate command em TESTING.md agora prefixa `CC=gcc CXX=g++ PATH=".../mingw64/bin:$PATH"` inline em todo comando de teste. Confirmado funcionando (T2 passou com `-race` depois disso).
**Resolution:** definitivo seria reiniciar a sessão do harness pra ver se `CC=clang` some do processo — até lá, o prefixo inline no Gate command é a solução permanente-o-suficiente. Revisar quando/se reiniciar a sessão.

## Recent Decisions (Last 15)

### AD-064: guard fail-fast contra `MustInject`/`MustInjectAll` chamado dentro de `Constructor` (2026-08-20)

**Decision:** `internal/inject.resolving` (novo `atomic.Bool`, package-level) marcado `true` por
`internal/resolver/stage3.go`'s `resolveGraph` durante toda a janela síncrona+concorrente de
Stage 3 (`inject.MarkResolving(true)` antes do loop `errgroup.Go`, `defer
inject.MarkResolving(false)`), limpo também em `Reset()` por simetria com todo outro estado
process-global do pacote. `Must[T]`/`MustAll[T]` checam a flag logo no topo (depois do
short-circuit de `GlobalSingletonFor`, que resolve IMEDIATO sem depender do grafo de Stage 3, então
é seguro chamar de qualquer lugar) -- panic imediato, nomeando o tipo genérico exato e explicando
causa + fix, se a flag estiver ligada.
**Reason:** achado REAL do próprio usuário fora desta sessão ("eu mesmo já fiz isso") -- chamar
`MustInject`/`MustInjectAll` de DENTRO do `Constructor` (em vez do builder fn que roda ANTES de
`p.Constructor(...)` ser registrado) sempre foi um erro silencioso: o `PendingEdge`/`PendingAllEdge`
registrado nesse momento tarde demais nunca é consultado por Stage 3 (que já montou o grafo de
dependência e já despachou todo goroutine antes de qualquer `Constructor` rodar) -- o
placeholder/slice retornado fica permanentemente zerado/vazio, sem NENHUM sinal de erro em lugar
nenhum. `declareControllers` (fase 2, Controller/Middleware/Guard/Interceptor/Filter) só roda
DEPOIS que `resolver.Resolve` retorna (confirmado lendo `internal/app/app.go`), então a flag nunca
falso-positiva pro caminho `directResolver` -- só pega exatamente o caso do `Provider`.
**Trade-off:** nenhum técnico -- aditivo, 1 leitura atomic por chamada (custo desprezível),
zero mudança de comportamento pra qualquer chamada legítima (builder-fn-time). `callConstructor`
já tinha `recover()` convertendo panic em `error` -- então o efeito observável pro chamador de
`NewApp`/`New` é um `error` com mensagem clara (não um panic cru), e `MustNewApp`/`MustNewApp`
continuam panicando via seu próprio wrapper de sempre, sem mudança de contrato de erro.

### AD-063: Provider-side `MustInjectAll` -- slice pré-alocado + escrita in-place via reflect (2026-08-20)

**Decision:** `gonest.MustInjectAll[T](p)` (T interface) passa a funcionar de dentro do builder fn
de um `gonest.NewProvider`, reusando a MESMA classe de indireção reflect que `MustInject[T]`
ponteiro Provider-a-Provider já usa há muito tempo (endereço estável / elemento sempre
endereçável), generalizada de "1 placeholder" pra "N slots de um slice pré-alocado". Novo:
`PendingAllEdge`/`findAllRefs`/`mustAllProvider` em `internal/inject/inject.go` (busca duplicada
de `internal/resolver`, não reusada -- ciclo de import, mesmo precedente de `mustLazy`);
`internal/resolver/graph.go`'s `BuildGraph` e `stage3.go`'s `invokeAndCopy` ganham loops aditivos
consumindo `inject.PendingAllEdges()`. Só `scope.Singleton` pode ser membro do slice (Transient
panica fail-fast ainda em Stage 2); ordem do slice não garantida; zero matches retorna slice vazio
sem panic.
**Reason:** motivado por `INSIGHT-MUST-INJECT-ALL.md` -- usuário quer agregar TODOS os
`port.Pingable` (vários adapters de infra) num `HealthUsecase` sem montar `[]port.Pingable`
manualmente fora do DI. Antes desta feature, `MustInjectAll` só funcionava a partir de
Controller/Middleware/Guard/Interceptor/Filter (`directResolver`, valores já resolvidos em
phase 2/3) -- `*provider.Provider` era explicitamente rejeitado, porque Stage 2 (`Declare`, quando
o builder fn de um Provider roda) acontece ANTES de Stage 3 resolver qualquer valor real.
**Trade-off:** nenhum técnico -- puramente aditivo (loops novos condicionais a
`PendingAllEdges()` não-vazio, custo zero em bootstraps que não usam a feature). Escopo aceito
como restrito a Singleton (sem caso de uso real pra Transient nesta rodada, confirmado com o
usuário via discuss). Ver .specs/features/provider-must-inject-all/{spec,design,tasks}.md.



### AD-062: `gonest.Logger` pluggable + `GetLogger`/`GetLoggerFor` -- acesso direto, não via `MustInject` -- ecosystem-wide panic logging (2026-08-18)

**Decision:** `internal/logger` vira instância trocável (`Logger` interface, `active` package var,
`consoleLogger` default) em vez de funções fixas; `AppOptions.Logger` troca no factory
(`NewApp`/`MustNewApp`), `MustNewTestApp` sempre reseta pro default. `gonest.GetLogger(optionalName
...string)`/`GetLoggerFor[T]()` são funções de pacote lendo `active` DIRETO -- não passam por
`internal/inject.Must[T]`, então funcionam de qualquer lugar (Provider Constructor incluso) sem
esbarrar na regra "Provider-a-Provider só aceita T pointer". Todos os 10 `recover()` do codebase
auditados; os 8 que eram 100% silenciosos ganharam `logger.Error`/`logger.GetLogger(ctx)` antes de
converter panic em resposta/erro (só o branch `exception.Exception`, erro de negócio esperado, fica
de fora -- logar isso seria ruído, não sinal).
**Reason:** achado real dogfooding fora deste repo (`erc/ctrl/api`) -- consumidor tentou
`gonest.MustInject[port.Logger](p)` dentro do `Constructor` de outro Provider e panicou. Investigação
mostrou 2 problemas empilhados: (1) gonest não tinha jeito de trocar seu PRÓPRIO logger de
diagnóstico (banner/contadores), só `internal/logger` hardcoded; (2) mesmo se tivesse, injetar uma
INTERFACE via `MustInject` Provider-a-Provider nunca funcionaria (regra pointer-only pré-existente,
não bug) -- `GetLogger`/`GetLoggerFor` contornam isso de propósito, sendo accessor direto em vez de
resolução DI. Rastrear o ecossistema revelou que a maioria dos `recover()` do framework (HTTP
dispatch, GraphQL resolver, SSE/WS transports) não logava NADA server-side, só convertia panic em
resposta -- gap real, não hipotético (achado lendo código, confirmado com `grep`, não assumido).
**Trade-off:** nenhum técnico -- puramente aditivo, toda função/assinatura pré-existente de
`internal/logger` continua funcionando idêntico (mesmo formato default). `GetLoggerFor[T]` só serve
pra contexto conhecido em compile-time -- onde o contexto só existe em runtime (evento de
Emitter/nome de Scheduler), o T5 usa `GetLogger(nomeString)` em vez disso.
**Impact:** `internal/logger` (novo `Logger`/`active`/`consoleLogger`/`contextLogger`), `AppOptions.Logger`,
`gonest.GetLogger`/`GetLoggerFor[T]`, 8 sites de `recover()` server-side ganham log estruturado.
`.specs/insight/LOGGER.md` novo. `go build`/`go vet`/`go test ./... -race -count=1` verdes, 25 pacotes.

_(Ver `.specs/project/STATE_ARCHIVE.md` para AD-061 até AD-001 completos, incluindo AD-025/026/027/029.)_

### Índice das entradas AD-061..AD-048 mantidas nesta janela (cabeçalhos preservados para busca; texto completo Decision/Reason/Trade-off/Impact em `STATE_ARCHIVE.md` e no histórico git deste arquivo -- nenhuma decisão foi perdida, só reindexada pra manter STATE.md < 30KB):

- AD-061: `HttpContext` unifica `(req, res)` de volta num parâmetro só; `Response`→`Reply` (write-side), `RouteResponse`→`Response` (builder OpenAPI) -- Milestone 26 (2026-07-25)
- AD-060: `gonest.MustSetupSwagger` -- panic-on-error convenience wrapper (2026-07-24)
- AD-059: banner de startup mostra `localhost:PORT`, não `0.0.0.0:PORT`, pra addr sem host (2026-07-24)
- AD-058: `HttpException.Error()` cai pra JSON de `Details()` quando `Message()` nunca foi setado (2026-07-24)
- AD-057: Mensagem de panic de lifecycle hook nomeia provider (T) + assinatura recebida + assinaturas aceitas (2026-07-24)
- AD-056: `TokenRef` unifica TODOS os markers de `Module` (não só Providers/Exports) -- Milestone 25, T1 (2026-07-24)
- AD-055: Workflow Conventions formalizadas em PROJECT.md -- fala em pt-br, separação por milestone, tag por milestone, subagentes sempre, README.md + `.examples/` sempre atualizados, site sempre em sincronia (2026-07-23)
- AD-054: `Module.Lazy`/`gonest.LazyModule` -- Milestone 24, escolha de módulo de dentro do grafo de DI, `.examples/notification-driver` migrado (2026-07-23)
- AD-053: `gonest.ProviderAs[T]` -- fallback implícito `Implements()` removido, resolução de interface exclusivamente explícita; `Thing_` formalizado (2026-07-23)
- AD-052: `Module.Exports` unificado (`ExportableRef`) -- reverte a separação `Exports`/`ExportModules` do AD-051 (2026-07-22)
- AD-051: `Module.ExportModules` -- reexport transitivo de módulo inteiro, estilo NestJS (2026-07-21)
- AD-050: `gonest.ProviderRef`/`ControllerRef`/`ResolverRef`/`MiddlewareRef`/`FilterRef`/`ListenerRef`/`SchedulerRef` -- reexport dos marker interfaces do Module (2026-07-21)
- AD-049: `gonest.SchemaFor[T]()` -- lookup por reflection do schema já registrado (2026-07-21)
- AD-048: `findFieldByOffset` -- 2 bugs reais de identificação de campo promovido em embed multi-nível/multi-campo (2026-07-21, achado dogfooding pós-AD-047)

## Recent Progress (Last 10)

- [2026-08-18] Duration Branch feature (`PropertyBuilder.Duration()`, kind `"duration"`, validação Min/Max/Enum via `time.ParseDuration`) complete. Gate: `go test ./... -count=1` verde, 25 pacotes.
- [2026-07-23] Milestone 24 (Module Lazy Loading) T1-T8 complete (`Module.Lazy`, `inject.Must[T]` 3º branch `mustLazy`). Gate: `go test ./... -race -count=1` verde, 24 pacotes. Ver AD-054.
- [2026-07-23] Milestone 22 (Provider Interface Export) + Milestone 23 (Thing_ Naming Convention) complete. Gate: `go test ./... -race -count=1` verde. Ver AD-053.
- [2026-07-21] Pós-Milestone 21: `.examples/full-text-search` ganhou Swagger/OpenAPI completo; 2 bugs reais corrigidos em `findFieldByOffset`. Gate: `go test ./... -race -count=1` verde, 24 pacotes core. Ver AD-048.
- [2026-07-21] Milestone 21 (Enum Branches) T1-T4 complete (`StringSchema.Enum`/`NumericSchema.Enum`). Gate: `go test ./... -race -count=1` verde, 24 pacotes. Ver AD-047.
- [2026-07-20] Housekeeping pós-Milestone 20: renames públicos (`LogLevel`→`LoggerLevel`, `GenerateOpenApiSchema`→`OpenapiGenerate`), `internal/value`→`internal/accessor`. Gate: `go build`/`go vet`/`go test ./...` verdes. Ver AD-046.
- [2026-07-19] Milestone 20 (Lifecycle Hooks) T1-T7 complete (5 hooks 1:1 com NestJS, confirmados via Context7). Gate: `go test ./... -race` verde, 25 pacotes. Ver AD-044.
- [2026-07-19] Milestone 19 (Config Loading) T1-T12 complete (`internal/dotenv`, `gonest.Dotenv()`, `envSource`/`ParseEnvInto`). Gate: `go test ./... -race` verde, 25 pacotes. Ver AD-043.
- [2026-07-19] Milestone 19 "Config Loading" especificada (spec/context prontos, Design/Tasks/Execute pendentes nesse ponto). Ver AD-042.
- [2026-07-18] `internal/appoptions` removido -- `Options` vive em `internal/app`, ciclo de import quebrado com `RegisterTestAdapter`. Gate: `go test ./... -race` verde, 25 pacotes. Ver AD-041.

## Lessons Learned (Last 5)

### L-008: `Context.route any` + assertion — deveria ter sido interface tipada (2026-07-13)

**Context:** T5 (feature Controller & Route) precisou ligar `httpctx.Context` a `*route.Route` (pra `MustParam` checar `HasParam`), mas `internal/route` já importa `internal/httpctx` — importar de volta ciclaria. Solução usada: `Context.route any` + type assertion em `route.MustParam`.
**Problem:** evaluator apontou que uma interface pequena definida DENTRO de `httpctx` (`type paramHost interface { HasParam(string) bool }`), satisfeita estruturalmente por `*route.Route`, resolveria o mesmo ciclo com segurança de tipo em compile-time — sem `any` + assertion que degrada silenciosamente pra "sem rota" em qualquer type mismatch.
**Solution:** não corrigido ainda (não bloqueia T5) — registrado como débito. Só 1 call site (`route.MustParam`) usa isso hoje, fica mais barato de trocar agora do que depois que mais coisa acoplar.
**Prevents:** quando precisar ligar 2 pacotes que já têm import na direção oposta (evitar ciclo), preferir interface pequena definida no pacote "de baixo" (satisfeita estruturalmente pelo de cima) em vez de `any`+assertion — mesmo custo de acoplamento, ganha segurança de tipo.

### L-009: Fiber v3 `Ctx.Params()` devolve view zero-copy sobre buffer reusado — precisa `strings.Clone` (2026-07-13)

**Context:** T9 (exemplo end-to-end `UserController`) escreveu `UserService.Create(name)` guardando `name` (vindo de `ctx.Param`→`fiberResponder.GetParam`→`fiber.Ctx.Params()`) num campo de struct persistido além da vida da request. Testes ficaram flaky, valor corrompido (`"1da"` em vez de `"Ada"`).
**Problem:** `fiber.Ctx.Params()` (fasthttp por baixo) devolve string sobre um buffer reusado entre requests — doc do Fiber diz explicitamente "Returned value is only valid within the handler... Make copies to use the value outside the Handler". `fiberResponder.GetParam` repassava o valor cru, sem copiar.
**Solution:** `GetParam` agora faz `strings.Clone(r.c.Params(name))`. Evaluator do T9 reproduziu o bug de propósito (reverteu o fix, rodou o teste novo 30x, 19 falhas com o padrão exato de corrupção previsto) antes de confirmar o fix.
**Prevents:** qualquer novo código em `internal/fiberapp` (ou adapter HTTP futuro) que leia string de `fiber.Ctx` e a repasse pra fora do escopo da request precisa copiar explicitamente — não assumir que string do Fiber/fasthttp é segura pra reter.

### L-010: `httpctx.Context.Header()`/`SetHeader()` são stores diferentes (request vs response) — não dá pra "ler, concatenar, escrever de volta" (2026-07-13)

**Context:** T4 da feature "Middleware" tentou provar ordem de execução de múltiplos middlewares fazendo cada um ler o header atual, concatenar um marcador, escrever de volta (`ctx.Header(...)` → `ctx.SetHeader(...)`).
**Problem:** `Context.Header(name)` delega pra `Responder.GetHeader` → lê o header da REQUEST recebida, não da resposta sendo construída. `Context.SetHeader` escreve no header da RESPONSE. São dois stores completamente diferentes — "ler o que acabei de escrever" nunca funciona através de `Header()`/`SetHeader()`.
**Solution:** T4 adaptou pra 3 técnicas alternativas via dispatch real (`app.Test`): slice compartilhado via closure, `resp.Header` do `*http.Response` real, e `ctx.WithRoute`/`ctx.Route()` reaproveitado como carrier de valor arbitrário.
**Prevents:** qualquer teste futuro que precise "ler o header que acabei de setar, dentro da mesma request" precisa saber que isso não é possível hoje via `Context` público — precisaria de um método novo tipo `Context.GetResponseHeader`, que não existe.

### L-011: design.md descreveu ordem de composição Guard/Interceptor invertida — achado só na revisão pós-implementação (2026-07-13)

**Context:** design.md da feature "Interceptor" especificou que a chain de interceptors envolveria a saída de `gatedHandler` (que já contém Guards + Handler). O dev sub-agent seguiu o algoritmo à risca.
**Problem:** essa composição produz ordem de EXECUÇÃO real Middleware → Interceptor(before) → Guard → Handler → Interceptor(after) — contradiz a ordem documentada (Middleware → Guard → Interceptor → Pipe → Handler) e o próprio propósito de um Guard: rejeitar a request antes de QUALQUER trabalho subsequente.
**Solution:** corrigido design.md (Interceptor envolve o `routeHandler` bruto; Guard envolve o RESULTADO, ficando mais externo). `internal/app/app.go` corrigido (troca de ordem de wrapping) + testes.
**Prevents:** ao desenhar composição de múltiplos estágios de pipeline que se envolvem uns aos outros, sempre TRAÇAR a ordem de EXECUÇÃO resultante (não só "quem chama quem no código") e comparar contra a ordem documentada ANTES de mandar pro developer. Sub-agents que notam divergência entre prosa e código do próprio design devem SEMPRE reportar como SPEC_DEVIATION explícito.

### L-012: `Pipe.Declare()` e `ctx.WithRoute()` nunca eram chamados em produção — só testes internos "trapaceavam" chamando manualmente (2026-07-13)

**Context:** ao especificar a feature "Pipe", descoberto que `internal/pipe` JÁ EXISTIA — só faltava re-export raiz. O primeiro teste que reproduz `ParseIntPipe` via dispatch HTTP REAL falhou de duas formas sucessivas.
**Problem:** (1) `pipe.New(fn)` defere `fn` até `Declare()` rodar — mas nada no bootstrap de DI percorria Pipes anexados a Routes pra declará-los. (2) `ctx.WithRoute(route)` nunca era chamado no fluxo real de dispatch — `MustParam[T]` depende disso pra achar Pipe customizado; sem isso, sempre caía no `defaultCoerce` genérico, ignorando silenciosamente qualquer Pipe customizado.
**Solution:** `Route.Param(name, p)` agora chama `p.Declare()` antes de armazenar. `internal/app`'s `registerRoutes` envolve o handler final com uma camada que chama `ctx.WithRoute(currentRoute)` antes de middleware/guard/interceptor/handler.
**Prevents:** (a) qualquer tipo com padrão `New(fn)` deferido precisa ter, no fluxo de PRODUÇÃO, uma chamada real a `Declare()` — se só teste chama manualmente, é sinal de gap; (b) testes que só validam uma peça isolada nunca vão pegar um bug de fiação entre peças — só um teste que sobe a app inteira e dispara request HTTP real prova que as peças estão conectadas.

## Quick Tasks Completed

_Nenhuma ainda._

## Deferred Ideas

- [ ] Abstração multi-adapter HTTP (net/http, Echo, Gin) — Captured during: definição de escopo v1
- [ ] Emitter/Scheduler/Terminus — Captured during: definição de escopo v1 (ver Future Considerations no ROADMAP.md)
- [ ] `gonest.FiberApp` como alias raiz de `internal/fiberapp.FiberApp` — Captured during: T5 de "App Bootstrap & Listen" (2026-07-13). Gap pré-existente — INSIGHT.md usa `gonest.FiberApp` no call-site literal, mas hoje só `fiberapp.FiberApp` existe. Baixo custo, baixa prioridade — pegar quando mexer na raiz de novo por outro motivo.
- [ ] `gonest.Context`/`gonest.Route`/`gonest.HttpGet` (e resto do enum `HttpMethod`) como aliases raiz — Captured during: T5 de "Middleware" (2026-07-13). Mesmo padrão do gap de `FiberApp` acima. Vale revisar TODOS os re-exports faltantes de uma vez antes de considerar a API pública "completa".

## Preferences

**Model Guidance Shown:** never

**Commit convention (2026-07-15):** every commit, every iteration/task, MUST be in en-US, using Conventional Commits format (`type(scope): summary`, e.g. `feat(emitter): add Emitter/Listener/MustOn`, `docs(specs): close Emitter feature`, `refactor(app)!: ...` for breaking changes). Types used so far: `feat`, `fix`, `refactor` (with `!` suffix for breaking changes), `docs`. Body (when present) explains WHY, not WHAT. Applies regardless of what language the conversation itself is happening in (repo conversations are often pt-BR, but commits stay en-US).

**Subagent workflow convention (2026-07-17, applies to EVERY feature from now on):** toda `tasks.md` é escrita pensando em 3 papéis de subagente distintos, delegados via o `Agent` tool:
- **Planner** -- roda UMA vez por feature, produz `tasks.md`: tasks atômicas, `[P]` paralelizáveis, `Depends on`/`Reuses`/`Done when`/`Tests`/`Gate`/`Commit` por task, granular o bastante pra virar o prompt INTEIRO de um Implementer.
- **Implementer** -- um subagente POR task. Recebe SÓ a definição daquela task -- NUNCA recebe outras tasks, histórico da conversa, nem relatórios de avaliação anteriores. Reporta: Status, Files changed, Gate check result, SPEC_DEVIATION.
- **Evaluator** -- roda DEPOIS de cada Implementer. Recebe a definição da task + o diff real (nunca confia só na alegação): confere `Done when`, `Gate` (roda o comando), SPEC_DEVIATION silencioso. Aprova ou devolve com motivo específico -- NUNCA corrige o código ele mesmo.
Motivo: cada papel roda com contexto MÍNIMO -- evita que um mesmo agente "marque a própria lição de casa".

**Site sync convention (2026-08-22, OBRIGATÓRIO, ver PROJECT.md):** toda mudança de API
pública/assinatura/exemplo dispara atualização do repo irmão
`C:\dev\github.com\gonest-dev\site` no MESMO milestone (nunca débito posterior) — mínimo
`content/docs/api-reference/*.mdx` + `content/docs/core-concepts/*.mdx` relevantes, nos 3
idiomas (en=`.mdx`, pt=`.pt.mdx`, es=`.es.mdx`). Motivado por drift real achado 2026-08-22:
`api-reference/controller.*.mdx` (3 idiomas) ainda mostrava a API pré-refatoração
(`RouteGet(path, func(c *HttpContext))`, sem `r.Handler`, `MustInject[T](req)` em vez de
`MustInject[T](r)`) — 2+ milestones sem sync porque a regra anterior citava o path errado do
site (`C:\dev\gonest-dev\site`, faltando `github.com`).

**Versioning convention (2026-07-15/16):** `{major}.{minor}.{release}` sob `v0` fixo PRA SEMPRE (nunca incrementa pra `v1`) -- `major` em BREAKING change, `minor` em FEATURE nova, `release` em FIX/patch. Tag real é semver 3-segmentos com pontos (`v0.6.0`, não `v0.6-0.0` -- confirmado inválido via `golang.org/x/mod/semver.IsValid`). Primeira tag começou em `0.6.0` (não `0.1.0`), refletindo os 6 commits breaking (`!`) já em git log antes da tag existir.
