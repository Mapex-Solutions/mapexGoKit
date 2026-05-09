# logger — Singleton zerolog wrapper

Process-wide structured logger built on [`rs/zerolog`](https://github.com/rs/zerolog). Initialised once at boot and used through package-level helpers (`Info`, `Warn`, `Error`, …) — there is no logger handle to thread through DI.

> Package name: `logger` (directory: `logger/`).

## Surface

### Initialisation

```go
type LoggerOptions struct {
    ServiceName    string   // e.g. "auth-service"
    ServiceVersion string   // e.g. "v1.0.0"
    Environment    string   // e.g. "development", "production"
    Level          LogLevel
}

func InitLogger(options LoggerOptions) zerolog.Logger
```

First call:
- Sets `zerolog.TimeFieldFormat = time.RFC3339`.
- Builds a logger with `Timestamp` + base context fields `env`, `service`, `version`.
- Sinks to **stdout**.
- Stores the result in the unexported singleton **and** assigns it to `zerolog/log.Logger` so any code using `log.Info()` from `rs/zerolog/log` sees the same configuration.

The init is guarded by `sync.Once`: subsequent `InitLogger` calls return the existing instance and do not re-apply options.

### Levels (`types.go`)

`LogLevel int8` mirrors zerolog's numeric levels so the API does not depend on zerolog at the call site.

| Constant | Value | zerolog level |
|---|---:|---|
| `TraceLevel` | `-1` | trace |
| `DebugLevel` | `0` | debug |
| `InfoLevel` | `1` | info |
| `WarnLevel` | `2` | warn |
| `ErrorLevel` | `3` | error |
| `FatalLevel` | `4` | fatal |
| `PanicLevel` | `5` | panic |
| `DisabledLevel` | `7` | disabled — suppresses all output |

### Field helper

```go
type Field struct {
    Key   string
    Value interface{}
}
```

Every helper accepts a variadic `...Field`. Each entry is forwarded to the event builder via `.Interface(key, value)`.

### Helpers (`methods.go`)

| Function | Behaviour |
|---|---|
| `Log(level zerolog.Level, msg string, fields ...Field)` | Generic entry. **Panics** if `msg == ""`. |
| `Info(msg, fields...)` | `zerolog.InfoLevel` |
| `Debug(msg, fields...)` | `zerolog.DebugLevel` |
| `Warn(msg, fields...)` | `zerolog.WarnLevel` |
| `Error(err error, msg string, fields...)` | Builds an `error` event with `Err(err)`; if `msg == ""` falls back to `err.Error()`. |
| `Panic(msg, fields...)` | **Does not log.** Panics with `"\033[31m" + msg + "\033[0m"` (red ANSI). The `fields` argument is **ignored**. |

> ⚠️ `Panic` does not actually emit a structured log event despite its signature suggesting otherwise — it only triggers a Go `panic` with an ANSI-coloured message. If you need a logged panic, call `Log(zerolog.PanicLevel, msg, fields...)` (which will both log and panic via zerolog) instead.

## Usage

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

## Notes

- All output goes to **stdout**. There is no file/syslog sink today.
- `InitLogger` returns the underlying `zerolog.Logger` so callers that need the raw API (e.g. `.With().Sub()` patterns) can keep a reference. But the package-level helpers always use the singleton stored internally.
- `LogLevel` and `zerolog.Level` are not aliased; `toZerologLevel(level LogLevel) zerolog.Level` does the conversion in one place.
