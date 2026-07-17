# reflexão: Suporte a Microserviços (Futuro)

No ecossistema NestJS, a arquitetura de microserviços é construída em cima de uma abstração fantástica de **Transportes** (TCP, Redis, NATS, MQTT, gRPC, RabbitMQ, Kafka). 

O grande diferencial do NestJS é que o modelo de programação permanece **exatamente o mesmo** da API HTTP: você usa o conceito de Controllers para escutar mensagens, a Injeção de Dependência para orquestrar serviços, e Filters/Interceptors/Guards funcionam nativamente.

Transpondo essa filosofia para a arquitetura baseada em closures (Builders) do Gonest, teríamos o seguinte design estrutural:

## 1. Separação Semântica: Consumers vs Controllers

No NestJS, um Controller pode expor rotas HTTP (`@Get`) e handlers de mensageria (`@MessagePattern` e `@EventPattern`) simultaneamente. 
Apesar de prático, semanticamente "Controller" remete fortemente ao mundo REST/HTTP.

No Gonest, nós corrigiríamos essa leve confusão arquitetural do Nest criando um Builder específico para microserviços: o **Consumer**.
Um `Consumer` pertence a um Módulo (ex: `module.Consumers(OrderConsumer)`) da mesma forma que um Controller, mas é dedicado a escutar brokers.

- **Message (RPC):** Requer-Resposta. O microserviço recebe a mensagem, processa e devolve um valor.
- **Event (Pub/Sub):** Fire-and-forget. O microserviço apenas escuta o evento, sem devolver resposta para quem enviou.

O código ficaria perfeitamente segregado do REST:

```go
var OrderConsumer = gonest.NewConsumer(func(c *gonest.Consumer) {
  // Injeção de dependência funciona igual
  orderService := gonest.MustInject[*OrderService](c)

  // Message Pattern (Request-Response RPC via Broker)
  c.Message("create_order", func(m *gonest.MessageRoute) {
    // Reutiliza o mesmo Schema do REST para documentação (AsyncAPI) e validação
    m.Payload(createOrderSchema)
    
    m.Handler(func(ctx *gonest.MicroserviceContext) {
      // ctx.Payload() devolve um Parseable, consumido pela MESMA
      // gonest.MustParse[T] já usada em REST/GraphQL/gRPC (unified-parse-api
      // feature) -- nenhuma função MustXxx nova por transporte.
      payload := gonest.MustParse[CreateOrderDTO](ctx.Payload(), createOrderSchema)

      // ctx.Reply envia o retorno de volta pelo broker (RPC)
      ctx.Reply(orderService.Create(payload))
    })
  })

  // Event Pattern (Fire-and-Forget / PubSub via Broker)
  c.Event("payment_processed", func(e *gonest.EventRoute) {
    e.Payload(paymentEventSchema)
    
    e.Handler(func(ctx *gonest.MicroserviceContext) {
      payload := gonest.MustParse[PaymentEventDTO](ctx.Payload(), paymentEventSchema)
      orderService.UpdateStatus(payload.OrderId, payload.Status)
      
      // Sem ctx.Reply(), pois é apenas um evento Pub/Sub
    })
  })
})
```

## 2. Abstração de Contexto (O triunfo do Parseable já existente)

Conforme discutido no suporte a GraphQL e gRPC, o Gonest reaproveitaria a
interface `gonest.Parseable` que já existe hoje (unified-parse-api feature)
para compartilhar os Schemas de validação -- nenhuma interface nova por
transporte, só mais um método (`ctx.Payload()`) devolvendo um `Parseable`.

O `*gonest.MicroserviceContext` saberia exatamente como fazer o "unmarshal" da mensagem (seja ela JSON vindo do Redis ou do RabbitMQ, ou Binário vindo do Kafka) e entregaria o dado para o `gonest.Schema` rodar suas restrições (`Required`, `Min`, `Max`) por trás de `ctx.Payload().ParseInto(...)`, exatamente como `jsonBodySource`/`formBodySource` já fazem para REST.

