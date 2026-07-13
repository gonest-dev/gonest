# State

**Last Updated:** 2026-07-13
**Current Work:** Milestone 1 — "Provider & DI Graph" e "Module Composition" COMPLETE. Feature "Controller & Route Registration": T1-T7 done + migração AD-004, todos evaluator PASS, commit `53cd63f`. Próxima: T8 (`NewApp[T]` genérico + Stage 2.5).

---

## Recent Decisions (Last 60 days)

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
- [ ] Renomear `MustResolve`→`MustInject` (e nomes públicos relacionados: `internal/resolve`→`internal/inject`) — Captured during: T8 evaluator. Motivo: evitar colisão de vocabulário com "Resolver" do GraphQL, caso o projeto suporte GraphQL no futuro. Nest já usa `@Injectable`/`@Inject`, `MustInject` fica mais alinhado. Usuário decidiu fazer o rename só depois de fechar a feature "Provider & DI Graph" inteira (T9-T11), não agora. `internal/resolver` (motor de grafo/DFS) NÃO precisa renomear — é implementação interna, não API pública.

---

## Todos

_Nenhum ainda._

---

## Preferences

**Model Guidance Shown:** never
