# Dotenv Loading Context

**Gathered:** 2026-07-18/19 (brainstorming em conversa, evoluindo `INSIGHT-CONFIG.md`)
**Spec:** `.specs/features/dotenv-loading/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Trazer carregamento de `.env` pro gonest, motivado pelo `ConfigModule` do NestJS e por um modelo
próprio do usuário (`C:\dev\leandroluk\gox\env`) que já existia ANTES desta sessão. O usuário deixou
claro que `gox/env` foi construído **mirando o comportamento do [`dotenvx`](https://dotenvx.com)**
(interpolação, comentários, etc.) -- ou seja, a referência de comportamento real não é `gox/env` em si
(que é implementação própria do usuário), mas o formato `.env` que o `dotenvx` define e documenta
publicamente. Esta é a referência a citar quando a feature for documentada no site (`gonest.dev`).

## Implementation Decisions

### `gonest.Dotenv()` é UM tipo, não dois (`Dotenv` + `Env`)

Rascunho original de `INSIGHT-CONFIG.md` tinha `gonest.Env()` como uma fonte de leitura separada de
`gonest.Dotenv()` (o loader). Usuário notou que isso cria dois conceitos pra descrever o MESMO espaço
de variáveis (`os.Environ()`) -- corrigido pra `*Dotenv` acumular as duas capacidades: `Load`/`MustLoad`
(side-effect, ESTA feature) e `ParseInto` satisfazendo `execution.Parseable` (feature
`env-schema-binding`, depende desta, spec separado). `gonest.Env()` foi removido do design.

### `gonest.Dotenv()` não passa por DI

Não é resolvível via `MustInject` -- precisa funcionar em `main()`, ANTES de qualquer `Module`/
`Provider`/`NewApp` bootstrap existir (o exemplo original do usuário já mostrava isso: `.env` carrega
como a PRIMEIRA linha de `main()`, antes de `NewApp`).

### Sintaxe: paridade completa com `dotenvx`, pesquisa real feita

`https://dotenvx.com/docs/env-file` (site oficial do `dotenvx`) foi lido via `WebFetch` nesta sessão --
não assumido. Regras confirmadas (citadas literalmente no `spec.md`):
- Comentários de linha inteira (`#` no início) e inline (regra de espaço antes do `#` pra valores sem
  aspas; `#` dentro/depois de aspas se comporta diferente)
- Interpolação (`${VAR}`/`$VAR`) aplicada em valores sem aspas e com aspas DUPLAS; aspas SIMPLES usam
  o valor literal, sem expandir
- 4 operadores de default/alternate: `${VAR:-default}`, `${VAR-default}`, `${VAR:+alternate}`,
  `${VAR+alternate}` -- mesma semântica de expansão de parâmetro do shell POSIX
- Multiline via backtick (`` VAR=`linha1\nlinha2` ``)
- Escapes (`\n`/`\r`/`\t`/`\\`) suportados em valores entre aspas duplas

Decisão do usuário (via `AskUserQuestion`): **paridade completa** já na v1, não só um núcleo mínimo --
os 4 operadores de default/alternate e o multiline via backtick entram no escopo (viraram P2 no
spec.md, não Out of Scope).

### O que NÃO faz parte desta feature

`dotenvx` (a ferramenta/CLI real) também tem um modo de `.env` CRIPTOGRAFADO (`.env.vault`,
`DOTENV_PRIVATE_KEY`) -- fora de escopo aqui, propositalmente. Esta feature implementa só o FORMATO de
texto `.env` que o `dotenvx` documenta, não o produto `dotenvx` inteiro (criptografia, CLI própria,
etc.).

## Specific References

- `https://dotenvx.com/docs/env-file` -- fonte da verdade da sintaxe, citar no site (`gonest.dev`)
  quando esta feature for documentada, mesmo padrão de outras features que citam a lib/protocolo real
  que seguem (ex: `graphql-realtime-protocols` cita `github.com/enisdenjo/graphql-ws`/`graphql-sse`)
- `https://github.com/dotenvx/dotenvx` -- repositório do `dotenvx`
- `C:\dev\leandroluk\gox\env` -- modelo próprio do usuário que motivou a feature (fora deste repo, só
  referência de inspiração de API, não fonte de comportamento -- a fonte de comportamento é o `dotenvx`
  em si)
- `INSIGHT-CONFIG.md` (raiz do repo) -- rascunho vivo que evoluiu até este spec, mantido como registro
  histórico do brainstorming

## Deferred Ideas

- Criptografia/`.env.vault` do `dotenvx` -- fora de escopo, ver spec.md
- Interpolação de comando de shell (`$(cmd)`) -- não faz parte do formato `.env` documentado do
  `dotenvx`, não incluído
- Hot-reload de `.env` em runtime -- nenhum caso de uso real levantado
- Política de precedência entre múltiplos `Load(paths...)` e entre arquivo vs. `os.Environ()`
  pré-existente -- deixado em aberto pra Design (spec.md's Edge Cases)
