# Dotenv Loading Specification

## Problem Statement

gonest não tem hoje nenhuma forma de carregar variáveis de ambiente a partir de um arquivo `.env` --
todo config real depende de `os.Environ()` já estar populado por fora (shell, systemd, Docker, CI).
Motivado pelo `ConfigModule` do NestJS e por um modelo próprio do usuário já existente
(`C:\dev\leandroluk\gox\env`), que por sua vez foi construído mirando o comportamento REAL do
[`dotenvx`](https://dotenvx.com) (`dotenvx/dotenvx` no GitHub) -- não um parser `.env` inventado do
zero. Esta feature traz esse mesmo comportamento pro `gonest`, como primeiro passo de uma feature
maior (Config Loading) que depois soma `env-schema-binding` (validação/binding tipado contra `Schema`,
spec separado).

## Goals

- [ ] `gonest.Dotenv()` retorna um singleton (`*Dotenv`) acessível de qualquer lugar, inclusive ANTES
      de qualquer `NewApp`/bootstrap existir (chamável no topo de `main()`)
- [ ] `Load(paths ...string) error` / `MustLoad(paths ...string)` carregam `.env` pro processo real
      (`os.Setenv`), com paridade de sintaxe COMPLETA com o `.env` do `dotenvx` (comentários,
      interpolação, aspas, escapes, multiline via backtick, operadores de default/alternate)

## Out of Scope

| Feature | Reason |
| ------- | ------ |
| Criptografia/`.env.vault` do `dotenvx` propriamente dito | `dotenvx` (a ferramenta/CLI) tem um modo de arquivo `.env` criptografado com chave pública -- fora de escopo, v1 é só arquivo texto local, sem segredo cifrado |
| Interpolação de comando (`$(cmd)`) | Não documentado como parte do formato `.env` do `dotenvx` (é sintaxe de shell, não do parser `.env`) -- não incluído |
| `env-schema-binding` (validar/popular struct tipada a partir da env) | Feature SEPARADA (`.specs/features/env-schema-binding/`), depende desta mas tem seu próprio spec/design/tasks |
| Hot-reload de `.env` (observar mudança de arquivo em runtime) | Não mencionado em nenhum caso de uso real levantado -- `.env` carrega uma vez, no boot |

## Design Decisions (herdadas do brainstorming em `INSIGHT-CONFIG.md`)

| # | Decisão |
| - | ------- |
| D1 | `gonest.Dotenv()` é singleton SEM DI -- não resolvível via `MustInject`, porque precisa funcionar em `main()` antes de qualquer `Module`/`Provider`/bootstrap existir |
| D2 | `*Dotenv` acumula 2 capacidades ao longo de 2 features: `Load`/`MustLoad` (esta feature) e `ParseInto` satisfazendo `execution.Parseable` (feature `env-schema-binding`, futura) -- UMA instância, não dois tipos |
| D3 | Sintaxe do `.env` segue o `dotenvx` REAL (`https://dotenvx.com/docs/env-file`), pesquisado nesta sessão -- não um formato inventado |
| D4 | Paridade completa com `dotenvx` já na v1 (decisão do usuário, via `AskUserQuestion`) -- inclui os 4 operadores de default/alternate e multiline via backtick, não só o núcleo (comentários+interpolação básica) |

## User Stories

### P1: Carregar `.env` simples ⭐ MVP

**User Story**: Como dev usando gonest, quero carregar um arquivo `.env` no início do `main()` pra
popular `os.Environ()` antes de qualquer Provider rodar.

**Why P1**: Sem isso, a feature `env-schema-binding` não tem nada pra ler -- é o pré-requisito de toda
a Config Loading.

**Acceptance Criteria**:

1. WHEN `gonest.Dotenv().Load("./.env")` roda e o arquivo existe THEN cada `CHAVE=valor` do arquivo
   SHALL virar uma variável real via `os.Setenv`
2. WHEN uma linha começa com `#` THEN SHALL ser ignorada por inteiro (comentário de linha)
3. WHEN o arquivo NÃO existe THEN `Load` SHALL retornar um `error` (não panicar) -- caller decide se
   ignora ou propaga
4. WHEN `gonest.Dotenv().MustLoad("./.env")` roda e o arquivo não existe THEN SHALL panicar

**Independent Test**: um `.env` com 3 linhas simples (`FOO=bar`, linha em branco, comentário) carregado
via `Load`, confirmar `os.Getenv("FOO") == "bar"` depois.

---

### P1: Comentários inline ⭐ MVP

**User Story**: Como dev, quero comentar o PORQUÊ de um valor na mesma linha da variável.

**Why P1**: Comportamento básico e frequente de `.env` real (todo dev já viu isso em outro projeto).

**Acceptance Criteria**:

1. WHEN o valor NÃO tem aspas e a linha tem ` # comentário` (espaço antes do `#`) THEN o comentário
   SHALL ser removido do valor -- `VAR=VAL # comment` → `VAL`
2. WHEN o valor NÃO tem aspas e o `#` vem GRUDADO no valor (sem espaço antes) THEN NÃO SHALL ser
   tratado como comentário -- `VAR=VAL# not a comment` → `VAL# not a comment`
3. WHEN o valor TEM aspas (simples ou duplas) e um `#` aparece DENTRO das aspas THEN NÃO SHALL ser
   removido -- `VAR="VAL # not a comment"` → `VAL # not a comment`
4. WHEN o valor tem aspas e o `#` vem DEPOIS do fechamento das aspas THEN SHALL ser tratado como
   comentário -- `VAR="VAL" # comment` → `VAL`

**Independent Test**: as 4 linhas acima num `.env`, `Load`, confirmar os 4 valores exatos via
`os.Getenv`.

---

### P1: Aspas simples vs duplas (interpolação) ⭐ MVP

**User Story**: Como dev, quero controlar se `${OUTRA_VAR}` dentro de um valor é expandido ou tratado
como texto literal.

**Why P1**: Diferença central entre aspas simples/duplas no `dotenvx` real -- sem isso a paridade não
existe de verdade.

**Acceptance Criteria**:

1. WHEN um valor NÃO tem aspas THEN interpolação (`${VAR}`/`$VAR`) SHALL ser aplicada
2. WHEN um valor tem aspas DUPLAS (`"..."`) THEN interpolação SHALL ser aplicada dentro delas
3. WHEN um valor tem aspas SIMPLES (`'...'`) THEN o conteúdo SHALL ser usado LITERALMENTE -- `$VAR`
   dentro de aspas simples SHALL permanecer como o texto `$VAR`, não expandido
4. WHEN `${VAR}` referencia uma variável já resolvida ANTES na mesma carga (arquivo ou `os.Environ()`
   pré-existente) THEN SHALL expandir pro valor real dessa variável

**Independent Test**: `A=hello`, `B=${A} world` (dupla/sem aspas, espera `hello world`),
`C='${A} world'` (simples, espera literal `${A} world`) -- carregar e conferir os 3 valores.

---

### P2: Operadores de default/alternate na interpolação

**User Story**: Como dev, quero um valor de fallback direto na expressão de interpolação, sem precisar
de lógica externa.

**Why P2**: Não é MVP (P1 sem isso já entrega valor real), mas faz parte da paridade completa decidida
com o usuário.

**Acceptance Criteria**:

1. WHEN `${VAR:-default}` E `VAR` está UNSET ou vazio THEN SHALL expandir pra `default`
2. WHEN `${VAR-default}` E `VAR` está UNSET (mas presente com string vazia NÃO conta) THEN SHALL
   expandir pra `default`
3. WHEN `${VAR:+alternate}` E `VAR` está SET e NÃO vazio THEN SHALL expandir pra `alternate` (não pro
   valor de `VAR`)
4. WHEN `${VAR+alternate}` E `VAR` está SET (mesmo vazio) THEN SHALL expandir pra `alternate`

**Independent Test**: 4 variáveis cobrindo cada operador com `VAR` unset/vazio/set, conferir os 4
resultados batem com a tabela acima.

---

### P2: Multiline via backtick

**User Story**: Como dev, quero um valor de várias linhas (ex: chave privada, JSON formatado) sem
escapar `\n` manualmente.

**Why P2**: Caso de uso real do `dotenvx` (mencionado na doc oficial), não MVP mas parte da paridade
completa.

**Acceptance Criteria**:

1. WHEN um valor é delimitado por backtick (`` VAR=`linha1\nlinha2` ``, texto literal entre backticks
   ocupando múltiplas linhas reais do arquivo) THEN SHALL preservar as quebras de linha reais no valor
   resultante

**Independent Test**: um `.env` com um valor multiline via backtick de 3 linhas, `Load`, conferir
`os.Getenv` retorna as 3 linhas com `\n` real entre elas.

---

### P2: Escapes em aspas duplas

**User Story**: Como dev, quero usar `\n`/`\t`/etc dentro de um valor entre aspas duplas sem precisar de
backtick.

**Why P2**: Complementa P1 (aspas duplas), parte da paridade completa.

**Acceptance Criteria**:

1. WHEN um valor entre aspas duplas contém `\n`, `\r`, `\t`, ou `\\` THEN SHALL ser interpretado como o
   caractere de escape real correspondente (não o texto literal de 2 caracteres)
2. WHEN uma aspa (simples ou dupla) é escapada com `\` dentro do mesmo tipo de aspas que a envolve
   THEN SHALL ser tratada como caractere literal, não fechamento antecipado

**Independent Test**: `VAR="linha1\nlinha2"` carregado, conferir o valor contém um `\n` real (byte
0x0A), não os 2 caracteres `\`+`n`.

---

## Edge Cases

- WHEN 2 `Load` paths são passados e o PRIMEIRO existe THEN o comportamento de múltiplos paths (usar só
  o primeiro que existir vs. carregar/mesclar todos em ordem) fica em aberto pra Design -- não decidido
  neste Specify
- WHEN uma variável já existe em `os.Environ()` (setada por fora, ex: shell) e o `.env` também define
  a MESMA chave THEN a política de precedência (arquivo sobrescreve processo, ou processo já setado
  vence) fica em aberto pra Design
- WHEN `${VAR}` referencia uma variável que NUNCA foi definida (nem no arquivo, nem em `os.Environ()`,
  nem tem operador de default) THEN SHALL expandir pra string vazia (comportamento padrão de shell/
  `dotenvx`, não erro)
- WHEN o arquivo `.env` tem uma linha malformada (ex: sem `=`) THEN o comportamento (ignorar a linha,
  ou `Load` retornar erro) fica em aberto pra Design

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --------------- | -------------------------------------------------- | ------- | ------- |
| DOTENV-01 | P1: Carregar `.env` simples | Specify | Pending |
| DOTENV-02 | P1: Comentários inline | Specify | Pending |
| DOTENV-03 | P1: Aspas simples vs duplas (interpolação) | Specify | Pending |
| DOTENV-04 | P2: Operadores de default/alternate | Specify | Pending |
| DOTENV-05 | P2: Multiline via backtick | Specify | Pending |
| DOTENV-06 | P2: Escapes em aspas duplas | Specify | Pending |

**Coverage:** 6 total, 0 mapped to tasks, 6 unmapped ⚠️ (normal em Specify -- mapeamento acontece em Tasks)

## Success Criteria

- [ ] `gonest.Dotenv().Load`/`MustLoad` funcionam contra um `.env` real cobrindo TODOS os P1+P2 acima
- [ ] Paridade de comportamento confirmada contra `https://dotenvx.com/docs/env-file` (não assumida --
      testes espelham os exemplos literais da doc oficial onde possível)
- [ ] `go test ./... -race` passa após a implementação
- [ ] `gonest.Dotenv()` chamável de dentro de `main()`, sem nenhum `Module`/`NewApp` já existir
