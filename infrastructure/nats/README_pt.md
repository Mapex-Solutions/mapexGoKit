# nats — Cliente JetStream, consumers, KV, FANOUT, DLQ

Wrapper sobre [`nats.go`](https://github.com/nats-io/nats.go) e [`nats.go/jetstream`](https://github.com/nats-io/nats.go/tree/main/jetstream). Cobre todo o conjunto de padrões que o Mapex usa em um único barramento:

- **Publish** JetStream (sync/async, dedup, controle de concorrência otimista)
- Publish core NATS **fire-and-forget** + flush em batch
- **Consumer pull-based** gerenciado com prefetch double-buffer, retries, DLQ, gerações de handler V1 + V2
- **FANOUT** (broadcast efêmero) sobre JetStream
- **Bucket KV** com CAS (Compare-And-Swap)
- **Publish agendado** via header `Nats-Schedule`
- Manutenção de stream: `EnsureStream`, `PurgeStreamSubject`, `HasPendingMessages`

> Nome do pacote: `natsModel` (diretório: `nats/`).

## Arquitetura em um diagrama

```
                        ┌────────────┐  ports.go
                        │  Publisher │
                        │ Subscriber │
                        │   Fetcher  │
   service code ───────▶│   Fanout   │◀── *Bus  ──┐
                        │ CorePub.   │             │
                        │ ScheduleM. │             │
                        └────────────┘             │
                                                   ▼
                                       ┌────────────────────┐
                                       │   *Client          │  nats.go +
                                       │  nc, js (JetStream)│  jetstream
                                       └────────┬───────────┘
                                                │
                          Push / PublishCore / StartConsumer / KV / FANOUT
```

`Bus` é um adapter fino que implementa todas as portas e delega ao `Client`. O código de serviço depende das *interfaces* em `ports.go`, não do `Client` diretamente.

## Construção

### `Config`

```go
type Config struct {
    Options   nats.Options                    // construído via nats.GetDefaultOptions().*, nats.Servers(...), etc.
    OnConnect func(c *Client)                 // chamado após connect inicial e a cada reconnect
}
```

### `New(cfg) (*Client, error)`

1. `cfg.Options.Connect()` abre o `*nats.Conn` subjacente.
2. `jetstream.New(nc)` constrói o contexto JetStream.
3. Se `OnConnect` está setado, é invocado uma vez e um reconnect handler é registrado para re-invocá-lo após cada reconexão (logado como `[INFRA:NATS] Reconnected — calling OnConnect callback`).
4. Loga `[INFRA:NATS] Connected to server`.

Métodos de `*Client`:

| Método | Propósito |
|---|---|
| `Ping() (time.Duration, error)` | Round-trip nativo PING/PONG via `nc.RTT()`. Erra quando `nc.IsClosed()`. |
| `IsConnected() bool` | `nc != nil && !nc.IsClosed()`. |
| `Close()` | Idempotente — seguro em cliente já fechado. |

`Bus` envolve um `Client`:

```go
bus := natsModel.NewBus(client)
_ = (*natsModel.Bus)(nil) // implementa: Publisher, Subscriber, Fetcher, Fanout, CorePublisher, ScheduleManager
bus.GetConn() // *nats.Conn cru para padrões request-reply (Auth Callout)
```

## Portas (interfaces em `ports.go`)

```go
type Publisher       interface { Publish(PublishConfig) error }
type Subscriber      interface { Subscribe(SubscribeConfig) (stop func() error, err error) }
type Fetcher         interface { Fetch(FetchConfig) (stop func() error, err error) }
type Fanout          interface { PublishFanout(ctx, subject, data); SubscribeFanout(stream, serviceName, subject, FanoutHandler); EnsureFanoutStream(FanoutStreamConfig) }
type CorePublisher   interface { PublishCore(PublishCoreConfig); FlushConnection() }
type ScheduleManager interface { PublishScheduled(ScheduledPublishConfig); PurgeStreamSubject(stream, subject); HasPendingMessages(stream, subject) (bool, error) }
type ConnectionProvider interface { GetConn() *nats.Conn }
type KeyValueStore   interface { Get; Put; Create; Update; Delete; Purge; Keys; Bucket }
```

## Publishing

### `Push` JetStream (sync ou async)

```go
err := bus.Publish(natsModel.PublishConfig{
    Ctx:     ctx,                          // opcional
    Subject: "events.raw",                 // obrigatório
    Data:    payload,                      // any — JSON-marshalado pelo Bus
    Headers: map[string]string{"x-trace": traceID},
})
```

Por baixo, `Client.Push(PushOptions)` adiciona headers opcionais:

| Field | Header setado |
|---|---|
| `MsgId` | `Nats-Msg-Id` (deduplicação dentro da janela `Duplicates` do stream) |
| `ExpectStream` | `Nats-Expected-Stream` |
| `ExpectLastMsgId` | `Nats-Expected-Last-Msg-Id` (concorrência otimista) |

Quando `Async=true`, `Push` espera no **future por mensagem** (`future.Ok()` / `future.Err()`) — não em `PublishAsyncComplete()` — para evitar o "convoy effect" onde toda goroutine bloqueia em todas as mensagens pendentes. Honra `Ctx` primeiro, depois `Timeout`, depois `context.Background()`.

### Core NATS fire-and-forget

Para batches de alta vazão:

```go
for _, e := range events {
    _ = bus.PublishCore(natsModel.PublishCoreConfig{
        Subject: "events.raw",
        Data:    e,
        MsgId:   e.ID, // dedup se o stream tem janela Duplicates
    })
}
_ = bus.FlushConnection() // único TCP roundtrip para o batch inteiro
```

`PublishCore` enfileira no buffer TCP sem esperar ACK. `FlushConnection()` é o que garante que os bytes saíram da máquina.

### Publish agendado (header `Nats-Schedule`)

```go
err := bus.PublishScheduled(natsModel.ScheduledPublishConfig{
    Subject:       "schedules.events",
    TargetSubject: "events.delayed",
    ScheduleAt:    time.Now().Add(10*time.Minute),
    Data:          payload,
    MsgId:         "evt-123", // dedup dentro do stream de schedule
})
```

O wrapper seta `Nats-Schedule: @at <RFC3339>` e `Nats-Schedule-Target: <subject>`. **Exige que o stream alvo tenha `AllowMsgSchedules: true`.**

## Subscribing

Há três caminhos de subscribe com ergonomias diferentes. Escolha exatamente um por consumer.

### `Bus.Subscribe` — callback simples

```go
stop, err := bus.Subscribe(natsModel.SubscribeConfig{
    Stream:  "EVENTS",
    Subject: "events.>",
    Durable: "events-printer",
    Handler: func(data []byte) error { fmt.Println(string(data)); return nil },
    // Pull: true → usa pull consumer
})
defer stop()
```

Auto-cria o stream e o consumer. `nil` no handler → `Ack`. Erro → `Nak` (redeliver imediato, sem retry/DLQ).

### `Bus.Fetch` — `BatchMode` opt-in para processamento bulk

```go
stop, err := bus.Fetch(natsModel.FetchConfig{
    Stream:    "EVENTS",
    Subject:   "events.>",
    Durable:   "events-bulk",
    BatchMode: true,
    BatchSize: 100,
    BatchHandler: func(msgs []natsModel.BatchMessage) error {
        // insert bulk em DB. nil → ACK todas; erro → NAK todas.
        return nil
    },
})
```

Padrões `BatchSize=10`, `Timeout=5s`. Setar `Handler` e `BatchHandler` ao mesmo tempo é rejeitado. `BatchHandler` exige `BatchMode=true`. Sem `BatchMode`, a chamada cai em `Subscribe(Pull=true)`.

### `Bus.StartConsumer` — gerenciado, ciclo de vida completo, retry + DLQ

Esse é o caminho de produção. `ConsumerOptions` tem **duas gerações de handler** — set apenas uma:

| Geração | Handlers | Modelo de controle |
|---|---|---|
| Legacy V1 | `Handler` (goroutines por mensagem) / `BatchHandler` (bulk) | Retorno `nil` → ACK todas; erro → NAK todas. **Sem retry/DLQ.** |
| Novo V2 (recomendado) | `MessageHandlerV2(*Message)` / `BatchMessageHandlerV2([]*Message)` | Caller chama `msg.Ack()` / `msg.Nack(err)` / `msg.Reject(reason)` / `msg.Term()`. **Retry + DLQ suportados.** |

O wrapper `*Message` da V2 carrega:

```go
Data    []byte
Headers map[string][]string
Subject string
DeliveryCount int
Timestamp     time.Time

// preenchido pelo caller antes de Reject/Nack para que o payload de DLQ os contenha
OrgId, PathKey, EventTrackerId string
```

#### Métodos de ciclo de vida V2

| Método | Comportamento |
|---|---|
| `Ack()` | Remove a mensagem do stream. |
| `Nack(err)` | Sem `RetryPolicy`: redelivery imediato. Com `RetryPolicy`: `NakWithDelay(GetDelayForAttempt(DeliveryCount))`; quando `DeliveryCount > MaxRetries`, manda para DLQ e ACK. |
| `Reject(reason)` | Vai direto para DLQ e ACK (sem retry — dado fatal/inválido). |
| `Term()` | Descarta silenciosamente — sem retry, sem DLQ. |

Se `Nack`/`Reject` disparam sem `DLQPolicy`, a mensagem é `Term()` (logada em Warn).

### Padrões de `ConsumerOptions` (`setConsumerDefaults`)

| Field | Padrão |
|---|---|
| `BatchSize` | `50` |
| `FetchTimeout` | `1 * time.Second` |
| `RetryDelay` | `2 * time.Second` |
| `MaxRetries` | `5` (retries do consumer-loop em **fetch** com falha) |
| `StopOnError` | `false` |
| `DuplicateWindow` (em `createOrGetConsumer`) | `15 * time.Minute` |

`MaxAckPending` no consumer JetStream é `BatchSize × 2` com piso de `128` (folga para double-buffer: um batch processando + um batch prefetched).

### Auto-provisioning de stream/consumer (`createOrGetConsumer`)

Idempotente:

- Se o stream está ausente → cria: `WorkQueuePolicy`, `FileStorage`, `Duplicates = DuplicateWindow`.
- Se o stream existe mas não captura o subject (match exato ou prefixo wildcard `*.>`) → atualiza o stream para incluir o subject.
- Se a janela `Duplicates` do stream difere da desejada → atualiza.
- Se o consumer está ausente → cria: `AckExplicitPolicy`, `DeliverAllPolicy`, `FilterSubject`, `MaxAckPending=128` (ou override). Quando `RetryPolicy` é informado, `MaxDeliver = MaxRetries + 1` e opcionalmente `AckWait = RetryPolicy.AckWait`. O `AckWait` no servidor é rede de proteção (valor alto); o backoff por tentativa é aplicado **client-side** via `NakWithDelay`.
- Se o consumer existe com `MaxAckPending` diferente do solicitado → deleta e recria.

### Prefetch double-buffer (`Consumer.start`)

O consumer mantém **um prefetch sempre em vôo**: enquanto o worker processa o batch N, o batch N+1 está sendo buscado. Quando o stream está vazio, `cons.Fetch(MaxWait)` bloqueia no servidor e funciona como rate-limiting natural; quando há dados, o próximo batch chega com 0 ms de idle.

Notas do loop de fetch:

- `processBatch` imprime **duas linhas em branco** em stdout antes de cada batch (separador visual). Remova se incomodar.
- Em `jetstream.ErrNoMessages` o loop continua sem contar contra `MaxRetries`.
- Em outros erros de fetch dorme `RetryDelay`. Após `MaxRetries` falhas consecutivas, o comportamento depende de `StopOnError`: para o consumer ou reseta o contador.

### Roteamento V1 vs V2 (`processBatch`)

Ordem de prioridade (apenas um dispara por batch):

1. `BatchMessageHandlerV2`
2. `MessageHandlerV2` (goroutines por mensagem)
3. `BatchHandler` (bulk, baseado em retorno)
4. `Handler` (goroutines por mensagem, baseado em retorno)

`validateConsumerOptions` rejeita chamadas que setam zero ou mais de um handler.

### Ciclo de vida de `Consumer`

| Método | Notas |
|---|---|
| `Stop()` | Idempotente via flag `stopped` (não atômico — chamadas concorrentes compartilham a mesma intenção; o primeiro fecha o `stopChan`). |
| `IsRunning() bool` | `!stopped`. |
| `GetOptions() ConsumerOptions` | Snapshot. |

`ConsumerManager` é um map em memória chaveado por nome: `Add`, `Stop`, `StopAll`, `Get`.

## Política de retry (`retry.go`)

```go
func DefaultRetryPolicy() *RetryPolicy {
    return &RetryPolicy{
        MaxRetries: 5,
        Backoff:    []time.Duration{1*time.Second, 5*time.Second, 30*time.Second, 2*time.Minute, 10*time.Minute},
        AckWait:    5 * time.Minute,
    }
}
```

| Helper | Efeito |
|---|---|
| `GetBackoffDurations()` | Cai em padrão para nil/vazio. |
| `GetDelayForAttempt(deliveryCount)` | `index = deliveryCount - 1`, clamped na última entrada. |
| `GetMaxDeliver()` | `MaxRetries + 1` (NATS conta a entrega original). Padrão `6`. |
| `GetAckWait()` | Cai em `30 * time.Second` se não setado. |
| `ShouldRetry(deliveryCount)` | `deliveryCount <= MaxRetries`. Trata `nil` como `MaxRetries=5`. |

## DLQ (`dlq.go`)

```go
func DefaultDLQPolicy(serviceName string) *DLQPolicy {
    return &DLQPolicy{
        Stream:      "MAPEXOS-DLQ",
        Subject:     "dlq.mapexos",
        ServiceName: serviceName,
        ServiceType: "unknown",
        EventType:   "unknown",
    }
}
```

O payload da DLQ é um JSON `DLQMessage` com:

- Identidade: `ID` (UUID v4), `EventTrackerId`
- Tenant: `OrgId`, `PathKey` (obrigatórios para filtragem multi-tenant)
- Contexto de serviço: `ServiceName`, `ServiceType`, `EventType`
- Original: `OriginalSubject`, `OriginalStream`, `OriginalData` (JSON cru), `OriginalHeaders`
- Erro: `LastError`, `ErrorCount`
- Entrega: `FirstDelivery`, `LastDelivery`, `TotalDeliveries`
- Consumer: `ConsumerName`
- Timestamps: `SentToDLQAt`

`Reject()` e `Nack()` (após max retries) publicam esse payload em `DLQPolicy.GetSubject()` e fazem `Ack` na original. Se o publish da DLQ falha, a original é `Nak()` (fallback).

## FANOUT (broadcast)

```go
err := bus.EnsureFanoutStream(natsModel.FanoutStreamConfig{
    Name:     "MAPEXOS-FANOUT",
    Subjects: []string{"mapexos.cache.>"},
    // Padrões: MaxAge=5m, MaxMsgs=10000, MaxBytes=10MB
})

sub, err := bus.SubscribeFanout("MAPEXOS-FANOUT", "assets-svc", "mapexos.cache.invalidate.assets",
    func(data []byte) error { handleInvalidate(data); return nil })
defer sub.Stop()

err = bus.PublishFanout(ctx, "mapexos.cache.invalidate.assets", payload)
```

Notas de implementação:

- O stream FANOUT usa `MemoryStorage`, `LimitsPolicy`, `Replicas: 1`, `Discard: DiscardOld`.
- O nome do consumer efêmero é `<serviceName>-fanout-<YYYYMMDD-HHMMSS>-<8 hex chars>`. O sufixo aleatório (via `utils/random.GenerateSessionID(4)`) impede colisões quando um serviço registra múltiplos subscribers fanout no mesmo segundo — sem ele, o segundo `CreateOrUpdateConsumer` sobrescreveria o primeiro e quebraria seu `FilterSubject`.
- `DeliverNewPolicy`, `AckWait=30s`, `InactiveThreshold=5min`.
- Erros do handler são logados em Warn mas a mensagem é **sempre ACK**.

## KV (bucket com CAS)

```go
store, err := client.CreateKeyValue(natsModel.KVConfig{
    Bucket:   "WORKFLOW-INSTANCES",
    Replicas: 3,
    History:  1,
    Storage:  jetstream.FileStorage, // padrão
})

rev, _ := store.Put("inst:123", payload)

entry, _ := store.Get("inst:123")
newRev, err := store.Update("inst:123", newPayload, entry.Revision)
if errors.Is(err, natsModel.ErrKVCASConflict) {
    // re-leia e tente de novo
}
```

| Método | Erros traduzidos |
|---|---|
| `Get(key)` | `jetstream.ErrKeyNotFound` → `ErrKVKeyNotFound` |
| `Put(key, value)` | nenhum específico (sempre create-or-overwrite) |
| `Create(key, value)` | `jetstream.ErrKeyExists` → `ErrKVKeyExists` |
| `Update(key, value, expectedRevision)` | `jetstream.ErrKeyExists` (usado como marca de CAS) → `ErrKVCASConflict` |
| `Delete(key)` | `jetstream.ErrKeyNotFound` → `ErrKVKeyNotFound` |
| `Purge(key)` | passa direto |
| `Keys()` | `jetstream.ErrNoKeysFound` → `[]string{}` |
| `Bucket() string` | nome do bucket subjacente |

Padrões: `Replicas=1`, `History=1`, `Storage=FileStorage`.

## Manutenção de stream

| Método | Comportamento |
|---|---|
| `Bus.EnsureStream(jetstream.StreamConfig)` | `CreateOrUpdateStream` — idempotente. |
| `Bus.PurgeStreamSubject(streamName, subject)` | Retorna `nil` se o stream não existe. |
| `Bus.HasPendingMessages(streamName, subject)` | `false` se o stream não existe. Inspeciona `info.State.Subjects` via subject-filter. |

## Helpers de teste

`message.go` expõe um entry point para testes unitários que não precisa de conexão NATS real:

```go
msg := natsModel.NewTestMessage([]byte("payload"), 0, &natsModel.TestMessageCallbacks{
    OnAck:    func() error { acked = true; return nil },
    OnNack:   func(err error) error { nacked = err; return nil },
    OnReject: func(reason string) error { rejected = reason; return nil },
})
yourHandler(msg)
```

Mocks da superfície pública do Bus vivem em `nats/mocks/`.

## Erros

| Sentinel | Origem |
|---|---|
| `ErrMissingHandler` | (definido; não levantado pelo código atual) |
| `ErrMissingSubject` | `Push` / `PublishCore` com subject vazio |
| `ErrMaxRetriesExceeded`, `ErrDLQPublishFailed`, `ErrMessageMetadataFailed`, `ErrDLQNotConfigured` | (definidos; não levantados pelo código atual — erros crus são embrulhados via `fmt.Errorf`) |
| `ErrKVKeyNotFound`, `ErrKVKeyExists`, `ErrKVCASConflict` | Operações KV |

> O arquivo de erros se chama `erros.go` (typo no source — mesmo pacote, sem impacto para o caller).

## Aliases (re-exports)

```go
type Msg         = nats.Msg
type Option      = nats.Option
type Options     = nats.Options
type StorageType = jetstream.StorageType
```

## Exemplo end-to-end (V2 + retry + DLQ)

```go
client, err := natsModel.New(natsModel.Config{Options: nats.GetDefaultOptions()})
if err != nil { return err }
defer client.Close()
bus := natsModel.NewBus(client)

// Provisionamento idempotente do stream
_ = bus.EnsureStream(jetstream.StreamConfig{
    Name:       "EVENTS-RAW",
    Subjects:   []string{"events.raw"},
    Retention:  jetstream.WorkQueuePolicy,
    Storage:    jetstream.FileStorage,
    Duplicates: 15 * time.Minute,
})

consumer, err := bus.StartConsumer(natsModel.ConsumerOptions{
    Stream:  "EVENTS-RAW",
    Subject: "events.raw",
    Durable: "events-processor",
    BatchSize: 100,
    RetryPolicy: natsModel.DefaultRetryPolicy(),
    DLQPolicy: &natsModel.DLQPolicy{
        ServiceName: "events-service",
        ServiceType: "processor",
        EventType:   "raw",
    },
    BatchMessageHandlerV2: func(messages []*natsModel.Message) {
        for _, msg := range messages {
            evt, err := parse(msg.Data)
            if err != nil {
                msg.Reject("invalid payload") // → DLQ agora
                continue
            }
            msg.OrgId = evt.OrgId       // preencha antes de Nack/Reject para a DLQ ter contexto de tenant
            msg.PathKey = evt.PathKey
            msg.EventTrackerId = evt.TrackerID

            if err := process(evt); err != nil {
                msg.Nack(err) // backoff retry; DLQ após MaxRetries
                continue
            }
            msg.Ack()
        }
    },
})
if err != nil { return err }
defer consumer.Stop()
```
