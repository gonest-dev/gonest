# Tasks: Modernização Go 1.27 — Generic Methods & Convenção Parse/Safe

### TASK-GEN-001: Atualizar runtime Go baseline para 1.27.0
- **REQ**: REQ-GEN-001
- **Symbols**: `go.mod`
- **What**: Atualizar diretiva de versão do Go em `go.mod` para `go 1.27.0` e rodar `go mod tidy`.
- **Where**: `go.mod`
- **Depends on**: none
- **Done when**: `go.mod` exibe `go 1.27.0` e compila limpo.
- **Gate**: `go vet ./...`

### TASK-GEN-002: Implementar generic methods de DI em `internal/inject` e tipos proprietários
- **REQ**: REQ-GEN-002, REQ-GEN-003
- **Symbols**: `Controller.Inject`, `Provider.Inject`, `InjectAll`, `SafeInject`
- **What**: Adicionar métodos genéricos `Inject[T any]() T`, `InjectAll[T any]() []T` e `SafeInject[T any]() (T, error)` diretamente em `*Controller`, `*Provider`, `*Middleware`, `*Guard`, `*Interceptor`, `*Filter`, `*Listener`, `*Resolver`, `*LazyModule`, `*Test`.
- **Where**: `internal/inject/inject.go`, `internal/controller/controller.go`, `internal/provider/provider.go`, `internal/middleware/middleware.go`, etc.
- **Depends on**: TASK-GEN-001
- **Done when**: Chamadas `c.Inject[*Service]()` compilam e resolvem dependências.
- **Gate**: `go test ./internal/inject ./internal/controller ./internal/provider`

### TASK-GEN-003: Implementar Provider Interface Binding `(p *Provider) As[T]()`
- **REQ**: REQ-GEN-004
- **Symbols**: `Provider.As`
- **What**: Adicionar método genérico `(p *Provider) As[T any]() ProviderRef` delegando para `module.ProviderAs[T](p)`.
- **Where**: `internal/provider/provider.go`, `internal/module/provider_as.go`
- **Depends on**: TASK-GEN-002
- **Done when**: `p.As[IMyInterface]()` registra com sucesso a view de interface.
- **Gate**: `go test ./internal/module ./internal/provider`

### TASK-GEN-004: Remodelar `execution.Parseable` como struct com `Parse[T]` e `SafeParse[T]`
- **REQ**: REQ-GEN-005
- **Symbols**: `Parseable.Parse`, `Parseable.SafeParse`
- **What**: Converter `Parseable` em struct concreta que encapsula a interface interna `ParseInto` e expõe `Parse[T any](s *Schema) T` e `SafeParse[T any](s *Schema) (T, error)`.
- **Where**: `internal/execution/request.go`, `internal/validate/`
- **Depends on**: TASK-GEN-001
- **Done when**: `req.Body().Json().Parse[*DTO](s)` compila e valida payloads.
- **Gate**: `go test ./internal/execution ./internal/validate`

### TASK-GEN-005: Atualizar `internal/app` para padrão `NewApp`/`SafeNewApp`, `Listen`/`SafeListen`, e Test Helpers
- **REQ**: REQ-GEN-006, REQ-GEN-007, REQ-GEN-008
- **Symbols**: `App.Listen`, `App.SafeListen`, `TestBuilder.Override`, `Test.Request`, `Test.SafeRequest`
- **What**: Reorganizar `NewApp` direto (retorna `*App`), criar `SafeNewApp`, atualizar `Listen`/`SafeListen`, `SetupSwagger`/`SafeSetupSwagger`, `NewTestApp`, `TestBuilder.Override[T]` e `Test.Request`/`SafeRequest`.
- **Where**: `internal/app/app.go`, `internal/app/test_app.go`
- **Depends on**: TASK-GEN-002, TASK-GEN-004
- **Done when**: Testes do pacote `internal/app` passam com as novas assinaturas.
- **Gate**: `go test ./internal/app`

