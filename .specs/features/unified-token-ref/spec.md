# Unified Token (TokenRef) — Spec

## Context

Achado real no consumer `erc`: `system/module.go`, `auth/module.go` e `infra/database/module.go`
todos declaram `var providers = []gonest.ProviderRef{...}`, passam pra `m.Providers(providers...)`
e depois tentam reusar a MESMA slice em `m.Exports(any(providers).([]gonest.ExportableRef)...)`
pra evitar redigitar a lista. Isso PANICA em runtime -- `interface conversion: interface {} is
[]module.ProviderRef, not []module.ExportableRef` -- porque Go não é covariante em slice de
interface: mesmo `ProviderRef` satisfazendo `ExportableRef` (embed desde AD-052), `[]ProviderRef`
nunca vira `[]ExportableRef` por type-assert nem por spread `...`. Não tem workaround sem
converter elemento a elemento.

Avaliado 2 saídas: (a) helper de conversão aditivo (`gonest.AsExportable(...)`), (b) resolver na
raiz -- um marker `TokenRef` único que todo builder do `Module` aceita, pra o dev declarar UMA
slice tipada certa desde o início e reusar em qualquer método sem conversão nenhuma. Usuário
rejeitou (a) explicitamente: gonest existe pra ser opção de migração de quem vem de NestJS, que já
enfrenta atrito extra escrevendo Go; "helper de conversão" é gambiarra em cima do sintoma, não
resolve a causa (assinaturas incompatíveis entre os métodos do builder). (b) escolhido, e expandido
pro escopo máximo: TODOS os métodos builder de `Module` (`Imports`, `Providers`, `Controllers`,
`Resolvers`, `Use`, `Filters`, `Listeners`, `Schedulers`, `Exports`) passam a aceitar `...TokenRef`,
roteando por type-switch internamente -- paridade total com o conceito de "provider token" do
NestJS, onde providers/imports/exports referenciam os mesmos tokens de forma intercambiável.

## User Story

Como usuário do gonest vindo de NestJS, quero declarar uma única slice tipada com os providers do
meu módulo e reusá-la tanto em `m.Providers(...)` quanto em `m.Exports(...)` sem nenhuma conversão
manual -- igual eu reusaria o mesmo array de classes em `providers: [...]` e `exports: [...]` no
Nest.

## Requirements

1. Novo marker interface `TokenRef interface { IsToken() }` (`internal/module/module.go`), base de
   TODOS os markers existentes (`ProviderRef`, `ControllerRef`, `ResolverRef`, `MiddlewareRef`,
   `FilterRef`, `ListenerRef`, `SchedulerRef` passam a embutir `TokenRef`) e implementado
   diretamente por `*Module`.
2. `ExportableRef` (AD-052) é REMOVIDO -- `TokenRef` assume seu papel (era estritamente mais
   restrito: só cobria Provider-or-Module; `TokenRef` cobre qualquer coisa registrável).
3. Todo método builder de `Module` muda de assinatura tipada (`...ProviderRef`,
   `...ControllerRef`, etc, incluindo `Imports(...*Module)`) pra `...TokenRef`, com type-switch
   interno roteando pro storage certo:
   - `Providers`: `case ProviderRef` → `m.providers`
   - `Controllers`: `case ControllerRef` → `m.controllers`
   - `Resolvers`: `case ResolverRef` → `m.resolvers`
   - `Use`: `case MiddlewareRef` → `m.middleware`
   - `Filters`: `case FilterRef` → `m.filters`
   - `Listeners`: `case ListenerRef` → `m.listeners`
   - `Schedulers`: `case SchedulerRef` → `m.schedulers`
   - `Imports`: `case *Module` → `m.imports`
   - `Exports`: `case *Module` → `m.exportedModules`; `case ProviderRef` → `m.exports`
4. WHEN um `TokenRef` passado pra um método builder NÃO bate no(s) case(s) esperado(s) daquele
   método (ex: passar um `ControllerRef` pra `Providers`, ou um `ProviderRef` pra `Imports`) THEN
   o método panica imediatamente com mensagem clara nomeando o método e o tipo recebido --
   fail-fast, mesma postura de `MustInject`. Comportamento antigo do `Exports` (ignorar
   silenciosamente tipo desconhecido, switch sem `default`) também vira panic, por consistência.
5. Todo tipo concreto que hoje implementa um marker (`*provider.Provider`,
   `*controller.Controller`, `*graphql.Resolver`, `*middleware.Middleware`, `*filter.Filter`,
   `*emitter.Listener[T]`, `*scheduler.Scheduler`, `*module.Module`,
   `*module.providerAsRef`) ganha `IsToken()`. `*provider.Provider` e `*module.Module`
   trocam `IsExportable()` por `IsToken()` (método antigo removido, marker antigo não existe mais).
6. `gonest.go`: alias `ExportableRef` removido, `TokenRef = module.TokenRef` novo. Os 7 alias
   `XxxRef` existentes continuam (agora cada um embute `TokenRef` por baixo, sem mudança visível
   de uso pra quem só passa 1 valor por vez).
7. Callsites internos (fakes de teste em `internal/module`, `internal/resolver`) e externos
   (consumer `erc`: `app/auth/module.go`, `app/system/module.go`, `infra/database/module.go`)
   migrados -- `erc` troca `[]gonest.ProviderRef` por `[]gonest.TokenRef` e remove o
   `any(providers).([]gonest.ExportableRef)` (spread direto: `m.Exports(providers...)`).

## Out of Scope

| Item | Motivo |
| --- | --- |
| Helper de conversão (`gonest.AsExportable`) | Rejeitado -- resolve sintoma, não causa; usuário quer paridade real com o "token" do Nest, não um workaround. |
| Deprecar/manter `ExportableRef` como alias duplo | Projeto já rejeitou "2 formas de fazer a mesma coisa" (AD-052 bullet 4) -- `TokenRef` substitui limpo. |
| Mudar a leitura (`EffectiveExports`, `validateExports`, `OwnProviders`, etc) | Só a forma de POPULAR os campos internos muda (via `TokenRef` + type-switch); os getters e a lógica de assembly/resolução continuam lendo os mesmos campos tipados (`[]ProviderRef`, `[]ControllerRef`, etc) de sempre. |

## Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| TOKEN-01 | `TokenRef` novo, base de todos os markers + `*Module` | Verified |
| TOKEN-02 | `ExportableRef` removido | Verified |
| TOKEN-03 | 9 métodos builder migrados pra `...TokenRef` com type-switch | Verified |
| TOKEN-04 | Panic fail-fast em token de tipo errado (incl. `Exports` que antes ignorava) | Verified |
| TOKEN-05 | Tipos concretos ganham `IsToken()` | Verified |
| TOKEN-06 | `gonest.go` alias atualizado | Verified |
| TOKEN-07 | Callsites internos + consumer `erc` migrados | Implementing (internos Verified, `erc` pendente T2) |
