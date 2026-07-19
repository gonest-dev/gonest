# GoNest - Features Roadmap (Baseado no NestJS)

Este documento mapeia as funcionalidades do [NestJS (docs.nestjs.com)](https://docs.nestjs.com) em relação ao que já existe no **GoNest** e o que ainda falta implementar.

---

## 🟢 1. Core & Visão Geral (Implementado)

A espinha dorsal do framework já está muito bem estabelecida, provando que a arquitetura foi bem adaptada do TS para o Go.

- [x] **Modules (`internal/module`)**: Agrupamento lógico de componentes, exportação/importação.
- [x] **Controllers (`internal/controller`, `internal/route`)**: Mapeamento de rotas e declaração de endpoints usando Fluent API.
- [x] **Providers/Services (`internal/provider`)**: Injeção de dependência e serviços.
- [x] **Middleware (`internal/middleware`)**: Funções executadas antes do ciclo de request.
- [x] **Exception Filters (`internal/filter`, `internal/exception`)**: Camada de captura e padronização de erros da aplicação.
- [x] **Pipes/Validation (`internal/validate`, `internal/schema`)**: Validação de schema, transformação de DTOs e extração (Params, Query, Body).
- [x] **Guards (`internal/guard`)**: Interceptação focada em autorização e permissões (role-based).
- [x] **Interceptors (`internal/interceptor`)**: AOP (Aspect-Oriented Programming) via HOFs para cache, log, mutação de resposta.
- [x] **Injection Scopes (`internal/scope`)**: Singleton, Request e Transient scopes.
- [x] **Execution Context (`internal/execution`)**: Contexto unificado que abstrai o HTTP Adapter subjacente (como no NestJS com Express/Fastify).
- [x] **Platform Adapters (`internal/adapter`)**: Adaptadores para motores HTTP (o Fiber atua como o motor padrão).

---

## 🟡 2. Fundamentos Específicos & Técnicas (Parcial / Em Discussão)

Algumas features do Nest dependem de Decorators nativos, e no GoNest foram traduzidas para um paradigma idiomático do Go.

- [x] **Custom Decorators**: Traduzidos para funções de Wrapper (Higher-Order Functions) ou métodos de configuração em `Route`.
- [x] **Event Emitter (`internal/emitter`)**: Disparo de eventos assíncronos dentro da aplicação (Padrão Pub/Sub local).
- [x] **Task Scheduling (`internal/scheduler`)**: Cron jobs declarativos (`@Cron`, `@Timeout`, `@Interval`).
- [x] **Lifecycle Hooks**: Equivalentes do `OnModuleInit`, `OnApplicationBootstrap`, `OnModuleDestroy`, `BeforeApplicationShutdown`, `OnApplicationShutdown`. Como inicializar DBs ou parar graceful shutdowns coordenados pelos módulos. (`Provider.OnModuleInit`/etc, Milestone 20 -- ver `.specs/project/ROADMAP.md`)
- [ ] **Dynamic Modules**: Permite que módulos sejam customizados em tempo de inicialização usando `Register` ou `forRoot` recebendo configurações dinâmicas (ex: ConfigModule.forRoot()).
- [ ] **Module Reference / Lazy Loading**: Busca manual de dependências e importação tardia para otimizar tempo de start ou resolver dependências circulares.

---

## 🔴 3. Integrações e Técnicas (Falta Fazer)

Aqui entram os pacotes equivalentes ao ecossistema `@nestjs/*`, que fornecem integrações com ferramentas externas famosas.

- [x] **Configuração (ConfigModule)**: Pacote unificado para carregar `.env`, definir variáveis globais e injetar o `ConfigService` com tipagem segura. (`internal/dotenv`, Milestone 19 -- ver `.specs/project/ROADMAP.md`)
- [ ] **Cache (CacheModule)**: Gerenciamento unificado de cache (em memória ou Redis) que interaja bem com o `Interceptor` do GoNest.
- [ ] **Database / ORM Integration**: Prover módulos plugáveis que envelopam as libs famosas em Go.
  - [ ] Módulo nativo para `database/sql`
  - [ ] Integração com `GORM`
  - [ ] Integração com `Ent`
- [ ] **Filas (Queues / Bull)**: Processamento de jobs em background baseados em Redis ou RabbitMQ.
- [ ] **Autenticação (Passport)**: Módulo de autenticação abstraindo estratégias de JWT, OAuth2, Local (Session).
- [ ] **Versioning**: Suporte a versionamento de rotas e controllers (`v1`, `v2`, Headers, URL Path).
- [ ] **Rate Limiting (ThrottlerModule)**: Prevenção de abusos limitando a quantidade de requisições por IP ou token.
- [ ] **Serialization**: Padronização e exclusão de propriedades (equivalente ao `class-transformer` com `@Exclude()`).

---

## 🔵 4. Ecossistemas Paralelos

O NestJS não vive só de REST HTTP. Ele tem motores paralelos para diferentes protocolos.

- [x] **GraphQL (`internal/graphql`, `internal/resolver`)**: Geração Code-First de SDL, Resolvers, Mutations, Queries. (Inclui suporte robusto recém implementado de realtime/subscriptions).
- [x] **OpenAPI / Swagger (`internal/openapi`)**: Geração automática de documentação a partir da AST/Schema validation das rotas.
- [ ] **WebSockets (@nestjs/websockets)**: Gateways abstratos para Socket.io / ws. Gerenciamento de rooms e conexões stateful fora do GraphQL.
- [ ] **Microservices (@nestjs/microservices)**: Arquitetura de mensageria onde Controllers reagem a eventos/patterns (gRPC, TCP, Redis Pub/Sub, Kafka, RabbitMQ, NATS).

---

## 🛠 5. Ferramentas (DevX)

- [ ] **CLI do GoNest (Nest CLI)**: Ferramenta de linha de comando (`gonest new`, `gonest g resource`, `gonest g controller`) para scaffolding de arquivos sem quebrar o padrão arquitetural.
- [ ] **Módulo de Testes Unitários**: Auxiliares para facilitar a montagem do "Testbed" sem ter que levantar o servidor web de fato (como `Test.createTestingModule()`).

---

## 🚦 Próximos Passos (Workflow TLC)

O GoNest utiliza o fluxo **TLC Spec-Driven** (`.specs/project/ROADMAP.md`).
Para iniciar qualquer um dos itens listados acima, basta disparar o processo de especificação:

> **"specify feature ConfigModule"** ou **"plan feature CLI do GoNest"**

Isso criará a pasta da feature em `.specs/features/`, elaborando os requisitos (`spec.md`), a arquitetura (`design.md`) e os passos atômicos (`tasks.md`) de forma adaptativa antes de escrever o código.