### TASK-GEN-006: Adicionar `(e *Emitter) Subscribe[T]` e `(d *Dotenv) Parse[T]`
- **REQ**: REQ-GEN-009, REQ-GEN-010
- **Symbols**: `Emitter.Subscribe`, `Dotenv.Parse`, `Dotenv.SafeParse`
- **What**: Transformar `Subscribe` em método em `*Emitter` e adicionar `Parse[T]`/`SafeParse[T]` em `*Dotenv`.
- **Where**: `internal/emitter/subscribe.go`, `internal/emitter/emitter.go`, `internal/dotenv/dotenv.go`
- **Depends on**: TASK-GEN-001
- **Done when**: Testes de emitter e dotenv compilam e passam.
- **Gate**: `go test ./internal/emitter ./internal/dotenv`

### TASK-GEN-007: Atualizar `gonest.go` público e limpar funções globais legadas
- **REQ**: REQ-GEN-002, REQ-GEN-003, REQ-GEN-004, REQ-GEN-005, REQ-GEN-006, REQ-GEN-007, REQ-GEN-008, REQ-GEN-009
- **Symbols**: `gonest.go`
- **What**: Remover `MustInject`, `MustInjectAll`, `ProviderAs`, `MustOverride`, `Subscribe`, `Parse`, `MustParse`, `MustNewApp`, `MustListen`, `MustSetupSwagger`, `MustNewTestApp`. Expor `NewApp`, `SafeNewApp`, `NewTestApp`.
- **Where**: `gonest.go`
- **Depends on**: TASK-GEN-002 até TASK-GEN-006
- **Done when**: `gonest.go` compila limpo sem as funções antigas.
- **Gate**: `go vet .`

### TASK-GEN-008: Atualizar todos os testes internos do repositório
- **REQ**: REQ-GEN-001 a REQ-GEN-010
- **Symbols**: `*_test.go`
- **What**: Migrar todas as chamadas `MustInject`, `MustParse`, `MustNewApp`, `MustListen`, etc. nos 25 pacotes de teste para as novas chamadas diretas.
- **Where**: Todos os `*_test.go`
- **Depends on**: TASK-GEN-007
- **Done when**: `go test ./... -race` passa verde em 100% dos pacotes.
- **Gate**: `go test ./... -race`

### TASK-GEN-009: Atualizar os exemplos em `.examples/*`
- **REQ**: REQ-GEN-001 a REQ-GEN-010
- **Symbols**: `.examples/*`
- **What**: Atualizar os projetos de exemplo para consumir os novos métodos (`c.Inject`, `p.As`, `req.Body().Json().Parse`, `NewApp`, `Listen`).
- **Where**: `.examples/`
- **Depends on**: TASK-GEN-008
- **Done when**: Exemplos compilam sem erros de tipo.
- **Gate**: `pwsh -Command "Get-ChildItem -Path .examples -Filter go.mod -Recurse | ForEach-Object { pushd $_.DirectoryName; go test ./...; popd }"`

### TASK-GEN-010: Atualizar documentação do site `../site` nos 3 idiomas
- **REQ**: REQ-GEN-011
- **Symbols**: `content/docs/**/*.{mdx,pt.mdx,es.mdx}`
- **What**: Atualizar referências de API e blocos de código em `../site` substituindo `MustInject` por `c.Inject`, `MustParse` por `.Parse`, `MustNewApp` por `NewApp`, etc.
- **Where**: `../site/content/docs/`
- **Depends on**: TASK-GEN-008
- **Done when**: Nenhum arquivo no site mantém a sintaxe legada.
- **Gate**: `git -C ../site diff` verificado.

### TASK-GEN-011: Commit e Push em ambos os repositórios
- **REQ**: REQ-GEN-011
- **Symbols**: git commits
- **What**: Criar commits convencionais nos repositórios `gonest` e `../site`, garantindo que ambos estão sincronizados e com push realizado para `origin/main`.
- **Where**: `gonest` e `../site`
- **Depends on**: TASK-GEN-009, TASK-GEN-010
- **Done when**: Ambos os repositórios estão limpos e com push efetuado.
- **Gate**: `git status` e `git -C ../site status`
