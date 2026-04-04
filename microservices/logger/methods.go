package logger

import (
	"github.com/rs/zerolog"
)

// Log logs a message at the specified level, with optional fields
func Log(level zerolog.Level, msg string, fields ...Field) {
	if msg == "" {
		panic("logger: message cannot be empty")
	}

	event := instance.WithLevel(level)
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Info logs an info-level message
func Info(msg string, fields ...Field) {
	Log(zerolog.InfoLevel, msg, fields...)
}

// Debug logs a debug-level message
func Debug(msg string, fields ...Field) {
	Log(zerolog.DebugLevel, msg, fields...)
}

// Warn logs a warning-level message
func Warn(msg string, fields ...Field) {
	Log(zerolog.WarnLevel, msg, fields...)
}

// Error logs an error-level message with optional structured fields
func Error(err error, msg string, fields ...Field) {
	if msg == "" {
		msg = err.Error()
	}

	event := instance.Error().Err(err)
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Panic logs a panic-level message and triggers a panic
func Panic(msg string, fields ...Field) {
	red := "\033[31m"  // ANSI code for red
	reset := "\033[0m" // Reset color
	panic(red + msg + reset)
}
