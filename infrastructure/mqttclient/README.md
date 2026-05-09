# mqttclient — Context-aware MQTT wrapper

Thin opinionated wrapper around [`eclipse/paho.mqtt.golang`](https://github.com/eclipse/paho.mqtt.golang). Hides paho's channel-based token API behind a `context.Context`-aware surface and a flat `Config` struct that mirrors `httpclient.Config` in shape.

## Why a wrapper

- Single integration point so the underlying library can be swapped without touching every caller.
- Flat configuration instead of paho's option-builder.
- `context.Context`-aware `Connect` / `Publish` / `Subscribe`.

**Non-goal:** subscription/consumer lifecycle is out of scope. Saga journeys publish only and observe via HTTP — `Subscribe` is exposed for completeness but not load-tested.

## Surface

### Config

```go
type Config struct {
    BrokerURL      string        // required, e.g. "tcp://host:1883" or "ssl://host:8883"
    ClientID       string        // default: "mapex-<unix-nanos>"
    Username       string
    Password       string
    KeepAlive      time.Duration // default: 30s — broker drop ≈ 1.5×KeepAlive
    ConnectTimeout time.Duration // default: 10s — also bounds Publish/Subscribe waits
    CleanSession   bool          // see "CleanSession behaviour" below
    AutoReconnect  bool          // default: false (drops fail tests instead of silently recovering)
}
```

For Mapex devices: `Username = assetUUID`, `Password =` per-asset MQTT password generated at asset create time.

### Constructor

```go
func New(cfg Config) (*Client, error)
```

Returns an error only when `BrokerURL` is empty. Defaults are applied immediately; the connection is **not** opened — call `Connect`.

### Methods on `*Client`

| Method | Behaviour |
|---|---|
| `Connect(ctx) error` | Opens the connection. Blocks until ack, `ConnectTimeout`, or `ctx` cancel. Idempotent: returns `nil` if already connected. |
| `Disconnect(quiesceMillis uint)` | Closes the connection. `0` = drop immediately; `250` = graceful flush of inflight frames. Safe on never-connected client (no-op, no panic). |
| `IsConnected() bool` | Current connection state. |
| `Publish(ctx, topic, qos, retained, payload) error` | Blocks until broker ack (QoS 1+) or `ctx` cancel. Returns `"mqttclient: not connected"` when called before `Connect`. |
| `Subscribe(ctx, topic, qos, handler) error` | Registers `handler(topic, payload)`. Same not-connected guard as `Publish`. |

Publish/Subscribe wait windows are bounded by `cfg.ConnectTimeout`, not a separate publish timeout.

### Concurrency

`Publish` is safe to call from multiple goroutines (paho serialises internally). `Connect` and `Disconnect` are guarded by an internal mutex so repeated calls during cleanup are well-defined.

## CleanSession behaviour

The wrapper **always sends `CleanSession=true`** on connect:

```go
// applyDefaults
if !c.cfg.CleanSession {
    c.cfg.CleanSession = true
}
```

Because Go cannot distinguish "explicitly false" from the zero value of a `bool`, the wrapper forces `true`. The internal `cfg` field is unexported, so persistent-session mode is **not reachable through the public API today**. If you need it, change `applyDefaults` (or expose a setter).

## Usage

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

## Tested behaviours (`client_test.go`)

- `New(Config{})` rejects empty `BrokerURL`.
- Defaults applied: `ClientID` prefix `mapex-`, `KeepAlive=30s`, `ConnectTimeout=10s`, `CleanSession=true`.
- Caller-supplied `ClientID` is preserved.
- `Publish` / `Subscribe` before `Connect` return a `not connected` error.
- `Disconnect` is idempotent on a never-connected client.
- `Connect` honours context cancellation against unreachable brokers.
