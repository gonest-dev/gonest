# gonest

**Vision:** Framework HTTP em Go com DX equivalente ao NestJS — DI, módulos, controllers, pipeline de request (middleware/guard/interceptor/filter/pipe) e metadata-driven validation/OpenAPI, tudo idiomático em Go (sem reflect mágico, sem decorators, builders type-safe via generics).
**For:** devs JS/TS já confortáveis com NestJS, avaliando migrar backend pra Go.
**Solves:** frameworks Go atuais (Gin, Echo, Fiber puro) exigem reaprender modelo mental do zero — sem DI estruturado, sem módulos, sem convenção de exception→resposta HTTP. gonest replica os padrões do Nest que essas pessoas já dominam, baixando a barreira de migração.

## Goals

- DX percebida como "Nest, só que Go" — dev NestJS lê um exemplo do gonest e reconhece o padrão sem precisar de doc extra.
- **Paridade de API com NestJS vence pureza idiomática Go quando os dois colidem.** Onde o Nest usa uma API unificada (ex: `@Module({ exports: [...] })` aceitando providers E módulos no mesmo array), gonest replica essa forma mesmo que o equivalente Go "puro" preferisse métodos separados por tipo — o objetivo é baixar atrito de quem já pensa em Nest, não maximizar idiomaticidade Go isolada. Ver AD-052 (`.specs/project/STATE.md`) para o caso concreto que fixou esta premissa.
- Zero reflect em runtime crítico (validação/DI) onde builder explícito resolver — reflect só onde Go genuinely não permite alternativa (ex: tipo de campo em `Property(&t.X)`).
- Cobertura das primitivas centrais do Nest (módulo, DI, pipeline de request, validação+OpenAPI) documentadas em INSIGHT.md antes de qualquer código de produção.

## Tech Stack

**Core:**

- Framework: gonest (próprio, em construção)
- Language: Go (versão a definir — mínimo compatível com generics, 1.18+)
- HTTP adapter: Fiber (`github.com/gofiber/fiber`) — único adapter no v1, sem camada de abstração multi-adapter ainda

**Key dependencies:**

- `github.com/gofiber/fiber` — servidor HTTP
- `github.com/google/uuid` — request-id (exemplo de middleware)
- `golang.org/x/sync/errgroup` — resolução paralela do bootstrap (DI graph)
- OpenAPI 3.1 — schema gerado a partir do metadata builder (`gonest.NewMetadata`)

## Scope

**v1 includes:**

- Core DI + Module/Controller/Route — `NewApp`, `NewProvider`, `NewController`, `NewModule`, bootstrap paralelo via `errgroup` (grafo de import resolvido antes do app subir)
- Pipeline de request completo — Middleware, Guard, Interceptor, Filter, Pipe (equivalente Nest), aplicável por controller ou globalmente
- Metadata builder — `NewMetadata`/`Property` com builder linear (branches tipados: String/Integer/Email/Array/Object/etc), alimenta validação runtime (`MustJsonBody`/`MustResolve`) e schema OpenAPI 3.1 a partir da mesma declaração
- Testing helpers — `MustNewTestApp`, `MustOverride` (mock por interface), `MustRequest`/asserts

**Explicitly out of scope (v1):**

- Abstração multi-adapter HTTP (net/http, Echo, Gin) — só Fiber por enquanto
- Emitter (event-emitter), Scheduler (cron/interval/timeout), Terminus/health check — ficam documentados no INSIGHT.md como referência futura, não entram no v1
- CLI de scaffolding (equivalente `nest new`/`nest generate`)
- Microservices/transport layer (equivalente `@nestjs/microservices`)

## Constraints

- Recursos: time pequeno/solo — tasks precisam ser atômicas e sequenciais, sem paralelismo de execução assumido por padrão.
- Timeline: sem deadline fixo — prioridade é acertar a DX (design da API) antes de crescer escopo.
- Técnico: Go não tem parâmetro de tipo em método (só em func/tipo livre) — todo design de builder precisa respeitar essa limitação (já resolvido no metadata builder via valor capturado em vez de `[T]` em método).

## Workflow Conventions

Regras de processo, não de código — como qualquer sessão (humana ou agente) deve conduzir
trabalho neste repo, independente da feature. Valem pra toda sessão futura, não só a que as
escreveu.

- **Idioma da fala do agente: pt-br.** Toda comunicação conversacional (respostas, updates,
  resumos) do agente é em português brasileiro. Código, commits, nomes de identificador,
  comentários no código e conteúdo do README.md continuam em inglês (convenção já
  estabelecida, não muda) — só a FALA do agente com o usuário é pt-br.
- **Trabalho sempre separado por milestone.** Nenhum lote de trabalho mistura 2+ milestones
  num único commit/PR — cada milestone fecha (Specify→Design→Tasks→Execute conforme o
  tamanho) antes do próximo começar. Commit + push acontece ao fim de CADA milestone, não só
  no fim de uma sessão inteira.
- **Cada milestone gera uma tag de versão nova.** Ao fechar um milestone (commit já na
  `main`), cortar e dar push numa tag `vX.Y.Z` (`v0.{major}.{minor}` — ver "Next Steps" do
  README.md pro esquema de versionamento) referenciando o commit daquele milestone
  especificamente, não um commit posterior que já inclui outro milestone.
- **Sempre usar subagentes quando possível.** Pesquisa, design, implementação de tasks e
  atualização de docs do site devem ser delegadas a subagentes (papel Implementer, seguindo o
  padrão Planner/Implementer/Evaluator já estabelecido — ver "Subagent workflow convention"
  em STATE.md) em vez de feitas inline pela sessão orquestradora, sempre que a tarefa permitir
  isolamento de contexto. A sessão orquestradora mantém os papéis de Planner (specify/design) e
  Evaluator (rodar gate checks, revisar diff, decidir commit).
- **README.md sempre atualizado.** Todo milestone que adiciona/muda API pública precisa
  refletir no README.md (seção "Implementation Status" quando aplicável, mais qualquer seção
  de "Documentation" relevante ao recurso) como parte do PRÓPRIO milestone, não como débito
  posterior.
- **Novo recurso relevante ganha `.examples/`.** Feature nova com superfície de uso real
  (não puramente interna) ganha um exemplo runnable em `.examples/` demonstrando o fluxo
  ponta a ponta, verificado ao vivo (curl/dispatch real) antes do milestone ser considerado
  fechado — mesmo padrão já seguido por toda feature anterior deste projeto.
- **Mudança documental → atualizar o site.** Qualquer mudança em `.specs/`, README.md, ou
  comportamento de API pública que afete a documentação pública dispara atualização do repo
  irmão `C:\dev\gonest-dev\site` (Next.js/fumadocs, `gonest.dev`) — commit + push separado
  desse repo, nos 3 idiomas mantidos (en/pt/es), seguindo o padrão de commit `docs: ...` já
  em uso lá.
