# State

**Last Updated:** 2026-07-12
**Current Work:** Project setup — PROJECT.md + ROADMAP.md prontos, aguardando primeira feature (Milestone 1)

---

## Recent Decisions (Last 60 days)

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

## Active Blockers

_Nenhum ainda._

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

---

## Todos

_Nenhum ainda._

---

## Preferences

**Model Guidance Shown:** never
