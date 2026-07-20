# Module Reference / Lazy Loading — Insight

Rascunho de reflexão (não spec formal) sobre o item pendente de `TODO.md`: "Module Reference / Lazy
Loading: Busca manual de dependências e importação tardia para otimizar tempo de start ou resolver
dependências circulares." Ao contrário de Dynamic Modules (`INSIGHT-DYNAMIC.md`, resolvido de graça),
este item é 3 coisas DIFERENTES empacotadas junto no Nest, com respostas diferentes cada uma.

## As 3 partes, confirmadas via Context7 (`nestjs/docs.nestjs.com`)

### 1. Busca manual de dependência (`ModuleRef.get()`)

Já resolvido -- `MustInject[T](owner)`/`MustInjectAll[T](owner)` JÁ é a busca manual, explícita, por
tipo, dentro do escopo de um módulo. Diferença real: `ModuleRef.get(SomeService)` do Nest é resolução
por TOKEN em runtime (sem checagem de tipo em compile-time, já que é reflection/decorator-based);
`MustInject[T]` do gonest é resolvido em compile-time via generics -- estritamente mais forte, não uma
lacuna. **Nada a fazer aqui.**

### 2. Lazy Loading de módulo (`LazyModuleLoader.load(() => import('./lazy.module'))`)

Confirmado via docs real: `LazyModuleLoader#load` faz `await import(...)` -- um **dynamic import de
JS**, carregando bytecode que ainda não tinha sido baixado/parseado, cacheado após a primeira chamada.
Documentado explicitamente: "lifecycle hook methods are not invoked in lazy loaded modules" (hooks de
`internal/app/lifecycle.go`, Milestone 20, não rodariam pra um módulo lazy -- consistente com o próprio
Nest).

**Hipótese central: isso não tem equivalente idiomático em Go, e não é por falta de trabalho -- é uma
diferença estrutural de runtime.** `import()` dinâmico em JS carrega CÓDIGO que ainda não existia na
memória do processo. Um binário Go é compilado -- TODO o código de TODO módulo já está no binário
final, sempre, não importa se é `Imports`'d ou não. Não existe "atrasar o carregamento do código" em
Go (a stdlib tem `plugin`, mas é notoriamente frágil, só Linux/macOS, e não é usado em nenhum lugar
deste projeto ou do ecossistema Go idiomático em geral). Motivação real do Nest pra Lazy Loading
(otimizar cold-start em serverless/webworkers evitando `import()` de módulos não usados naquela
invocação) **não se aplica** a um binário Go, que já paga esse custo inteiro em build-time, não em
tempo de request.

O que SERIA um paralelo honesto, mas é uma feature DIFERENTE: gonest's Stage 3
(`internal/resolver/stage3.go`) resolve TODO Provider `Singleton` registrado, eager, concorrente,
durante `NewApp` -- confirmado lendo o comentário real do `Resolve`: "Every provider registered in any
of modules is resolved, not only ones reachable via a recorded pending edge". Não existe hoje uma forma
de dizer "não rode o `Constructor` deste Provider até ele ser realmente pedido pela primeira vez". Isso
seria **"Lazy Providers"**, não "Lazy Modules" -- código já compilado e presente, só a INSTANCIAÇÃO
adiada.

**Exemplo real e plausível** (não teórico -- adaptado do próprio FAQ oficial do Nest,
`content/faq/serverless.md`, confirmado via Context7): o Nest documenta lazy loading condicional por
TIPO de invocação serverless --

```typescript
if (workerType === WorkerType.A) {
  const { WorkerAModule } = await import('./worker-a.module');
  const moduleRef = await this.lazyModuleLoader.load(() => WorkerAModule);
  // ...
} else if (workerType === WorkerType.B) {
  const { WorkerBModule } = await import('./worker-b.module');
  const moduleRef = await this.lazyModuleLoader.load(() => WorkerBModule);
  // ...
}
```

O cenário Go equivalente É real e comum: um binário Lambda/Cloud Function ÚNICO que atende múltiplos
tipos de evento (gatilho S3, gatilho SQS, API Gateway) via um único `main()`/handler. Hoje, se
`AppModule` importa os 3 módulos (`S3WorkerModule`, `SQSWorkerModule`, `ApiWorkerModule`), Stage 3
instancia os 3 conjuntos de Provider (abre client S3 E client SQS E pool de DB) em TODO cold start,
mesmo que só UM desses caminhos vá ser usado nessa invocação específica -- exatamente o desperdício que
motivou a feature no Nest, só que resolvido lá via não-carregar-o-código; aqui o código já está
carregado (é um binário só), o desperdício é só a INSTANCIAÇÃO das dependências caras. Um
`LazyProvider[T]` (nome provisório) que adia `Constructor` pro primeiro `MustInject` de verdade
resolveria esse caso específico sem precisar de dynamic loading nenhum -- só disciplina sobre QUANDO
rodar `Constructor`, não SE o código existe.

### 3. Dependência circular (`forwardRef()`)

Confirmado via docs real: `forwardRef(() => X)` é usado em DOIS lugares -- `Module.imports` (pra
módulos que se importam mutuamente) e `@Inject(forwardRef(() => X))` (pra Providers que dependem um do
outro). O problema que resolve é, de novo, **específico de JS**: numa dependência circular de arquivos
(`a.ts` importa `b.ts` que importa `a.ts`), a classe referenciada pode ainda não estar definida no
momento em que o decorator roda -- `forwardRef` embrulha a referência numa função (`() => X`),
adiando a avaliação pra depois que ambos os arquivos já carregaram.

