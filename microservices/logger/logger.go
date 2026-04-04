package logger

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	once     sync.Once
	instance zerolog.Logger
)

// InitLogger configura e retorna uma instância singleton do logger
func InitLogger(options LoggerOptions) zerolog.Logger {
	once.Do(func() {
		zerolog.TimeFieldFormat = time.RFC3339

		instance = zerolog.New(os.Stdout).
			Level(toZerologLevel(options.Level)).
			With().
			Timestamp().
			Str("env", options.Environment).
			Str("service", options.ServiceName).
			Str("version", options.ServiceVersion).
			Logger()

		log.Logger = instance
	})

	return instance
}

// toZerologLevel converte LogLevel interno para zerolog.Level
func toZerologLevel(level LogLevel) zerolog.Level {
	return zerolog.Level(level)
}
