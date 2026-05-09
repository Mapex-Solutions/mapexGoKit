# mock_servers — Mocks de protocolo in-process para testes e benchmarks

Quatro servidores TCP autocontidos que falam apenas o suficiente dos respectivos protocolos para validar publisher executors sem nenhum broker externo. Cada servidor captura mensagens recebidas em um `chan Message` para assertions, ou drena automaticamente para benchmarks.

| Subpacote | Protocolo | Campos capturados |
|---|---|---|
| `mqtt` | MQTT 3.1.1 (binário) | `Topic`, `Payload`, `QoS` |
| `nats` | Protocolo texto do NATS (incl. HPUB) | `Subject`, `Data`, `Headers` |
| `rabbitmq` | AMQP 0-9-1 (binário, handshake completo) | `Exchange`, `RoutingKey`, `Body` |
| `smtp` | SMTP (texto) | `From`, `Recipients []string`, `Data` |

## API comum (idêntica nos quatro)

```go
// Testes unitários — porta aleatória, t.Fatal em falha de listen.
port, messages, cleanup := mqtt.StartServer(t)
defer cleanup()
// ... dirigir o sistema sob teste contra 127.0.0.1:port ...
got := <-messages

// Benchmarks — porta fixa, channel auto-drenado.
cleanup, err := mqtt.ForBenchmark(1883)
if err != nil { return err }
defer cleanup()
```

`messages` é um channel buffered (`cap = 10`). `cleanup()` fecha o listener, espera conexões em andamento via `sync.WaitGroup` e então fecha o channel.

Ambos os entry points delegam a um `startServer(listener)` privado que possui o accept loop, então a única diferença entre as variantes de unit-test e benchmark é **como o listener é aberto** e **se o channel é lido**.

## Cobertura de protocolo

### `mqtt` — MQTT 3.1.1

Implementa os pacotes de controle mínimos necessários para um publisher:

| Pacote | Comportamento |
|---|---|
| CONNECT | Responde CONNACK accepted (`0x20 0x02 0x00 0x00`) |
| PUBLISH | Captura `{Topic, Payload, QoS}`. Responde PUBACK apenas quando QoS=1. |
| PINGREQ | Responde PINGRESP (`0xD0 0x00`) |
| DISCONNECT | Fecha a conexão |

Decodificação de inteiro de tamanho variável é feita via `readVarInt`.

### `nats` — Protocolo NATS

Envia a linha `INFO` no accept anunciando `headers:true` para que clientes HPUB negociem corretamente.

| Verbo | Comportamento |
|---|---|
| `CONNECT` | Silencioso (modo não-verbose) |
| `PING` | Responde `PONG\r\n` |
| `PUB <subj> [reply] <size>` | Captura `{Subject, Data}` |
| `HPUB <subj> [reply] <hsize> <total>` | Faz parse dos headers NATS em `Headers map[string]string`, body em `Data` |
| `SUB` / `UNSUB` / linha em branco | Ignorado |

### `rabbitmq` — AMQP 0-9-1

Executa o handshake AMQP **completo** para que qualquer cliente padrão (ex: `streadway/amqp`) conecte sem problemas:

```
Protocol Header
  → Connection.Start → StartOk
  → Connection.Tune (channel-max=2047, frame-max=131072, heartbeat=0) → TuneOk
  → Connection.Open → OpenOk
  → Channel.Open → OpenOk
  → (opcional) Exchange.Declare → DeclareOk
  → Basic.Publish + Content Header + Content Body  ← capturado aqui
  → Channel.Close → CloseOk
  → Connection.Close → CloseOk
```

Publishes de body vazio são emitidos assim que o content-header chega; publishes com body são emitidos no frame de body.

### `smtp` — SMTP

| Verbo | Comportamento |
|---|---|
| `EHLO` / `HELO` | `250-mock Hello\r\n250-AUTH PLAIN LOGIN\r\n250 OK` |
| `AUTH` | `235 2.7.0 Authentication successful` (sempre aceita) |
| `MAIL FROM:` | Captura endereço entre `<>` |
| `RCPT TO:` | Acrescenta endereço entre `<>` em `Recipients` |
| `DATA` | `354 Start mail input; end with <CRLF>.<CRLF>` e coleta o body até `\r\n.\r\n` |
| `QUIT` | `221 2.0.0 Bye` e desconecta |
| qualquer outro | `250 OK` |

O body do email é entregue literalmente em `Message.Data` (linhas separadas por CRLF, incluindo headers MIME e tudo o mais).

## Notas

- Listeners sempre fazem bind em `127.0.0.1`. Testes não precisam de rede acessível.
- O channel de captura é um `chan Message` buffered com capacidade 10 — um teste que produzir mais de 10 mensagens sem drenar bloqueará a goroutine do handler. `ForBenchmark` já drena.
- São deliberadamente mínimos e **não** são brokers spec-complete. Respondem apenas aos verbos que os publisher executors do Mapex realmente usam.
