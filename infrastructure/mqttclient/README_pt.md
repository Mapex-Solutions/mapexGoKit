# mqttclient — Wrapper MQTT com suporte a context

Wrapper enxuto e opinativo sobre [`eclipse/paho.mqtt.golang`](https://github.com/eclipse/paho.mqtt.golang). Esconde a API baseada em channels/tokens do paho atrás de uma superfície orientada a `context.Context` e uma struct `Config` plana, no mesmo formato de `httpclient.Config`.

## Por que um wrapper

- Ponto único de integração para que a biblioteca subjacente possa ser trocada sem alterar todos os callers.
- Configuração plana em vez do option-builder do paho.
- `Connect` / `Publish` / `Subscribe` cientes de `context.Context`.

**Fora de escopo:** ciclo de vida de subscription/consumer. Saga journeys apenas publicam e observam via HTTP — `Subscribe` é exposto por completude, mas não passou por load-test.

## Superfície

### Config

```go
type Config struct {
    BrokerURL      string        // obrigatório, ex: "tcp://host:1883" ou "ssl://host:8883"
    ClientID       string        // padrão: "mapex-<unix-nanos>"
    Username       string
    Password       string
    KeepAlive      time.Duration // padrão: 30s — drop do broker ≈ 1,5×KeepAlive
    ConnectTimeout time.Duration // padrão: 10s — também limita esperas de Publish/Subscribe
    CleanSession   bool          // ver "Comportamento de CleanSession" abaixo
    AutoReconnect  bool          // padrão: false (drops fazem testes falharem ao invés de se recuperarem em silêncio)
}
```

Para dispositivos Mapex: `Username = assetUUID`, `Password =` senha MQTT por-asset gerada no momento da criação.

### Construtor

```go
func New(cfg Config) (*Client, error)
```

Retorna erro apenas quando `BrokerURL` está vazio. Defaults são aplicados imediatamente; a conexão **não** é aberta — chame `Connect`.

### Métodos em `*Client`

| Método | Comportamento |
|---|---|
| `Connect(ctx) error` | Abre a conexão. Bloqueia até ack, `ConnectTimeout`, ou cancelamento de `ctx`. Idempotente: retorna `nil` se já conectado. |
| `Disconnect(quiesceMillis uint)` | Fecha a conexão. `0` = drop imediato; `250` = flush gracioso de frames pendentes. Seguro em cliente nunca conectado (no-op, sem panic). |
| `IsConnected() bool` | Estado atual da conexão. |
| `Publish(ctx, topic, qos, retained, payload) error` | Bloqueia até ack do broker (QoS 1+) ou cancelamento. Retorna `"mqttclient: not connected"` quando chamado antes de `Connect`. |
| `Subscribe(ctx, topic, qos, handler) error` | Registra `handler(topic, payload)`. Mesma proteção de não-conectado de `Publish`. |

Janelas de espera em Publish/Subscribe são limitadas por `cfg.ConnectTimeout`, não há timeout separado de publish.

### Concorrência

`Publish` é seguro para múltiplas goroutines (paho serializa internamente). `Connect` e `Disconnect` são protegidos por um mutex interno, garantindo comportamento bem definido em chamadas repetidas durante cleanup.

## Comportamento de CleanSession

O wrapper **sempre envia `CleanSession=true`** no connect:

```go
// applyDefaults
if !c.cfg.CleanSession {
    c.cfg.CleanSession = true
}
```

Como Go não consegue diferenciar "explicitamente false" do valor zero de `bool`, o wrapper força `true`. O campo `cfg` interno é unexported, então o modo de sessão persistente **não é alcançável pela API pública hoje**. Se precisar, altere `applyDefaults` (ou exponha um setter).

## Uso

```go
c, err := mqttclient.New(mqttclient.Config{
    BrokerURL: "tcp://broker:1883",
    Username:  assetUUID,
    Password:  mqttPassword,
})
if err != nil { return err }

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := c.Connect(ctx); err != nil { return err }
defer c.Disconnect(250)

if err := c.Publish(ctx, "mapex/asset/"+assetUUID+"/telemetry", 1, false, payload); err != nil {
    return err
}
```

## Comportamentos testados (`client_test.go`)

- `New(Config{})` rejeita `BrokerURL` vazio.
- Defaults aplicados: prefixo `mapex-` no `ClientID`, `KeepAlive=30s`, `ConnectTimeout=10s`, `CleanSession=true`.
- `ClientID` informado pelo caller é preservado.
- `Publish` / `Subscribe` antes de `Connect` retornam erro `not connected`.
- `Disconnect` é idempotente em cliente nunca conectado.
- `Connect` respeita cancelamento de contexto contra brokers inacessíveis.
