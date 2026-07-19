# Config Loading (Milestone 19) — Tasks

**Covers TWO features in ONE `tasks.md`** (same pattern `graphql-realtime-protocols/tasks.md`
already used to cover multiple protocol "tracks" in a single file):

- **Track A: Dotenv Loading** — `.specs/features/dotenv-loading/{spec,context,design}.md`
  (DOTENV-01..06)
- **Track B: Env → Schema Binding** — `.specs/features/env-schema-binding/{spec,context,design}.md`
  (ENVCFG-01..03) — depends on Track A

**Canonical copy**: this file, at `.specs/features/dotenv-loading/tasks.md`. The identical file
also lives at `.specs/features/env-schema-binding/tasks.md` (copy, not a symlink — Git has no
portable symlink guarantee across this repo's dev environments) — if the two ever diverge, THIS
file (`dotenv-loading/tasks.md`) is the source of truth; update the other copy to match.

**Status**: Draft

---

## Subagent Roles (ver `.specs/project/STATE.md`'s "Subagent workflow convention")

- **Planner** — já rodou (esta sessão) pra produzir este `tasks.md`. Não roda de novo pra
  nenhuma das 2 features a menos que o escopo mude.
- **Implementer** — 1 subagente por task abaixo (ou por grupo `[P]` em paralelo). Recebe SÓ a
  definição daquela task (What/Where/Depends on/Reuses/Done when/Tests/Gate/Commit) — NUNCA as
  outras tasks, o histórico desta conversa, nem relatórios de avaliação de tasks anteriores.
  Reporta: Status (Complete/Blocked/Partial), Files changed, Gate check result, SPEC_DEVIATION
  (se houver).
- **Evaluator** — roda DEPOIS de cada Implementer, ANTES de marcar a task como concluída. Recebe
  a definição da task + o diff real (nunca só o relatório do Implementer sozinho) e confere:
  `Done when` bate item a item, `Gate` passou de verdade (roda o comando, não assume), nenhum
  `SPEC_DEVIATION` foi introduzido silenciosamente. Aprova (task vira `completed`) ou devolve pro
  Implementer com o motivo específico — NUNCA corrige o código ele mesmo.

Todo prompt de Implementer deve incluir: a task inteira (What/Where/Depends on/Reuses/Done
when/Tests/Gate/Commit), `.specs/codebase/TESTING.md` (não há `CONVENTIONS.md` ainda neste repo),
e o trecho relevante de `spec.md`/`context.md`/`design.md` da feature correspondente referenciado
pela task.

---

## Execution Plan

### Track A: Dotenv Loading — Sequential (é a base de tudo, inclusive de Track B)

```
T1 -> T2 -> T3 -> T4 -> T5 -> T6 -> T7 -> T8
```

### Track B: Env → Schema Binding — parcialmente paralela à Track A

```
T9 [P] (paralelo a T2..T8, só depende de T1 existir)
T9 -> T10 -> T11
```

### Merge: gate final

```
T7 + T8 + T11 ─> T12
```

---

## Task Breakdown

### T1: `internal/dotenv` pacote novo — `Dotenv` struct + singleton `Get()` + esqueleto `Load`/`MustLoad`

**What**: Criar o pacote `internal/dotenv` (novo, zero dependência de framework — só stdlib).
Arquivo `internal/dotenv/dotenv.go`: `type Dotenv struct{}` (sem campos por enquanto — ganha
`ParseInto` na Track B, mas o TIPO precisa existir já aqui pra tudo mais compilar), um singleton
de pacote (`var instance = &Dotenv{}`) e `func Get() *Dotenv { return instance }` (NÃO um
`New()`-style construtor — existe exatamente UMA instância, dona do framework, mesma forma de
chamada que `gonest.Dotenv().Load(...)` exige, nunca `gonest.NewDotenv().Load(...)`).
`(*Dotenv) Load(paths ...string) error` e `(*Dotenv) MustLoad(paths ...string)` nesta task são
ESQUELETO: `Load` já implementa a extremidade (`os.ReadFile` por path, retornando o erro do
próprio `os.ReadFile` wrapeado como `"gonest: dotenv load %q: %w"` se o arquivo não existir — cobre
sozinho DOTENV-01 AC3) mas delega o PARSE do conteúdo do arquivo pra uma função `parseFile(raw
[]byte) ([]envPair, error)` que ainda NÃO existe de verdade nesta task — criar um stub que retorna
`nil, nil` (nenhum par, sem erro) só pra `Load` compilar e rodar ponta a ponta sem nenhum efeito
observável ainda (T2 substitui o stub pela implementação real). `MustLoad` chama `Load` e
`panic(err)` se não-nil — mesma convenção "Must-prefixed panics" de `MustParse`/`MustNewApp` em
`gonest.go`.
**Where**: `internal/dotenv/dotenv.go` (novo), `internal/dotenv/parse.go` (novo, só o stub
`parseFile` + `type envPair struct{ Key, Value string }`)
**Depends on**: None
**Reuses**: nada (pacote genuinamente novo) — NÃO importar `internal/inject`/`internal/module`/
qualquer pacote de DI, esse é o ponto central da feature (chamável de `main()` antes de qualquer
bootstrap existir, `dotenv-loading/design.md`'s Architecture Overview)
**Requirement**: DOTENV-01 (parcial — a parte de "arquivo não existe retorna erro" e "MustLoad
panica")

**Done when**:
- [ ] `dotenv.Get() *Dotenv` retorna sempre a MESMA instância (`==` entre duas chamadas)
- [ ] `Load("./caminho/que/nao/existe.env")` retorna um `error` não-nil, NÃO panica
- [ ] `MustLoad("./caminho/que/nao/existe.env")` panica
- [ ] `Load` com um path que EXISTE (mesmo vazio) não retorna erro (o stub `parseFile` retorna
      `nil, nil`, então nada é setado ainda — comportamento observável só chega em T2+)
- [ ] `internal/dotenv` não importa nenhum pacote de `internal/inject`, `internal/module`,
      `internal/provider`, `internal/resolver`, `internal/app` (confirmar via
      `go list -deps ./internal/dotenv/...` não listando nenhum desses)
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestGet_ReturnsSameSingletonInstance`, `TestLoad_NonexistentPath_ReturnsError`,
`TestMustLoad_NonexistentPath_Panics`, `TestLoad_ExistingEmptyPath_NoError` (novos,
`internal/dotenv/dotenv_test.go`, usando `t.TempDir()` pra criar um arquivo `.env` vazio real)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): Dotenv singleton, Load/MustLoad skeleton over stubbed parser`

---

### T2: `parseFile` real — classificação de linha + dispatch por tipo de aspas (SEM interpolação/escape ainda)

**What**: Substituir o stub `parseFile` (T1) pela implementação real de classificação de linha, em
`internal/dotenv/parse.go`. `parseFile(raw []byte) ([]envPair, error)`: divide `raw` em linhas
(`\n`, tolerando `\r\n` — trim do `\r` final de cada linha antes de qualquer outra coisa). Pra cada
linha (ordem preservada — `design.md`'s nota sobre interpolação referenciar chave definida ANTES
na mesma carga, `[]envPair` não `map`, precisa vir em T3 mas a ORDEM já precisa estar certa aqui):
trim de espaço em branco à ESQUERDA apenas (trailing fica pro parse do valor); linha vazia após
esse trim → pular; linha começando com `#` → pular (comentário de linha inteira); senão, deve
conter `=` — a parte ANTES do primeiro `=` (trimmed) é a key, o resto é a expressão de valor crua
passada pra `parseValue`. Linha sem `=` (e não vazia, não comentário) → `parseFile` retorna erro
`"gonest: malformed line %d in dotenv: %q"` imediatamente (fail loud, `design.md`'s "Open Edge
Cases Resolved" table — este é o único requirement de erro desta task, mesmo que
`Load`/`os.ReadFile`'s próprio wrapping de erro de arquivo já exista desde T1). `parseValue(raw
string) (value string, err error)` NESTA task: dispatch pelo caractere de ABERTURA do valor —
`` ` `` (backtick), `'` (aspas simples), `"` (aspas duplas), ou bare (qualquer outro char) — mas
SÓ extrai o conteúdo cru delimitado (lê até a aspas/backtick de fechamento correspondente NÃO
escapada, ou até fim de linha pro caso bare), SEM aplicar interpolação, SEM aplicar escapes de
`\n`/`\t`/etc, SEM stripar comentário inline, SEM suportar multiline de verdade no backtick ainda
(essas 4 capacidades são T3/T4/T5/T6 — esta task só prova que o CARACTERE certo dispara o branch
certo e o conteúdo cru é extraído até o delimitador certo). Aspas/backtick sem fechamento antes do
fim da linha (backtick sem fechamento antes do fim do ARQUIVO, já que backtick é multiline por
natureza — mas nesta task ainda não é multiline de verdade, então "fim da linha" mesmo) → erro
`"gonest: unterminated quote in dotenv, line %d"`.
**Where**: `internal/dotenv/parse.go`
**Depends on**: T1
**Reuses**: `envPair` (T1's stub, agora usado de verdade)
**Requirement**: DOTENV-01 (linha malformada), DOTENV-03 (dispatch de aspas simples/duplas/bare —
a PARTE de "reconhecer o delimitador certo", a interpolação em si é T3)

**Done when**:
- [ ] `parseFile` classifica corretamente: linha em branco (ignorada), linha `# comentário` (linha
      inteira ignorada), linha `KEY=VALUE` (vira 1 `envPair`)
- [ ] Linha sem `=` retorna erro `parseFile`, identificando o número da linha
- [ ] `parseValue` extrai o conteúdo cru correto pra cada delimitador: bare (`FOO=bar` → `bar`),
      aspas simples (`FOO='bar'` → `bar`, aspas removidas), aspas duplas (`FOO="bar"` → `bar`,
      aspas removidas), backtick de uma linha só (`` FOO=`bar` `` → `bar`, backticks removidos)
- [ ] Aspas/backtick sem fechamento retorna erro identificando a linha
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestParseFile_BlankLine_Skipped`, `TestParseFile_WholeLineComment_Skipped`,
`TestParseFile_MissingEquals_ReturnsError`, `TestParseValue_Bare_ExtractsRaw`,
`TestParseValue_SingleQuoted_StripsQuotes`, `TestParseValue_DoubleQuoted_StripsQuotes`,
`TestParseValue_Backtick_StripsBackticks`, `TestParseValue_UnterminatedQuote_ReturnsError` (novos,
`internal/dotenv/parse_test.go`)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): parseFile line classification + quote-style dispatch (no interpolation yet)`

---

### T3: Interpolação (`${VAR}`/`$VAR` + 4 operadores default/alternate) — bare e aspas duplas

**What**: `resolveInterpolation(s string, resolved map[string]string) string` novo em
`internal/dotenv/parse.go`, reconhecendo `$VAR`, `${VAR}`, `${VAR:-default}`, `${VAR-default}`,
`${VAR:+alt}`, `${VAR+alt}` dentro de `s` (hand-rolled scanner varrendo `s` char a char — NÃO uma
regex única pro texto inteiro, `design.md`'s Tech Decisions justifica isso pela necessidade de
lookup incremental por variável, não é sobre performance). Pra cada `VAR` encontrado: procura
primeiro em `resolved` (chaves já parseadas ANTES na MESMA carga — `parseFile`'s `[]envPair` em
ordem, T2 já preserva a ordem, esta task é quem PASSA o mapa incrementalmente conforme cada linha é
processada), senão em `os.Getenv(VAR)` (uma key de um `Load` anterior ou do processo real). Semântica dos 4 operadores (POSIX parameter expansion — espelhar exatamente):
`${VAR:-default}` → `default` se `VAR` unset OU vazio; `${VAR-default}` → `default` SÓ se `VAR`
unset (vazio-mas-setado NÃO conta); `${VAR:+alt}` → `alt` se `VAR` setado E não-vazio (nunca o
valor de `VAR`); `${VAR+alt}` → `alt` se `VAR` setado (mesmo vazio). `$VAR`/`${VAR}` sem operador e
sem valor encontrado em lugar nenhum → string vazia (não erro — `design.md`'s Edge Cases Resolved).
Ligar em `parseValue` (T2): SÓ os branches bare e aspas-duplas chamam `resolveInterpolation` no
conteúdo já extraído; aspas simples e backtick NÃO chamam (aspas simples é literal por design,
`spec.md` P1 AC3; backtick nem tem interpolação documentada). `parseFile` precisa passar o
`resolved map[string]string` incrementalmente conforme processa cada `envPair` em ordem (cada
key recém-parseada entra em `resolved` ANTES da próxima linha ser processada, senão AC4 do P1 do
spec — "`${VAR}` referencia uma variável já resolvida ANTES na mesma carga" — não teria como
funcionar).
**Where**: `internal/dotenv/parse.go`
**Depends on**: T2
**Reuses**: `envPair`, o dispatch de `parseValue` de T2
**Requirement**: DOTENV-03 (interpolação em si, o dispatch de aspas já foi T2), DOTENV-04 (4
operadores)

**Done when**:
- [ ] `A=hello` / `B=${A} world` (sem aspas) → `B` resolve pra `hello world`
- [ ] `A=hello` / `B="${A} world"` (aspas duplas) → `B` resolve pra `hello world`
- [ ] `${VAR:-default}` com `VAR` unset OU vazio → `default`
- [ ] `${VAR-default}` com `VAR` unset → `default`; com `VAR` setado vazio → NÃO usa o default
      (resolve pro valor vazio real)
- [ ] `${VAR:+alt}` com `VAR` setado e não-vazio → `alt` (não o valor de `VAR`)
- [ ] `${VAR+alt}` com `VAR` setado mesmo vazio → `alt`
- [ ] `${NUNCA_DEFINIDA}` (sem operador, sem valor em `resolved` nem `os.Getenv`) → string vazia,
      NÃO erro
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestResolveInterpolation_BareDollarVar_ExpandsFromResolvedMap`,
`TestResolveInterpolation_DoubleQuoted_Expands`,
`TestResolveInterpolation_ColonDashDefault_UnsetOrEmpty`,
`TestResolveInterpolation_DashDefault_OnlyUnsetNotEmpty`,
`TestResolveInterpolation_ColonPlusAlternate_SetAndNonEmpty`,
`TestResolveInterpolation_PlusAlternate_SetEvenEmpty`,
`TestResolveInterpolation_UndefinedNoOperator_ExpandsEmpty` (novos,
`internal/dotenv/parse_test.go`); `TestParseFile_LaterLineReferencesEarlierKey_ResolvesRealValue`
(prova a ordem incremental via `resolved`)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): interpolation (${VAR}/$VAR + 4 default/alternate operators)`

---

### T4: Escapes em aspas duplas (`\n`/`\r`/`\t`/`\\`) + aspas escapadas dentro do mesmo tipo

**What**: `applyEscapes(s string) string` novo em `internal/dotenv/parse.go`, convertendo as
sequências de 2 caracteres `\n`, `\r`, `\t`, `\\` dentro de `s` pro caractere real correspondente
(byte `0x0A`/`0x0D`/`0x09`/`\`). Ligar SÓ no branch aspas-duplas de `parseValue` (T2/T3) — ordem
confirmada em `design.md`'s Components: escapes resolvidos PRIMEIRO no conteúdo cru entre aspas,
DEPOIS `resolveInterpolation` no resultado já com escapes aplicados (evita um `\$` — não documentado
como escape do dotenvx, fora de escopo — ser lido errado como gatilho de interpolação antes de
qualquer coisa). Além disso: `parseValue`'s leitura do conteúdo entre aspas (T2, tanto simples
quanto duplas) precisa reconhecer `\'` dentro de aspas simples e `\"` dentro de aspas duplas como
uma aspa LITERAL (não fecha a aspas antecipadamente) — esta task corrige esse comportamento em T2's
scanner de extração (a extração ficou "boba" o bastante em T2 pra não escapar aspas do mesmo tipo;
esta task é quem faz o scanner reconhecer o escape ANTES de considerar aquele char um delimitador
de fechamento).
**Where**: `internal/dotenv/parse.go`
**Depends on**: T3
**Reuses**: o scanner de extração de conteúdo entre aspas já existente (T2), agora estendido pra
reconhecer `\'`/`\"` como literal antes de checar fechamento
**Requirement**: DOTENV-06

**Done when**:
- [ ] `VAR="linha1\nlinha2"` → valor final contém um `\n` REAL (byte `0x0A`), não os 2 caracteres
      `\`+`n`
- [ ] `\r`, `\t`, `\\` em aspas duplas também convertidos pro caractere real
- [ ] `VAR='ele disse \'oi\''` (aspas simples com `\'` escapada) → aspas simples internas
      preservadas como caractere literal `'`, não fecham o valor antecipadamente
- [ ] `VAR="ele disse \"oi\""` (mesma coisa com aspas duplas) → idem
- [ ] Escapes NÃO se aplicam a valores bare/backtick (só aspas duplas aplicam `\n` etc; bare já não
      suportava por design, `spec.md`'s P2 story)
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestApplyEscapes_NewlineTabCarriageReturnBackslash_ConvertsToRealChar`,
`TestParseValue_DoubleQuoted_EscapedQuoteIsLiteral`,
`TestParseValue_SingleQuoted_EscapedQuoteIsLiteral` (novos, `internal/dotenv/parse_test.go`)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): double-quote escape sequences (\n \r \t \\) + escaped same-type quotes`

---

### T5: Comentário inline (regra de espaço pra bare, regra de depois-da-aspa pra quoted)

**What**: `stripInlineComment` (ou equivalente inline no próprio `parseValue`'s branch bare, decisão
do Implementer) aplicando exatamente as 4 regras literais de `dotenv-loading/spec.md`'s "P1:
Comentários inline": (1) bare sem aspas, ` #` (espaço então hash) → tudo a partir do espaço é
removido: `VAR=VAL # comment` → `VAL`; (2) bare, `#` GRUDADO sem espaço antes → NÃO é comentário:
`VAR=VAL# not a comment` → `VAL# not a comment`; (3) valor com aspas (simples ou duplas), `#`
DENTRO das aspas → NÃO é comentário, faz parte do valor: `VAR="VAL # not a comment"` → `VAL # not a
comment`; (4) valor com aspas, `#` DEPOIS do fechamento das aspas → É comentário:
`VAR="VAL" # comment` → `VAL`. Regra (3) já é uma consequência NATURAL do scanner de extração de
T2/T4 (o `#` dentro das aspas nunca é visto pelo strip-de-comentário porque já foi consumido como
parte do conteúdo entre aspas antes dele existir) — esta task só precisa GARANTIR isso com teste
explícito, o código novo real é (1)/(2)/(4). Aplicar DEPOIS de escapes/interpolação já resolvidos
(comentário é sobre a sintaxe da LINHA crua, não sobre o valor já processado — decidir a ORDEM
exata olhando se o `#` de comentário pode estar dentro de uma interpolação, ex: `VAR=${A:-x#y} #
real comment`; tratar o strip de comentário como parte da EXTRAÇÃO do texto bruto de `parseValue`,
ANTES de `resolveInterpolation`/`applyEscapes` rodarem em cima do que sobrou — evita cortar um `#`
que faça parte de um valor de interpolação).
**Where**: `internal/dotenv/parse.go`
**Depends on**: T4
**Reuses**: o scanner de extração já existente (T2/T4)
**Requirement**: DOTENV-02

**Done when**:
- [ ] `VAR=VAL # comment` → `VAL` (espaço antes do `#`, bare)
- [ ] `VAR=VAL# not a comment` → `VAL# not a comment` (sem espaço antes do `#`, bare)
- [ ] `VAR="VAL # not a comment"` → `VAL # not a comment` (`#` dentro de aspas)
- [ ] `VAR="VAL" # comment` → `VAL` (`#` depois do fechamento das aspas)
- [ ] Os 4 casos acima testados NUM MESMO `.env` (as 4 linhas literais do `spec.md`), `Load`,
      confirmando os 4 valores via `os.Getenv` — mesmo Independent Test que `spec.md` já descreve
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestParseValue_BareSpaceHash_StripsComment`,
`TestParseValue_BareHashNoSpace_KeepsAsValue`, `TestParseValue_DoubleQuoted_HashInsideNotComment`,
`TestParseValue_DoubleQuoted_HashAfterClosingQuote_StripsComment` (novos,
`internal/dotenv/parse_test.go`); `TestLoad_FourInlineCommentLines_MatchesSpecExamples` (integration
leve, `internal/dotenv/dotenv_test.go`, usando `t.TempDir()` com um `.env` real contendo as 4 linhas
literais do `spec.md`)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): inline comment stripping (bare space-rule, quoted after-close-rule)`

---

### T6: Multiline via backtick

**What**: Estender o branch backtick de `parseValue`/`parseFile` (T2) pra multiline DE VERDADE: hoje
(T2) só lê até o backtick de fechamento na MESMA linha; esta task faz o scanner continuar lendo
LINHAS REAIS subsequentes do arquivo (já disponíveis por inteiro, já que `parseFile` lê o arquivo
inteiro de uma vez, não streaming linha-a-linha) até encontrar o backtick de fechamento não-escapado,
preservando quebras de linha REAIS (`\n` de verdade) entre as linhas consumidas no valor final. Como
isso consome MÚLTIPLAS linhas do arquivo de uma vez, `parseFile`'s loop de linha-a-linha (T2) precisa
avançar o índice/cursor de linha corretamente pra pular todas as linhas consumidas pelo bloco
backtick (não processar as linhas internas do backtick como se fossem `KEY=VALUE` novas). Sem
interpolação, sem escape (mesma decisão de T2/design.md — backtick é texto literal puro span de
linhas, distinto do processamento de aspas duplas).
**Where**: `internal/dotenv/parse.go`
**Depends on**: T5
**Reuses**: o branch backtick já existente (T2), o loop de linha de `parseFile` (T2, ajustado pra
avançar múltiplas linhas)
**Requirement**: DOTENV-05

**Done when**:
- [ ] Um valor backtick de 3 linhas reais no arquivo (`` VAR=`linha1\nlinha2\nlinha3` `` escrito
      como 3 linhas FÍSICAS reais no `.env`, não `\n` escapado) resolve pra uma string Go contendo 2
      quebras de linha reais (`\n` byte real) entre as 3 partes
- [ ] Linhas APÓS o backtick de fechamento continuam sendo processadas normalmente como
      `KEY=VALUE` novas (o cursor de `parseFile` não perde sincronia)
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestParseValue_BacktickMultiline_PreservesRealNewlines`,
`TestParseFile_LineAfterBacktickBlock_ParsedNormally` (novos, `internal/dotenv/parse_test.go`)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): backtick multiline values preserve real newlines`

---

### T7: Precedência entre paths + `os.Environ()` pré-existente (first-wins) + `Load`/`MustLoad` finais

**What**: Fechar `Dotenv.Load` (T1's esqueleto) com a política de precedência real: pra CADA path,
NA ORDEM recebida, `parseFile` já produz `[]envPair` com valores totalmente resolvidos (aspas,
interpolação, escapes, comentário — tudo já pronto desde T2..T6); `Load` então, pra CADA `envPair`
de CADA path em ordem, chama `os.Setenv(key, value)` SÓ SE `os.LookupEnv(key)` reportar ausente —
isso automaticamente implementa "first path wins" (um path processado ANTES já fez `Setenv`, então
o path seguinte vê a chave como presente e pula) E "processo pré-existente sempre vence" (uma env
var já setada por fora do processo ANTES de qualquer `Load` rodar também aparece como presente logo
de cara). Confirmar que um `Load(pathA, pathB)` onde AMBOS definem a MESMA chave com valores
diferentes resulta no valor de `pathA` (o primeiro). Nenhuma mudança na assinatura de `Load`/
`MustLoad` (já corretas desde T1) — esta task é sobre a ORDEM de aplicação do `Setenv`, que hoje (T1)
nem existe de verdade porque o `parseFile` stub retornava `nil, nil`.
**Where**: `internal/dotenv/dotenv.go`
**Depends on**: T6
**Reuses**: `parseFile` (agora completo desde T2..T6), `os.LookupEnv`/`os.Setenv`
**Requirement**: DOTENV-01 (fecha o requirement por completo — os ACs restantes já cobertos por
T1/T2), edge cases de precedência do `spec.md`/resolvidos em `design.md`

**Done when**:
- [ ] `Load("a.env", "b.env")` onde ambos definem `FOO` com valores diferentes → `os.Getenv("FOO")`
      após `Load` é o valor de `a.env` (primeiro path vence)
- [ ] `os.Setenv("FOO", "pre-existing")` ANTES de `Load("a.env")` (que também define `FOO`) → após
      `Load`, `os.Getenv("FOO")` continua `"pre-existing"` (processo pré-existente vence sobre
      arquivo)
- [ ] Uma chave presente SÓ em `b.env` (segundo path, não em `a.env` nem pré-existente) é setada
      normalmente
- [ ] `go test ./internal/dotenv/... -race` passa

**Tests**: unit — `TestLoad_MultiplePaths_FirstPathWinsSameKey`,
`TestLoad_PreExistingProcessEnv_WinsOverFile`, `TestLoad_KeyOnlyInSecondPath_IsSet` (novos,
`internal/dotenv/dotenv_test.go`, cada um limpando/restaurando `os.Environ()` relevante via
`t.Setenv`/`t.Cleanup` pra não vazar estado entre testes -- ATENÇÃO: `os.Setenv` é estado GLOBAL do
processo, estes testes NÃO SÃO paralelizáveis entre si nem com outros testes do pacote que também
mexem em env vars, usar `t.Setenv` em vez de `os.Setenv` cru sempre que possível pra restauração
automática)
**Gate**: quick (`go test ./internal/dotenv/... -race`)
**Commit**: `feat(dotenv): first-path-wins precedence, pre-existing process env always wins`

---

### T8: `gonest.Dotenv()` exportado em `gonest.go` (root re-export)

**What**: Adicionar em `gonest.go` (junto dos outros re-exports, ex: perto de `Parseable`/`Parse`/
`MustParse` já existentes em torno da linha ~837-880, ou numa seção nova "Dotenv" seguindo a mesma
formatação de seção que cada bloco de re-export já usa) um `var Dotenv = dotenv.Get` (thin
one-line wrapper, mesmo padrão de TODO outro re-export deste arquivo — `var NewModule =
module.New`, etc.) — chamável como `gonest.Dotenv().Load(...)`/`gonest.Dotenv().MustLoad(...)`.
Import novo: `"gonest.dev/gonest/internal/dotenv"`.
**Where**: `gonest.go`
**Depends on**: T7
**Reuses**: `dotenv.Get` (T1)
**Requirement**: Success Criteria (`spec.md`): "`gonest.Dotenv()` chamável de dentro de `main()`,
sem nenhum `Module`/`NewApp` já existir"

**Done when**:
- [ ] `gonest.Dotenv()` existe, retorna `*gonest.Dotenv` (alias de `*dotenv.Dotenv` se necessário —
      decisão do Implementer se um `type Dotenv = dotenv.Dotenv` é necessário pra assinatura ficar
      limpa, seguindo o padrão de outros tipos re-exportados como `type Module = module.Module`)
- [ ] Um teste NA RAIZ do módulo (`gonest_test.go`, pacote `gonest_test` ou `gonest`, o que já for a
      convenção do arquivo) chama `gonest.Dotenv().Load(...)` contra um `.env` real de teste SEM
      chamar `NewApp`/`MustNewApp`/`NewModule` antes — prova literal do critério de sucesso
- [ ] `go build ./...` passa
- [ ] `go test ./... -race` passa

**Tests**: unit/integration — `TestDotenv_CallableWithoutAnyAppBootstrap` (novo, `gonest_test.go`,
`t.TempDir()` com um `.env` real, `Load` chamado como a PRIMEIRA linha do teste, nenhum `NewApp`
antes)
**Gate**: full (`go test ./... -race`)
**Commit**: `feat(gonest): export Dotenv() -- root re-export, callable before any bootstrap`

---

### T9: `PropertyBuilder.Default`/`DefaultValue` em `internal/schema/schema.go` [P] (paralelo a T2..T8)

**What**: Adicionar `(*PropertyBuilder) Default(value any) *PropertyBuilder` e `(*PropertyBuilder)
DefaultValue() (any, bool)` em `internal/schema/schema.go`, bem ao lado de `Custom`/`CustomFunc`
(por volta da linha 620-645) — MESMO padrão exato: `Default` guarda `value` num campo não-exportado
novo (ex: `p.defaultValue`), seta um bool de presença novo (ex: `p.hasDefault = true`), retorna `p`
BARE (chainable, sem nenhum outro efeito colateral, last-call-wins se chamado 2x — mesma convenção
de `Custom`). `DefaultValue()` espelha `CustomFunc()`'s forma exata `(value, bool)`: retorna
`(nil, false)` se `Default` nunca foi chamado, `(value, true)` caso contrário. ZERO mudança em
nenhum outro método/campo existente de `PropertyBuilder` — esta task é aditiva pura, usável em
QUALQUER `PropertyBuilder` (a decisão de QUEM lê `DefaultValue()` — só `envSource`, nesta feature —
é responsabilidade de T10, não desta task).
**Where**: `internal/schema/schema.go`
**Depends on**: None (independe de qualquer coisa da Track A — só precisa que `internal/schema`
já exista, o que já é verdade hoje)
**Reuses**: o padrão exato campo/método de `Custom`/`CustomFunc` (mesmo arquivo, cópia adaptada, não
um padrão novo inventado)
**Requirement**: ENVCFG-03 (a parte de `PropertyBuilder.Default` em si — o USO dele por `envSource`
é T10)

**Done when**:
- [ ] `p.Default(5432)` seguido de `p.DefaultValue()` retorna `(5432, true)`
- [ ] Um `PropertyBuilder` onde `Default` NUNCA foi chamado retorna `(nil, false)` de `DefaultValue()`
- [ ] Chamar `Default` 2 vezes no mesmo `PropertyBuilder` → última chamada vence (mesma convenção de
      `Custom`)
- [ ] `Default` retorna `p` (chainable — `builder.Required().Default(x)` compila e funciona)
- [ ] Nenhum teste existente de `internal/schema` quebra
- [ ] `go test ./internal/schema/... -race` passa

**Tests**: unit — `TestPropertyBuilder_Default_SetsDefaultValue`,
`TestPropertyBuilder_DefaultValue_NeverCalled_ReturnsFalse`,
`TestPropertyBuilder_Default_LastCallWins`, `TestPropertyBuilder_Default_ReturnsSelfForChaining`
(novos, `internal/schema/schema_test.go` ou arquivo de teste já existente equivalente — seguir a
convenção de nomenclatura de arquivo já usada pros testes de `Custom`)
**Gate**: quick (`go test ./internal/schema/... -race`)
**Commit**: `feat(schema): PropertyBuilder.Default/DefaultValue -- fallback for absent source data`

---

### T10: `envSource`/`ParseEnvInto` em `internal/validate/env.go`

**What**: Novo arquivo `internal/validate/env.go`, mesmo pacote de `params.go`/`query.go`/
`headers.go`/`form.go`/`validate.go`. `func ParseEnvInto(dst any, schemaArg any) error` — função
LIVRE (não uma struct com `req *execution.Request` como `paramsSource` — não existe request num
contexto de config-loading; `design.md`'s Tech Decisions justifica isso explicitamente). Espelha
`paramsSource.ParseInto` (`internal/validate/params.go:63-120`, LIDO nesta sessão de planejamento —
usar como referência EXATA de forma, não reinventar a estrutura): `resolveSchema(m, dstVal.Type())`
primeiro; pra cada `m.OwnProperties()`: `key, visible := tagKeyVisible(p.Field(), "env")` (mesmo
helper, tag NOVA `"env"` em vez de `"param"`), `continue` se `!visible`; presença via
`os.LookupEnv(key)` (NÃO `os.Getenv` sozinho — precisa distinguir unset de vazio-mas-setado,
`design.md`'s Edge Cases Resolved table); se AUSENTE (`ok=false` do `LookupEnv`) E `p.DefaultValue()`
(T9) reporta presença: `presence[key] = defaultValue` DIRETO, SEM passar por `coerceParamString`/
`validateValue` (o valor de `Default` já É o tipo Go certo, não uma string crua precisando de
coerção — `design.md`'s Tech Decisions é explícito sobre isso); se AUSENTE e SEM `Default`: mesmo
branch de `paramsSource` — `p.IsRequired()` vira uma `violation{Field: key, Message: "required"}`
coletada (collect-all, NUNCA para na primeira), senão `continue` (campo fica zero-value); se
PRESENTE (`ok=true`, mesmo se a string for vazia): mesmo branch de `paramsSource` —
`CustomFunc()` presente → `validateValue(raw, p, key)` direto com a string crua; senão
`coerceParamString(raw, p.KindValue())` (REUSA sem NENHUMA mudança) seguido de `validateValue`.
Termina com `if len(violations) > 0 { return exception.NewBadRequestException(violations) }` e
`populate(dstVal, presence, m, "env")` (REUSA sem NENHUMA mudança, só a tag string difere de
`"param"`/`"json"`). Import novo: `"os"`. Este arquivo é testável SEM depender do parser de `.env`
completo (Track A) — os testes usam `os.Setenv`/`t.Setenv` diretamente, provando `envSource`
isoladamente do carregamento de arquivo (decisão do Planner: mais barato e mais isolado que exigir
um `.env` real só pra testar coerção/Required/Default).
**Where**: `internal/validate/env.go` (novo)
**Depends on**: T9 (precisa de `PropertyBuilder.DefaultValue()` já existindo pra compilar)
**Reuses**: `resolveSchema`, `tagKeyVisible`, `coerceParamString`, `validateValue`, `populate`
(TODOS de `internal/validate/{validate,params}.go`, ZERO mudança em qualquer um deles)
**Requirement**: ENVCFG-01, ENVCFG-02, ENVCFG-03 (a parte de `envSource` usar `Default`)

**Done when**:
- [ ] Struct com 4 campos `env:"..."`, todas as 4 env vars setadas (via `t.Setenv`) com valores
      válidos, `ParseEnvInto` popula uma instância idêntica aos valores esperados (mesmo Independent
      Test do `spec.md`'s P1)
- [ ] Campo `Integer()`/`Boolean()` lê a string crua da env var e coerciona corretamente
      (reusando `coerceParamString`), mesmo erro de validação que `param`/`query` produzem se a
      coerção falhar
- [ ] 2 campos `.Required()` sem `Default`, nenhuma env var setada → erro contendo EXATAMENTE 2
      violações (collect-all, mesmo Independent Test do `spec.md`'s 2ª story)
- [ ] Campo com `.Default("127.0.0.1")`, env var ausente → resolve pra `"127.0.0.1"`; MESMO campo
      com a env var setada pra outro valor → resolve pro valor real da env var, NÃO o default
      (mesmo Independent Test do `spec.md`'s 3ª story)
- [ ] Env var setada mas VAZIA (`t.Setenv("DB_HOST", "")`) é tratada como PRESENTE (não aciona
      `Default`, vai pro caminho normal de coerção/validação) — `design.md`'s Edge Case resolvido
- [ ] Campo sem tag `env:"..."` no mesmo `Schema` é simplesmente ignorado (sem erro de build,
      fica no zero-value) — `design.md`'s 2º Edge Case resolvido
- [ ] `go test ./internal/validate/... -race` passa, ZERO mudança de comportamento em
      `params_test.go`/`query_test.go`/`headers_test.go`/`form_test.go`/`validate_test.go`
      existentes

**Tests**: unit — `TestParseEnvInto_AllFieldsSet_PopulatesCorrectly`,
`TestParseEnvInto_IntegerCoercionFails_RecordsViolation`,
`TestParseEnvInto_TwoRequiredMissing_CollectsBothViolations`,
`TestParseEnvInto_DefaultUsedWhenAbsent_RealValueUsedWhenPresent`,
`TestParseEnvInto_EmptyButSetEnvVar_TreatedAsPresent`,
`TestParseEnvInto_FieldWithoutEnvTag_Ignored` (novos, `internal/validate/env_test.go`, usando
`t.Setenv` — nunca `os.Setenv` cru — pra restauração automática entre casos)
**Gate**: full (`go test ./... -race` -- confirma zero regressão nas outras fontes)
**Commit**: `feat(validate): envSource/ParseEnvInto -- reuses coerceParamString/validateValue/populate unchanged`

---

### T11: `Dotenv.ParseInto` + `gonest.MustParse[T](gonest.Dotenv(), schema)` ponta a ponta

**What**: Adicionar `(*Dotenv) ParseInto(dst any, schema any) error { return
validate.ParseEnvInto(dst, schema) }` em `internal/dotenv/dotenv.go` (mesmo arquivo de
`Load`/`MustLoad` — o arquivo não está grande o bastante pra justificar split, mas se o Implementer
achar que está, um `internal/dotenv/parseinto.go` novo é aceitável, decisão dele) — delegação de
UMA LINHA, satisfaz `execution.Parseable` (`internal/execution/request.go:43-45`, interface de 1
método `ParseInto(dst any, schema any) error`) no MESMO tipo `*Dotenv` que já tem `Load`/`MustLoad`
(Track A) — confirma `dotenv-loading/context.md`'s D2 ("UMA instância, não dois tipos"). Import
novo em `internal/dotenv`: `"gonest.dev/gonest/internal/validate"` — confirmar ACÍCLICO
(`internal/validate` não importa `internal/dotenv` hoje, e não ganha nenhuma referência de volta
nesta task). Teste ponta a ponta OBRIGATÓRIO: um `.env` REAL (`t.TempDir()`) carregado via
`gonest.Dotenv().MustLoad(...)`, seguido de `gonest.MustParse[DatabaseConfig](gonest.Dotenv(),
schema)` retornando a struct populada — prova que Track A (parser real) + Track B (bind real)
funcionam JUNTAS, não só cada uma isolada (T7/T10 já provaram cada metade sozinha via
`os.Setenv`/arquivo cru; esta é a PRIMEIRA task que precisa das duas rodando em conjunto).
**Where**: `internal/dotenv/dotenv.go` (ou `internal/dotenv/parseinto.go` novo)
**Depends on**: T10, T8 (precisa de `gonest.Dotenv()` já exportado pro teste ponta a ponta chamar
via `gonest.` em vez de `dotenv.`)
**Reuses**: `validate.ParseEnvInto` (T10) — 100% da lógica, esta task só satisfaz o NOME do método
que `Parseable` exige
**Requirement**: ENVCFG-01 (fecha o requirement — "`*Dotenv` satisfaz `execution.Parseable`")

**Done when**:
- [ ] `*dotenv.Dotenv` satisfaz `execution.Parseable` (compila como tal, ex: `var _
      execution.Parseable = (*dotenv.Dotenv)(nil)` como guarda de compilação)
- [ ] `gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)` funciona igual a qualquer outro
      `Parse[T]`/`MustParse[T]` já existente (mesmo contrato: `Parse[T]` retorna `(T, error)`,
      `MustParse[T]` panica)
- [ ] Teste ponta a ponta: `.env` real via `t.TempDir()` → `MustLoad` → `MustParse[T]` → struct
      populada com os valores REAIS do arquivo (não só `os.Setenv` direto)
- [ ] `internal/dotenv` importando `internal/validate` não introduz ciclo (`go build ./...` prova
      isso sozinho)
- [ ] `go test ./... -race` passa

**Tests**: unit/integration — `TestDotenv_SatisfiesParseable` (compile-time guard),
`TestDotenv_ParseInto_DelegatesToParseEnvInto` (novo, `internal/dotenv/dotenv_test.go`);
`TestMustParse_DotenvEndToEnd_LoadThenBind` (novo, `gonest_test.go`, `.env` real via `t.TempDir()`
→ `gonest.Dotenv().MustLoad` → `gonest.MustParse[T]`)
**Gate**: full (`go test ./... -race`)
**Commit**: `feat(dotenv): ParseInto satisfies execution.Parseable -- Load+MustParse work end-to-end`

---

### T12: Gate final — suite completa, exemplo mínimo, STATE.md/ROADMAP.md

**What**: Rodar a suite completa, confirmar zero regressão. Criar um exemplo mínimo novo em
`.examples/` (ex: `.examples/config-dotenv/`) demonstrando o fluxo ponta a ponta real: `main()`
chamando `gonest.Dotenv().MustLoad("./.env")` como a PRIMEIRA linha (ANTES de qualquer `NewApp`),
seguido de um `Provider.Constructor` chamando `gonest.MustParse[DatabaseConfig](gonest.Dotenv(),
databaseConfigSchema)` — prova o caso de uso central das duas features juntas (`ConfigModule`-like
do NestJS, motivação original do `spec.md`). Rodar o exemplo de VERDADE (`go run`/`go build` dentro
do módulo do exemplo), não só compilar — mesma lição já registrada em `STATE.md` sobre AD-036/AD-037
(bugs reais só aparecem rodando de verdade). Atualizar `.specs/project/STATE.md` com um AD novo
documentando a execução desta feature (SPEC_DEVIATIONs, se houver, mesmo padrão de toda feature
anterior) e `.specs/project/ROADMAP.md`'s Milestone 19 → COMPLETE. Atualizar as tabelas de
Requirement Traceability de AMBOS `spec.md` (`dotenv-loading` E `env-schema-binding`) de "Pending"
pra "Verified".
**Where**: raiz, `.examples/config-dotenv/*` (novo), `.specs/project/{STATE,ROADMAP}.md`,
`.specs/features/dotenv-loading/spec.md`, `.specs/features/env-schema-binding/spec.md`
**Depends on**: T11
**Requirement**: Success Criteria de ambos os `spec.md`

**Done when**:
- [ ] `go test ./... -race` passa
- [ ] `go build ./...` passa (repo raiz + TODO `.examples/*`, não só o novo)
- [ ] `.examples/config-dotenv` roda de verdade (`go run`), demonstrando `MustLoad` +
      `MustParse[T]` ponta a ponta com um `.env` real de exemplo
- [ ] `STATE.md` tem novo AD documentando a execução (+ SPEC_DEVIATIONs, se houver)
- [ ] `ROADMAP.md`'s Milestone 19 → COMPLETE
- [ ] `dotenv-loading/spec.md`'s traceability table: todo DOTENV-0x → Verified
- [ ] `env-schema-binding/spec.md`'s traceability table: todo ENVCFG-0x → Verified

**Tests**: integration (suite completa) + manual/scripted (`.examples/config-dotenv` rodado de
verdade, evidência colada no relatório do Implementer, não assumida)
**Gate**: full (`go test ./... -race`) + manual (`go run .examples/config-dotenv`)
**Commit**: `chore: finalize Config Loading (Milestone 19) -- dotenv-loading + env-schema-binding, update STATE`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Dotenv singleton + Load/MustLoad skeleton | 1 pacote novo, esqueleto | ✅ |
| T2: parseFile classification + quote dispatch | 1 arquivo, escopo fechado (sem interpolação/escape) | ✅ |
| T3: interpolation + 4 operators | extensão do mesmo arquivo, 1 capability | ✅ |
| T4: double-quote escapes + escaped quotes | extensão, 1 capability | ✅ |
| T5: inline comment stripping | extensão, 1 capability (4 regras literais do spec) | ✅ |
| T6: backtick multiline | extensão, 1 capability | ✅ |
| T7: precedence (first-wins) | extensão, fecha Load/MustLoad de verdade | ✅ |
| T8: gonest.Dotenv() root re-export | 1 var + 1 teste | ✅ |
| T9: PropertyBuilder.Default/DefaultValue | 2 métodos, cópia de padrão existente | ✅ |
| T10: envSource/ParseEnvInto | 1 arquivo novo, reuso pesado (zero função nova reescrita) | ✅ |
| T11: Dotenv.ParseInto + e2e | 1 método de 1 linha + 1 teste ponta a ponta | ✅ |
| T12: gate final | verificação + exemplo + docs | ✅ |
