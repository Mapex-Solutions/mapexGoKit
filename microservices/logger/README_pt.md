# logger — Wrapper singleton de zerolog

Logger estruturado de escopo de processo construído sobre [`rs/zerolog`](https://github.com/rs/zerolog). Inicializado uma vez no boot e usado via helpers de pacote (`Info`, `Warn`, `Error`, …) — não há handle de logger para passar via DI.

> Nome do pacote: `logger` (diretório: `logger/`).

## Superfície

### Inicialização

```go
type LoggerOptions struct {
    ServiceName    string   // ex: "auth-service"
    ServiceVersion string   // ex: "v1.0.0"
    Environment    string   // ex: "development", "production"
    Level          LogLevel
}

func InitLogger(options LoggerOptions) zerolog.Logger
```

Primeira chamada:
- Seta `zerolog.TimeFieldFormat = time.RFC3339`.
- Constrói um logger com `Timestamp` + campos base `env`, `service`, `version`.
- Sink em **stdout**.
- Armazena o resultado no singleton unexported **e** atribui em `zerolog/log.Logger` para que qualquer código que use `log.Info()` de `rs/zerolog/log` enxergue a mesma configuração.

O init é protegido por `sync.Once`: chamadas subsequentes a `InitLogger` retornam a instância existente e não reaplicam opções.

### Níveis (`types.go`)

`LogLevel int8` espelha os níveis numéricos do zerolog para que a API no call site não dependa de zerolog.

| Constante | Valor | Nível zerolog |
|---|---:|---|
| `TraceLevel` | `-1` | trace |
| `DebugLevel` | `0` | debug |
| `InfoLevel` | `1` | info |
| `WarnLevel` | `2` | warn |
| `ErrorLevel` | `3` | error |
| `FatalLevel` | `4` | fatal |
| `PanicLevel` | `5` | panic |
| `DisabledLevel` | `7` | disabled — suprime todo o output |

### Helper de Field

```go
type Field struct {
    Key   string
    Value interface{}
}
```

Todo helper aceita `...Field` variadic. Cada entrada é encaminhada ao event builder via `.Interface(key, value)`.

### Helpers (`methods.go`)

| Função | Comportamento |
|---|---|
| `Log(level zerolog.Level, msg string, fields ...Field)` | Entrada genérica. **Faz panic** se `msg == ""`. |
| `Info(msg, fields...)` | `zerolog.InfoLevel` |
| `Debug(msg, fields...)` | `zerolog.DebugLevel` |
| `Warn(msg, fields...)` | `zerolog.WarnLevel` |
| `Error(err error, msg string, fields...)` | Constrói event `error` com `Err(err)`; se `msg == ""` cai em `err.Error()`. |
| `Panic(msg, fields...)` | **Não loga.** Faz panic com `"\033[31m" + msg + "\033[0m"` (ANSI vermelho). O argumento `fields` é **ignorado**. |

> ⚠️ `Panic` não emite log estruturado apesar da assinatura sugerir o contrário — só dispara um `panic` Go com mensagem colorida em ANSI. Se precisar de panic logado, chame `Log(zerolog.PanicLevel, msg, fields...)` (que loga e faz panic via zerolog).

## Uso

### Bootstrap

```go
logger.InitLogger(logger.LoggerOptions{
    ServiceName:    "auth-service",
    ServiceVersion: "v1.0.0",
    Environment:    os.Getenv("ENV"),
    Level:          logger.InfoLevel,
})
```

### Logging

```go
logger.Info("server started", logger.Field{Key: "port", Value: 8080})

logger.Error(err, "failed to refresh token",
    logger.Field{Key: "userId", Value: userID},
    logger.Field{Key: "tenant", Value: tenantID},
)

logger.Debug("cache hit", logger.Field{Key: "key", Value: cacheKey})
```

## Notas

- Toda saída vai para **stdout**. Não há sink em arquivo/syslog hoje.
- `InitLogger` retorna o `zerolog.Logger` subjacente para callers que precisam da API crua (ex: padrões `.With().Sub()`) manterem uma referência. Mas os helpers de pacote sempre usam o singleton interno.
- `LogLevel` e `zerolog.Level` não são aliases; `toZerologLevel(level LogLevel) zerolog.Level` faz a conversão em um único lugar.