Se um dado inválido chegar pelo broker, a camada do Gonest rejeita a mensagem automaticamente, disparando um `BadRequestException` (RPC Exception) de volta para o chamador, sem que o `orderService` seja sequer acionado.

## 3. ClientProxy (Enviando Mensagens)

Para se comunicar com outros microserviços, o NestJS utiliza o conceito de `ClientProxy`. No Gonest, registraríamos o Client como um Provider e o injetaríamos em qualquer lugar.

```go
// Registra o cliente que aponta para o microserviço de Faturamento
var BillingClientProvider = gonest.NewClientProxy("BILLING_SERVICE", func(client *gonest.ClientOptions) {
  client.Transport(gonest.TransportRedis)
  client.Url("redis://localhost:6379")
})

var OrderService = gonest.NewProvider(func(p *gonest.Provider) {
  p.Scope(gonest.ScopeSingleton)
  
  billingClient := gonest.MustInjectClient("BILLING_SERVICE", p) // Retorna *gonest.ClientProxy
  
  p.Constructor(func() *orderServiceImpl {
    return &orderServiceImpl{ billing: billingClient }
  })
})

type orderServiceImpl struct {
  billing *gonest.ClientProxy
}

func (s *orderServiceImpl) Create(data OrderData) any {
  order := s.db.Save(data)
  
  // Dispara um EVENTO (fire-and-forget) para o serviço de faturamento
  s.billing.Emit("order_created", order)
  
  // Envia uma MENSAGEM (RPC request-response) e aguarda o retorno (síncrono/bloqueante no Go)
  response, err := s.billing.Send("process_fraud_check", order)
  
  return order
}
```

## 4. Inicialização Híbrida da Aplicação

Assim como no NestJS (usando `app.connectMicroservice()`), o Gonest permitiria que uma mesma aplicação rodasse um servidor HTTP e se conectasse a múltiplos brokers simultaneamente durante o bootstrap.

```go
func bootstrap() error {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})

  // Conecta ao Redis para escutar as rotas c.Message e c.Event
  app.ConnectMicroservice(func(micro *gonest.MicroserviceOptions) {
    micro.Transport(gonest.TransportRedis)
    micro.Url("redis://localhost:6379")
  })
  
  // Conecta ao RabbitMQ também (suporta múltiplos transportes ao mesmo tempo)
  app.ConnectMicroservice(func(micro *gonest.MicroserviceOptions) {
    micro.Transport(gonest.TransportRMQ)
    micro.Url("amqp://localhost:5672")
    micro.Queue("orders_queue")
  })

  // StartAllMicroservices sobe os consumers de RabbitMQ, Redis, etc em goroutines (background)
  app.MustStartAllMicroservices()

  // MustListen bloqueia a main goroutine segurando o servidor HTTP no ar
  app.MustListen(":3000")
  
  return nil
}
```

### Por que isso é revolucionário no ecossistema Go?
Em Go, a maioria das pessoas cria microserviços conectando manualmente a bibliotecas de NATS, RabbitMQ ou Kafka, enchendo o código de lógicas de retry, ack/nack, unmarshal e switch-cases infinitos para rotear eventos.

Com essa abstração, o Gonest cuidaria de:
- **Roteamento:** A string `"create_order"` bate direto na closure de handler apropriada.
- **Decodificação e Validação:** O `gonest.Schema` já valida os dados recebidos pelo broker.
- **Resiliência:** Acknowledgement (Ack/Nack) automático. Se o handler panicar, o Gonest recusa a mensagem (nack). Se rodar com sucesso, ele aceita (ack).
- **Consistência Híbrida Semântica:** A aplicação pode servir rotas HTTP via `Controllers` e comandos assíncronos via `Consumers`, compartilhando os mesmos `Providers` (serviços de negócio), mas mantendo as portas de entrada bem separadas arquiteturalmente.
