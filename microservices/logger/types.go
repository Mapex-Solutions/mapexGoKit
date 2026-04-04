package logger

// LogLevel define os níveis de log locais (espelha os do zerolog)
type LogLevel int8

const (
	TraceLevel    LogLevel = -1
	DebugLevel    LogLevel = 0
	InfoLevel     LogLevel = 1
	WarnLevel     LogLevel = 2
	ErrorLevel    LogLevel = 3
	FatalLevel    LogLevel = 4
	PanicLevel    LogLevel = 5
	DisabledLevel LogLevel = 7 // zerolog.Disabled — suppresses all log output
)

// Field represents a structured log field (key-value pair)
type Field struct {
	Key   string
	Value interface{}
}

// LoggerOptions defines the configuration for initializing the logger
type LoggerOptions struct {
	ServiceName    string   // e.g., "auth-service"
	ServiceVersion string   // e.g., "v1.0.0"
	Environment    string   // e.g., "development", "production"
	Level          LogLevel // e.g., "InfoLevel"
}
