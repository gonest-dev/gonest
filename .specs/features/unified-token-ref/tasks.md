# Unified Token (TokenRef) — Tasks

2 tasks sequenciais: T1 core (gonest repo), T2 consumer (erc). Cada uma via Implementer subagent
+ Evaluator (sessão orquestradora, gate rodado do zero antes de aprovar), mesmo padrão de
AD-051/AD-052.

## T1: TokenRef core

**What:**
1. `internal/module/module.go`: adiciona `TokenRef interface { IsToken() }`; todo marker existente
   (`ProviderRef`, `ControllerRef`, `ResolverRef`, `MiddlewareRef`, `FilterRef`, `ListenerRef`,
   `SchedulerRef`) passa a embutir `TokenRef` em vez dos métodos soltos; remove `ExportableRef`
   inteiro; `*Module.IsExportable()` vira `*Module.IsToken()`; reescreve os 9 métodos builder
   (`Imports`, `Providers`, `Controllers`, `Resolvers`, `Use`, `Filters`, `Listeners`,
   `Schedulers`, `Exports`) pra assinatura `...TokenRef` com type-switch/type-assert + panic
   fail-fast no caso inesperado (mensagens nomeando `Module.Xxx` + `%T` do tipo recebido, ver
   design.md pros esqueletos exatos). Doc comments de cada método atualizados citando o novo
   comportamento de panic.
2. `internal/provider/provider.go`: `IsExportable()` → `IsToken()` em `*Provider`.
3. `internal/module/provider_as.go`: `IsExportable()` → `IsToken()` em `*providerAsRef`.
4. `internal/controller/controller.go`, `internal/graphql/resolver.go`,
   `internal/middleware/middleware.go`, `internal/filter/filter.go`,
   `internal/emitter/listener.go`, `internal/scheduler/scheduler.go`: cada tipo concreto ganha
   `IsToken() {}` novo, ao lado do marker method que já tinha.
5. `internal/module/assemble.go`: atualiza doc comments que citam `ExportableRef` por nome (lógica
   de `validateExports` inalterada).
6. Fakes de teste ganham `IsToken()`: `internal/module/module_test.go` (`fakeProvider`,
   `fakeController`, `fakeResolver`, `fakeMiddleware`, `fakeFilter`, e qualquer outro fake de
   marker presente no arquivo), `internal/resolver/{resolver_test.go,direct_test.go}`
   (`fakeProvider`).
7. Testes novos cobrindo o panic path de cada um dos 9 builders (token do tipo errado) --
   `internal/module/module_test.go`, seguindo o padrão de teste de panic já usado no repo
   (checar um exemplo existente antes de escrever).
8. `gonest.go`: remove alias `ExportableRef`, adiciona `TokenRef = module.TokenRef`; doc comment
   do bloco de alias `XxxRef` ganha 1 frase sobre o embed novo.
9. `.specs/project/STATE.md`: nova entrada `AD-056` documentando a decisão (referencia AD-052,
   cita o pedido do usuário de escopo máximo e a rejeição do helper de conversão).
10. `.specs/project/ROADMAP.md`: nova entrada `Milestone 25: Unified Token (TokenRef)`.

**Where:** `internal/module/{module.go,assemble.go,module_test.go,provider_as.go}`,
`internal/provider/provider.go`, `internal/controller/controller.go`,
`internal/graphql/resolver.go`, `internal/middleware/middleware.go`,
`internal/filter/filter.go`, `internal/emitter/listener.go`, `internal/scheduler/scheduler.go`,
`internal/resolver/{resolver_test.go,direct_test.go}`, `gonest.go`, `.specs/project/STATE.md`,
`.specs/project/ROADMAP.md`.

**Depends on:** nada (AD-052 já mergeado).

**Reuses:** todos os getters (`OwnProviders`, `OwnControllers`, `ImportedModules`,
`ExportedProviders`, `EffectiveExports`, `OwnMiddleware`, `OwnFilters`, `OwnListeners`,
`OwnSchedulers`, `OwnExportedModules`) e `assemble.go`/`internal/resolver` -- zero mudança, só leem
campos internos já tipados como sempre.

**Done when:** `ExportableRef` não existe mais em lugar nenhum; os 9 métodos builder de `Module`
aceitam `...TokenRef`; passar o tipo errado pra qualquer um panica com mensagem nomeando
método+tipo; `go build ./...` limpo.

**Tests:** suites existentes (`module_test.go`, `reexport_test.go`, `resolver_test.go`,
`direct_test.go`) passando sem mudança de asserção (só assinatura de entrada mudou); 9 testes
novos de panic path (1 por builder).

**Gate:** `go test ./... -race -count=1` (gonest repo) verde, 24+ pacotes, zero asserção
pré-existente alterada.

---

## T2: Consumer erc migration

**What:** os 3 arquivos com o padrão idêntico trocam `[]gonest.ProviderRef` por
`[]gonest.TokenRef` e removem o `any(providers).([]gonest.ExportableRef)` (spread direto no
`Exports`):
- `C:\dev\leandroluk\erc\ctrl\api\app\auth\module.go`
- `C:\dev\leandroluk\erc\ctrl\api\app\system\module.go`
- `C:\dev\leandroluk\erc\ctrl\api\infra\database\module.go`

**Where:** os 3 arquivos acima.

**Depends on:** T1 (precisa da tag nova do gonest publicada -- ou `go.mod replace` local pra testar
antes da tag, confirmar com Evaluator qual usar).

**Reuses:** nada -- é migração pura de callsite, mesmo shape nos 3 arquivos.

**Done when:** nenhum `gonest.ExportableRef` nem `any(...).([]gonest.ExportableRef)` sobra em
`erc`; os 3 módulos compilam e o `m.Exports(providers...)` não panica mais em runtime.

**Tests:** nenhum teste novo (mudança de callsite, sem lógica de negócio nova) -- critério é
`go build ./...` limpo no repo `erc` + (se houver) suite de testes do `erc` rodando verde.

**Gate:** `go build ./...` (repo `erc`) limpo. Se `erc` tiver testes cobrindo bootstrap desses
módulos, rodar também.