**Esse problema específico (referência a classe/módulo ainda não definida por causa de ordem de
import) não existe em Go** -- o compilador Go resolve toda a dependência de PACOTES/tipos
estaticamente antes de rodar qualquer coisa; não tem "ainda não carregado" em runtime pra um tipo
Go. O que SOBRA do problema, meio que escondido atrás do `forwardRef` no Nest, é o problema de fundo
de verdade: uma dependência circular real no GRAFO DE INJEÇÃO (Provider A precisa de uma instância de
B, B precisa de uma instância de A) -- isso é um problema de ORDEM DE INICIALIZAÇÃO genuíno, que existe
em QUALQUER linguagem, JS ou Go. `internal/resolver/cycle.go`'s `DetectCycle` (DFS 3-cores, roda ANTES
de Stage 3) já PEGA esse caso -- mas só DETECTA e falha (erro alto e claro), não oferece nenhum jeito
de RESOLVER um ciclo legítimo (mesmo que raro/desencorajado, o próprio Nest trata isso como "geralmente
sinal de problema de design", mas ainda assim dá um escape hatch).

**Exemplo real e plausível** (não teórico -- é literalmente o exemplo canônico do doc oficial de
circular dependency do Nest, `content/fundamentals/circular-dependency.md`, confirmado via Context7):
`CatsService` e `CommonService` precisam um do outro --

```typescript
// CommonService precisa chamar de volta pra CatsService (ex: invalidar algo
// específico de gato quando um evento genérico de "entidade mudou" dispara)
@Injectable()
export class CommonService {
  constructor(
    @Inject(forwardRef(() => CatsService))
    private catsService: CatsService,
  ) {}
}

// CatsService também precisa de CommonService (ex: logging/utilitário
// compartilhado que TODO domínio usa, inclusive Cats)
@Injectable()
export class CatsService {
  constructor(
    @Inject(forwardRef(() => CommonService))
    private commonService: CommonService,
  ) {}
}
```

Cenário real: um serviço de domínio (`CatsService`) que depende de um serviço utilitário genérico
(`CommonService`, ex: logging/auditoria/cache compartilhado) -- comum -- MAS o utilitário genérico
também precisa chamar de volta pro domínio específico (ex: `CommonService.onEntityChanged` precisa
perguntar pro `CatsService` algo específico de gato). Ciclo genuíno, não erro de design bobo.

**Hipótese concreta pra gonest, SE fizer sentido**: um `LazyInject[T](owner) func() T` (nome
provisório) -- em vez de devolver `T` já resolvido, devolve uma FUNÇÃO que resolve `T` na primeira
chamada (lazy, memoizado depois). Um Provider poderia declarar essa dependência SEM que ela vire uma
edge que `DetectCycle` considera pra detecção de ciclo (já que a resolução real só acontece depois que
TODO o grafo -- incluindo o ciclo -- já foi montado, não durante `Constructor`). Isso espelha o papel
real que `forwardRef()` cumpre no Nest (quebrar a ORDEM de inicialização, não "resolver" o ciclo de
verdade -- o ciclo lógico continua existindo, só a INSTANCIAÇÃO para de exigir ordem linear).

**Diferença estrutural importante vs Nest**: no exemplo do Nest, os DOIS lados usam `forwardRef`
(`CatsService` E `CommonService`), porque o problema que resolvem lá é "classe ainda não definida" --
simétrico, os dois arquivos sofrem do mesmo problema de ordem de import. Em gonest, o problema real é
só ORDEM DE INICIALIZAÇÃO (não definição de tipo) -- só UM dos dois lados precisaria de `LazyInject`
pra quebrar o ciclo (ex: só `CommonServiceProvider` usa `LazyInject[*CatsService]`, `CatsServiceProvider`
continua usando `MustInject[*CommonService]` normal) -- mais simples que o Nest, não simétrico.

## Conclusão preliminar (pré-Discuss)

| Parte | Status | Ação |
| --- | --- | --- |
| Busca manual de dependência | Já resolvido (`MustInject`) | Nenhuma -- já é estritamente melhor que `ModuleRef.get()` |
| Lazy Loading de módulo | Sem paralelo idiomático em Go (compilado, não runtime-loaded) | Documentar por que não existe, não implementar |
| Lazy Providers (instanciação adiada, NÃO é o que o Nest chama de Lazy Loading) | Feature real, mas sem caso de uso concreto ainda | Fora de escopo até aparecer pedido real |
| Dependência circular (`DetectCycle`) | Já detecta e falha alto | Mantém -- é o comportamento correto por padrão |
| Escape hatch pra ciclo legítimo (`LazyInject[T]`) | Gap real, não implementado | Candidato a Discuss/Specify formal, SE o usuário tiver um caso real batendo nisso (não construir especulativamente) |

**Recomendação**: marcar "Busca manual de dependência" como coberta (já é `MustInject`). Manter "Lazy
Loading" e "dependências circulares" como itens SEPARADOS e mais precisos no `TODO.md` (o item atual
mistura 3 coisas com respostas bem diferentes) -- e não implementar nada disso especulativamente; o
`LazyInject[T]` só vale a pena se um ciclo real aparecer num módulo de verdade, não como exercício
teórico.
