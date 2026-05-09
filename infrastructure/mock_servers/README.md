# mock_servers — In-process protocol mocks for tests and benchmarks

Four self-contained TCP servers that speak just enough of their respective wire protocols to validate publisher executors without any external broker. Every server captures incoming messages into a `chan Message` for assertions, or auto-drains for benchmarks.

| Subpackage | Protocol | Captured fields |
|---|---|---|
| `mqtt` | MQTT 3.1.1 (binary) | `Topic`, `Payload`, `QoS` |
| `nats` | NATS text protocol (incl. HPUB) | `Subject`, `Data`, `Headers` |
| `rabbitmq` | AMQP 0-9-1 (binary, full handshake) | `Exchange`, `RoutingKey`, `Body` |
| `smtp` | SMTP (text) | `From`, `Recipients []string`, `Data` |

## Common API (identical across all four)

```go
// Unit tests — random port, t.Fatal on listen failure.
port, messages, cleanup := mqtt.StartServer(t)
defer cleanup()
// ... drive the system under test against 127.0.0.1:port ...
got := <-messages

// Benchmarks — fixed port, channel auto-drained.
cleanup, err := mqtt.ForBenchmark(1883)
if err != nil { return err }
defer cleanup()
```

`messages` is a buffered channel (`cap = 10`). `cleanup()` closes the listener, waits for in-flight connections via `sync.WaitGroup`, then closes the channel.

Both entry points delegate to a private `startServer(listener)` that owns the accept loop, so the only thing that differs between unit-test and benchmark variants is **how the listener is opened** and **whether the channel is read**.

## Protocol coverage

### `mqtt` — MQTT 3.1.1

Implements the minimum control packets needed by a publisher:

| Packet | Behaviour |
|---|---|
| CONNECT | Replies CONNACK accepted (`0x20 0x02 0x00 0x00`) |
| PUBLISH | Captures `{Topic, Payload, QoS}`. Replies PUBACK only when QoS=1. |
| PINGREQ | Replies PINGRESP (`0xD0 0x00`) |
| DISCONNECT | Closes the connection |

Variable-length integer decoding is implemented via `readVarInt`.

### `nats` — NATS protocol

Sends the `INFO` line on accept advertising `headers:true` so HPUB clients negotiate correctly.

| Verb | Behaviour |
|---|---|
| `CONNECT` | Silent (non-verbose mode) |
| `PING` | Replies `PONG\r\n` |
| `PUB <subj> [reply] <size>` | Captures `{Subject, Data}` |
| `HPUB <subj> [reply] <hsize> <total>` | Parses NATS headers into `Headers map[string]string`, body in `Data` |
| `SUB` / `UNSUB` / blank | Ignored |

### `rabbitmq` — AMQP 0-9-1

Walks the **full** AMQP handshake so any standard client (e.g. `streadway/amqp`) connects cleanly:

```
Protocol Header
  → Connection.Start → StartOk
  → Connection.Tune (channel-max=2047, frame-max=131072, heartbeat=0) → TuneOk
  → Connection.Open → OpenOk
  → Channel.Open → OpenOk
  → (optional) Exchange.Declare → DeclareOk
  → Basic.Publish + Content Header + Content Body  ← captured here
  → Channel.Close → CloseOk
  → Connection.Close → CloseOk
```

Empty-body publishes are emitted as soon as the content-header arrives; non-empty publishes are emitted on the body frame.

### `smtp` — SMTP

| Verb | Behaviour |
|---|---|
| `EHLO` / `HELO` | `250-mock Hello\r\n250-AUTH PLAIN LOGIN\r\n250 OK` |
| `AUTH` | `235 2.7.0 Authentication successful` (always accepts) |
| `MAIL FROM:` | Captures address between `<>` |
| `RCPT TO:` | Appends address between `<>` to `Recipients` |
| `DATA` | `354 Start mail input; end with <CRLF>.<CRLF>` then collects body until `\r\n.\r\n` |
| `QUIT` | `221 2.0.0 Bye` and disconnects |
| anything else | `250 OK` |

The mail body is delivered verbatim in `Message.Data` (CRLF-joined lines, MIME-headers and all).

## Notes

- Listeners always bind to `127.0.0.1`. Tests do not need network reachability.
- The capture channel is a buffered `chan Message` with capacity 10 — a test that produces more than 10 messages without draining will block the handler goroutine. `ForBenchmark` already drains.
- These are deliberately minimal and **not** spec-complete brokers. They reply only to the verbs the Mapex publisher executors actually use.
