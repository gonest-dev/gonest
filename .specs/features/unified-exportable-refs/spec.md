# Unified Exports (ExportableRef) — Spec

## Context

AD-051 (`module-reexport`) adicionou `Module.ExportModules(mods ...*Module)` como método
SEPARADO de `Module.Exports(ps ...ProviderRef)`, com decisão explícita registrada em
design.md linha 219: não unificar porque Go não tem union type e forçar `*Module` a
implementar `ProviderRef` seria "abstração falsa" (Module não é um tipo de Provider).

Essa decisão é revertida aqui à luz da premissa fixada em PROJECT.md (Goals): **paridade de
API com NestJS vence pureza idiomática Go quando colidem.** No Nest, `@Module({ exports: [...] })`
aceita providers e módulos no MESMO array — é exatamente essa forma que gonest deve replicar,
mesmo que a Go idiomática preferisse dois métodos.

## User Story

Como usuário do gonest vindo de NestJS, quero escrever `m.Exports(someProvider, someModule)`
num único array — igual ao `exports: [SomeProvider, SomeModule]` do Nest — em vez de precisar
saber que módulos vão num método (`ExportModules`) e providers em outro (`Exports`).

## Requirements

1. WHEN um dev chama `m.Exports(p)` com `p` satisfazendo `ProviderRef` THEN o provider é
   registrado exatamente como hoje (`m.exports`) — comportamento existente preservado.
2. WHEN um dev chama `m.Exports(mod)` com `mod` sendo `*Module` THEN o módulo é registrado
   como re-export transitivo (`m.exportedModules`), mesmo efeito que `ExportModules(mod)` tinha.
3. WHEN um dev mistura os dois no mesmo call, `m.Exports(p, mod)` THEN ambos são roteados
   corretamente pro storage certo, na mesma chamada.
4. `Module.ExportModules` é REMOVIDO (não fica como alias/deprecated) — API única, sem 2
   caminhos pro mesmo conceito, mesma filosofia "um jeito de fazer" já seguida no resto do
   builder.
5. `EffectiveExports()`, `validateExports` (assemble.go), `OwnExportedModules()`,
   `ExportedProviders()` continuam com o MESMO comportamento — só a forma de POPULAR
   `m.exports`/`m.exportedModules` muda, não a leitura.
6. Todo callsite interno (`erc` consumer, examples, testes) migrado de `ExportModules(x)` pra
   `Exports(x)`.

## Out of Scope

- Não reabre a discussão de `ImportModules`/`ImportProviders` (rejeitada em conversa anterior —
  `Imports`/`Providers`/`Controllers` não são "imports", são declarações do próprio módulo).
- Não muda `Imports` (só módulos, sempre foi assim, sem ambiguidade a resolver).

## Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| EXPORT-01 | Exports aceita ProviderRef | Verified (comportamento preexistente) |
| EXPORT-02 | Exports aceita *Module | Implementing |
| EXPORT-03 | Exports aceita mix na mesma chamada | Implementing |
| EXPORT-04 | ExportModules removido | Implementing |
| EXPORT-05 | Leitura (EffectiveExports/validateExports) inalterada | Implementing |
| EXPORT-06 | Callsites migrados | Implementing |
